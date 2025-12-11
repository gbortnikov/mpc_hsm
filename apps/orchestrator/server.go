package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/mpc_hsm/orchestrator/config"
	"github.com/mpc_hsm/orchestrator/graph"
	"github.com/mpc_hsm/orchestrator/logger"
	"github.com/vektah/gqlparser/v2/ast"
)

// corsMiddleware добавляет CORS заголовки для всех запросов
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Разрешаем все источники для разработки
		// В продакшене следует ограничить список источников
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization, X-CSRF-Token")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Обрабатываем preflight запросы
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Загружаем переменные окружения из .env файла
	// Игнорируем ошибку, если файл не найден (переменные могут быть установлены другим способом)
	_ = godotenv.Load()

	// Загружаем конфигурацию
	cfg := config.Load()

	logger.Info("Загружена конфигурация", map[string]interface{}{
		"port":                    cfg.Server.Port,
		"max_iterations":          cfg.Session.MaxIterations,
		"iteration_sleep_ms":      cfg.Session.IterationSleepMs,
		"initialization_sleep_ms": cfg.Session.InitializationSleepMs,
	})

	// Создаем resolver с конфигурацией
	resolver := graph.NewResolver(cfg)
	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))

	// HTTP транспорты
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	// WebSocket транспорт для подписок
	srv.AddTransport(&transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// В продакшене следует настроить CORS политику
				allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
				if allowedOrigin == "" {
					return true // Разрешаем все источники для разработки
				}
				origin := r.Header.Get("Origin")
				return origin == allowedOrigin
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		KeepAlivePingInterval: cfg.Server.WebSocketPingInterval,
	})

	// Кэширование запросов
	srv.SetQueryCache(lru.New[*ast.QueryDocument](cfg.Server.MaxQueryCacheSize))

	// Расширения
	if cfg.Server.EnableIntrospection {
		srv.Use(extension.Introspection{})
		logger.Info("GraphQL introspection включен", nil)
	}
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](cfg.Server.MaxAPQCacheSize),
	})

	// Маршруты
	mux := http.NewServeMux()

	// GraphQL endpoint с CORS
	mux.Handle("/query", corsMiddleware(srv))

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Playground (опционально)
	if cfg.Server.EnablePlayground {
		mux.Handle("/", playground.Handler("MPC Оркестратор", "/query"))
		logger.Info("GraphQL Playground включен", map[string]interface{}{
			"url": "http://localhost:" + cfg.Server.Port + "/",
		})
	}

	// HTTP сервер с таймаутами
	httpServer := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Запуск сервера в отдельной горутине
	go func() {
		logger.Info("MPC Оркестратор запущен", map[string]interface{}{
			"port":     cfg.Server.Port,
			"endpoint": "http://localhost:" + cfg.Server.Port + "/query",
		})

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Ошибка запуска сервера", map[string]interface{}{
				"error": err.Error(),
			})
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Останавливаем оркестратор...", nil)

	// Даем серверу 30 секунд на завершение текущих запросов
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("Ошибка остановки сервера", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Закрываем все соединения с нодами
	resolver.NodeManager.CloseAll()

	logger.Info("Оркестратор остановлен", nil)
}
