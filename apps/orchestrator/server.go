package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	"github.com/mpc_hsm/orchestrator/graph"
	"github.com/vektah/gqlparser/v2/ast"
)

const defaultPort = "8081"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	resolver := graph.NewResolver()
	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))

	// HTTP транспорты
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	// WebSocket транспорт для подписок
	srv.AddTransport(&transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Разрешаем все источники для разработки
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		KeepAlivePingInterval: 10 * time.Second,
	})

	// Кэширование запросов
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	// Расширения
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	// Маршруты
	http.Handle("/", playground.Handler("MPC Оркестратор", "/query"))
	http.Handle("/query", srv)

	log.Printf("MPC Оркестратор запущен на http://localhost:%s/", port)
	log.Printf("GraphQL Playground: http://localhost:%s/", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
