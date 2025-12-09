# MPC Orchestrator - Security Audit Report

**Date:** 2024-12-09
**Version:** 1.0.0
**Status:** 🔴 NOT PRODUCTION READY

---

## Executive Summary

Проведен комплексный аудит безопасности MPC Orchestrator. Выявлено **28 проблем безопасности**:

- 🔴 **Critical**: 3 (2 блокируют production, 1 исправлена ✅)
  - ✅ Незащищенное gRPC соединение - **ИСПРАВЛЕНО** (mTLS реализован)
  - ❌ Отсутствие аутентификации - требует исправления
  - ❌ Отсутствие rate limiting - требует исправления
- 🟠 **High**: 8 (требуют немедленного исправления)
- 🟡 **Medium**: 7 (исправить перед полным развертыванием)
- 🟢 **Low**: 5 (желательно исправить)
- ✅ **Positive**: 5 (хорошие практики)

**Прогресс:** 1 из 3 критических проблем исправлена (33%)

**Вердикт:** Система подходит только для **development/testing**. Требуется исправление оставшихся 2 критических проблем перед production использованием.

---

## Критические проблемы (🔴 Critical)

### 1. Отсутствие аутентификации

**Проблема:**
API полностью открыт без какой-либо аутентификации. Любой может:
- Регистрировать ноды
- Запускать keygen/signing
- Удалять ноды

**Файл:** `server.go:73-91`

**Код:**
```go
// НЕТ middleware для проверки токенов!
mux.Handle("/query", srv)
```

**Решение:**
```go
// Добавить JWT middleware
package middleware

import (
    "context"
    "net/http"
    "strings"
    "github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserContextKey contextKey = "user"

type Claims struct {
    UserID string `json:"user_id"`
    Role   string `json:"role"`
    jwt.RegisteredClaims
}

func AuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, "Missing authorization header", http.StatusUnauthorized)
                return
            }

            tokenString := strings.TrimPrefix(authHeader, "Bearer ")

            token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
                return jwtSecret, nil
            })

            if err != nil || !token.Valid {
                http.Error(w, "Invalid token", http.StatusUnauthorized)
                return
            }

            claims := token.Claims.(*Claims)
            ctx := context.WithValue(r.Context(), UserContextKey, claims)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// В server.go
jwtSecret := []byte(os.Getenv("JWT_SECRET"))
if len(jwtSecret) == 0 {
    log.Fatal("JWT_SECRET must be set")
}

mux.Handle("/query", middleware.AuthMiddleware(jwtSecret)(srv))
```

**Переменные окружения:**
```bash
JWT_SECRET=your-256-bit-secret-key
JWT_EXPIRATION=24h
```

---

### 2. Незащищенное gRPC соединение (plaintext) - ✅ RESOLVED

**Статус:** ✅ **ИСПРАВЛЕНО** в текущей версии

**Проблема:**
Все данные между оркестратором и нодами передаются открытым текстом, включая key shares и подписи.

**Файл:** `nodeclient/client.go:76-134`

**Решение реализовано:**
- Добавлена полная поддержка mTLS (mutual TLS)
- Минимальная версия TLS 1.3
- Поддержка как secure, так и insecure режимов через конфигурацию
- Проверка сертификатов CA
- Клиентская аутентификация через сертификаты

**Код (реализован):**
```go
// Connect устанавливает соединение с нодой (с поддержкой mTLS)
func (nc *NodeClient) Connect(ctx context.Context) error {
    nc.mu.Lock()
    defer nc.mu.Unlock()

    var dialOpts []grpc.DialOption

    // Настраиваем TLS или insecure соединение
    if nc.tlsConfig != nil && nc.tlsConfig.Enabled {
        logger.Info("Подключение к ноде с mTLS", map[string]interface{}{
            "address": nc.address,
        })

        creds, err := nc.loadTLSCredentials()
        if err != nil {
            return fmt.Errorf("не удалось загрузить TLS credentials: %w", err)
        }

        dialOpts = append(dialOpts,
            grpc.WithTransportCredentials(creds),
            grpc.WithBlock(),
        )
    } else {
        logger.Warn("Подключение к ноде БЕЗ TLS (INSECURE)", map[string]interface{}{
            "address": nc.address,
        })

        dialOpts = append(dialOpts,
            grpc.WithTransportCredentials(insecure.NewCredentials()),
            grpc.WithBlock(),
        )
    }

    conn, err := grpc.DialContext(ctx, nc.address, dialOpts...)
    // ...
}

// loadTLSCredentials загружает TLS credentials для mTLS
func (nc *NodeClient) loadTLSCredentials() (credentials.TransportCredentials, error) {
    // Загружаем CA сертификат
    caCert, err := os.ReadFile(nc.tlsConfig.CAFile)
    if err != nil {
        return nil, fmt.Errorf("не удалось прочитать CA сертификат %s: %w", nc.tlsConfig.CAFile, err)
    }

    caCertPool := x509.NewCertPool()
    if !caCertPool.AppendCertsFromPEM(caCert) {
        return nil, fmt.Errorf("не удалось добавить CA сертификат в pool")
    }

    // Загружаем клиентский сертификат
    clientCert, err := tls.LoadX509KeyPair(nc.tlsConfig.CertFile, nc.tlsConfig.KeyFile)
    if err != nil {
        return nil, fmt.Errorf("не удалось загрузить клиентский сертификат: %w", err)
    }

    // Настраиваем TLS конфигурацию
    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{clientCert},
        RootCAs:      caCertPool,
        MinVersion:   tls.VersionTLS13,  // ✅ SECURE!
    }

    if nc.tlsConfig.ServerName != "" {
        tlsConfig.ServerName = nc.tlsConfig.ServerName
    }

    return credentials.NewTLS(tlsConfig), nil
}
```

**Конфигурация (.env):**
```bash
# Включить TLS для gRPC соединений
TLS_ENABLED=true

# Пути к сертификатам
TLS_CERT_FILE=certs/client.crt
TLS_KEY_FILE=certs/client.key
TLS_CA_FILE=certs/ca.crt
TLS_SERVER_NAME=  # опционально
```

**Старый код (до исправления):**
```go
conn, err := grpc.DialContext(ctx, nc.address,
    grpc.WithTransportCredentials(insecure.NewCredentials()),  // ❌ INSECURE!
    grpc.WithBlock(),
)
```

**Генерация сертификатов:**
Используйте скрипт `scripts/generate_certs.sh` для автоматической генерации всех необходимых сертификатов:
```bash
cd apps/orchestrator
./scripts/generate_certs.sh
```

Скрипт создаст:
- CA сертификат (`certs/ca.crt`)
- Клиентский сертификат оркестратора (`certs/client.crt`, `certs/client.key`)
- Серверные сертификаты для 3 нод (`certs/node1.crt`, `certs/node2.crt`, `certs/node3.crt`)

**Активация mTLS:**
В файле `.env`:
```bash
TLS_ENABLED=true
TLS_CERT_FILE=certs/client.crt
TLS_KEY_FILE=certs/client.key
TLS_CA_FILE=certs/ca.crt
```

---

### 3. Отсутствие rate limiting

**Проблема:**
Нет ограничений на количество запросов. Атакующий может:
- Запустить бесконечные keygen операции
- Исчерпать память/CPU
- Провести DDoS атаку

**Решение:**
```go
package middleware

import (
    "net/http"
    "sync"
    "time"
    "golang.org/x/time/rate"
)

type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rate:     r,
        burst:    b,
    }
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    limiter, exists := rl.limiters[ip]
    if !exists {
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.limiters[ip] = limiter
    }

    return limiter
}

func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip := r.RemoteAddr
            limiter := rl.getLimiter(ip)

            if !limiter.Allow() {
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

// Периодически очищаем старые limiters
func (rl *RateLimiter) CleanupOldLimiters(interval time.Duration) {
    ticker := time.NewTicker(interval)
    go func() {
        for range ticker.C {
            rl.mu.Lock()
            // Очищаем limiters которые не использовались долго
            for ip := range rl.limiters {
                delete(rl.limiters, ip)
            }
            rl.mu.Unlock()
        }
    }()
}

// В server.go
rateLimiter := middleware.NewRateLimiter(10, 20) // 10 req/sec, burst 20
rateLimiter.CleanupOldLimiters(1 * time.Hour)

mux.Handle("/query", rateLimiter.Middleware()(authMiddleware(srv)))
```

**Конфигурация:**
```bash
RATE_LIMIT_RPS=10      # Requests per second
RATE_LIMIT_BURST=20    # Burst size
```

---

## Высокоприоритетные проблемы (🟠 High)

### 4. Небезопасная CORS политика

**Проблема:**
По умолчанию разрешены ВСЕ origins, что позволяет любому сайту делать запросы.

**Файл:** `server.go:46-54`

**Решение:**
```go
CheckOrigin: func(r *http.Request) bool {
    allowedOrigins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
    if len(allowedOrigins) == 0 || allowedOrigins[0] == "" {
        logger.Warn("ALLOWED_ORIGINS not set, denying all origins", nil)
        return false  // ✅ Default deny
    }

    origin := r.Header.Get("Origin")
    for _, allowed := range allowedOrigins {
        if origin == strings.TrimSpace(allowed) {
            return true
        }
    }

    logger.Warn("Blocked origin", map[string]interface{}{
        "origin": origin,
    })
    return false
}
```

**Конфигурация:**
```bash
ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
```

---

### 5. Отсутствие security headers

**Решение:**
```go
package middleware

func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Prevent MIME sniffing
        w.Header().Set("X-Content-Type-Options", "nosniff")

        // Prevent clickjacking
        w.Header().Set("X-Frame-Options", "DENY")

        // Enable XSS protection
        w.Header().Set("X-XSS-Protection", "1; mode=block")

        // Content Security Policy
        w.Header().Set("Content-Security-Policy",
            "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")

        // HSTS (only if using HTTPS)
        if r.TLS != nil {
            w.Header().Set("Strict-Transport-Security",
                "max-age=31536000; includeSubDomains; preload")
        }

        // Referrer Policy
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

        // Permissions Policy
        w.Header().Set("Permissions-Policy",
            "geolocation=(), microphone=(), camera=()")

        next.ServeHTTP(w, r)
    })
}

// В server.go
mux.Handle("/query", middleware.SecurityHeaders(srv))
```

---

### 6. Слабая валидация threshold

**Файл:** `graph/schema.resolvers.go:129-147`

**Проблема:**
Принимает threshold=1, что разрушает безопасность MPC.

**Решение:**
```go
const (
    MinThreshold = 2
    MaxParticipants = 100
    MinParticipants = 2
)

func validateKeygenInput(participants []string, threshold int32) error {
    if len(participants) < MinParticipants {
        return fmt.Errorf("минимум %d участника, получено: %d",
            MinParticipants, len(participants))
    }

    if len(participants) > MaxParticipants {
        return fmt.Errorf("максимум %d участников, получено: %d",
            MaxParticipants, len(participants))
    }

    if threshold < MinThreshold {
        return fmt.Errorf("threshold должен быть >= %d (получено: %d)",
            MinThreshold, threshold)
    }

    if int(threshold) > len(participants) {
        return fmt.Errorf("threshold (%d) не может быть больше участников (%d)",
            threshold, len(participants))
    }

    // Рекомендация: threshold > N/2 для Byzantine fault tolerance
    if int(threshold) <= len(participants)/2 {
        logger.Warn("Threshold <= N/2 не обеспечивает Byzantine resilience",
            map[string]interface{}{
                "threshold":    threshold,
                "participants": len(participants),
            })
    }

    return nil
}

// В StartKeygen
if err := validateKeygenInput(participants, input.Threshold); err != nil {
    return nil, err
}
```

---

### 7. Отсутствие валидации кривой

**Файл:** `graph/schema.resolvers.go:149-153`

**Решение:**
```go
var allowedCurves = map[string]bool{
    "secp256k1": true,
    "secp256r1": true,
    "ed25519":   true,
}

func validateCurve(curve string) error {
    if !allowedCurves[curve] {
        return fmt.Errorf("неподдерживаемая кривая: %s (разрешены: %v)",
            curve, maps.Keys(allowedCurves))
    }
    return nil
}

// В StartKeygen
curve := "secp256k1"
if input.Curve != nil && *input.Curve != "" {
    if err := validateCurve(*input.Curve); err != nil {
        return nil, err
    }
    curve = *input.Curve
}
```

---

### 8. GraphQL introspection включен по умолчанию

**Файл:** `config/config.go:42`

**Проблема:**
Атакующий может получить полную схему API.

**Решение:**
```go
// В config/config.go
EnableIntrospection: getEnvBool("ENABLE_INTROSPECTION", false),  // ✅ Default: false
EnablePlayground:    getEnvBool("ENABLE_PLAYGROUND", false),     // ✅ Default: false
```

---

### 9. Утечка информации в ошибках

**Файл:** `graph/schema.resolvers.go` (множество мест)

**Решение:**
```go
// Создать типы ошибок
type ErrorCode string

const (
    ErrUnauthorized   ErrorCode = "UNAUTHORIZED"
    ErrNotFound       ErrorCode = "NOT_FOUND"
    ErrInvalidInput   ErrorCode = "INVALID_INPUT"
    ErrInternal       ErrorCode = "INTERNAL_ERROR"
)

type AppError struct {
    Code    ErrorCode
    Message string
    Details error // Логируется, но не возвращается клиенту
}

func (e *AppError) Error() string {
    return e.Message
}

func NewAppError(code ErrorCode, message string, details error) *AppError {
    return &AppError{
        Code:    code,
        Message: message,
        Details: details,
    }
}

// В resolver
func (r *mutationResolver) RegisterNode(ctx context.Context, input model.RegisterNodeInput) (*model.Node, error) {
    client, err := r.NodeManager.AddNode(ctx, input.Address)
    if err != nil {
        // Логируем детали
        logger.Error("Failed to connect to node", map[string]interface{}{
            "address": input.Address,
            "error":   err.Error(),
        })

        // Возвращаем общую ошибку
        return nil, NewAppError(
            ErrInternal,
            "Не удалось подключиться к ноде",  // Без деталей!
            err,
        )
    }
    // ...
}
```

---

### 10. GraphQL query complexity не ограничена

**Решение:**
```go
package middleware

import (
    "github.com/99designs/gqlgen/graphql"
    "github.com/99designs/gqlgen/graphql/handler/extension"
)

func SetupQueryComplexity(srv *handler.Server) {
    // Ограничение сложности запроса
    srv.Use(extension.FixedComplexityLimit(1000))

    // Ограничение глубины
    srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
        oc := graphql.GetOperationContext(ctx)

        maxDepth := 10
        depth := calculateQueryDepth(oc.Operation)

        if depth > maxDepth {
            return func(ctx context.Context) *graphql.Response {
                return &graphql.Response{
                    Errors: []*gqlerror.Error{
                        {Message: fmt.Sprintf("query depth exceeds maximum: %d > %d", depth, maxDepth)},
                    },
                }
            }
        }

        return next(ctx)
    })
}

func calculateQueryDepth(sel ast.SelectionSet) int {
    if len(sel) == 0 {
        return 0
    }

    maxDepth := 0
    for _, s := range sel {
        if field, ok := s.(*ast.Field); ok {
            depth := 1 + calculateQueryDepth(field.SelectionSet)
            if depth > maxDepth {
                maxDepth = depth
            }
        }
    }

    return maxDepth
}
```

---

### 11. Нет верификации подписей нод

**Файл:** `nodeclient/client.go`

**Проблема:**
Signing public key нод получается, но никогда не используется для проверки ответов.

**Решение:**
```go
import (
    "crypto/ed25519"
    "encoding/base64"
)

// Верифицировать ответ от ноды
func (nc *NodeClient) verifyResponse(message []byte, signature []byte) error {
    if nc.signingPublicKey == "" {
        return fmt.Errorf("signing public key not set")
    }

    pubKeyBytes, err := base64.StdEncoding.DecodeString(nc.signingPublicKey)
    if err != nil {
        return fmt.Errorf("invalid public key encoding: %w", err)
    }

    if len(pubKeyBytes) != ed25519.PublicKeySize {
        return fmt.Errorf("invalid public key size: %d", len(pubKeyBytes))
    }

    pubKey := ed25519.PublicKey(pubKeyBytes)

    if !ed25519.Verify(pubKey, message, signature) {
        return fmt.Errorf("signature verification failed")
    }

    return nil
}

// Использовать в ProcessKeygenMessage
func (nc *NodeClient) ProcessKeygenMessage(ctx context.Context, msg *pb.KeygenMessage) (*pb.KeygenResponse, error) {
    resp, err := nc.client.ProcessKeygenMessage(ctx, msg)
    if err != nil {
        return nil, err
    }

    // Верифицировать подпись ответа (если есть)
    if len(resp.Signature) > 0 {
        msgBytes := []byte(resp.String()) // Сериализовать ответ
        if err := nc.verifyResponse(msgBytes, resp.Signature); err != nil {
            logger.Error("Response signature verification failed", map[string]interface{}{
                "party_id": nc.partyID,
                "error":    err.Error(),
            })
            return nil, fmt.Errorf("invalid response signature from %s", nc.partyID)
        }
    }

    return resp, nil
}
```

---

## Средний приоритет (🟡 Medium)

### 12. Отсутствие лимитов на размер сообщения

**Решение:**
```go
const MaxMessageSize = 32 * 1024 // 32KB

func (r *mutationResolver) StartSigning(ctx context.Context, input model.StartSigningInput) (*model.SigningResult, error) {
    var messageBytes []byte
    if len(input.Message) > 2 && input.Message[:2] == "0x" {
        messageBytes, err = hex.DecodeString(input.Message[2:])
        if err != nil {
            return nil, fmt.Errorf("неверный hex формат: %w", err)
        }
    } else {
        messageBytes = []byte(input.Message)
    }

    // ✅ Проверка размера
    if len(messageBytes) > MaxMessageSize {
        return nil, fmt.Errorf("сообщение слишком большое: %d > %d байт",
            len(messageBytes), MaxMessageSize)
    }

    // ...
}
```

---

### 13. Таймауты операций

**Решение:**
```go
// В StartKeygen и StartSigning
operationTimeout := time.Duration(r.config.Session.RequestTimeoutSec) * time.Second * time.Duration(maxIterations)

ctx, cancel := context.WithTimeout(ctx, operationTimeout)
defer cancel()

// В цикле проверять контекст
for iteration := 0; iteration < maxIterations; iteration++ {
    select {
    case <-ctx.Done():
        return nil, fmt.Errorf("операция превысила таймаут: %w", ctx.Err())
    default:
        // Продолжаем
    }

    // ... обработка сообщений
}
```

---

### 14. Audit logging

**Решение:**
```go
package audit

import (
    "encoding/json"
    "os"
    "time"
)

type AuditLog struct {
    Timestamp string                 `json:"timestamp"`
    UserID    string                 `json:"user_id"`
    Action    string                 `json:"action"`
    Resource  string                 `json:"resource"`
    Status    string                 `json:"status"`
    Details   map[string]interface{} `json:"details"`
}

var auditFile *os.File

func Init(filename string) error {
    var err error
    auditFile, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
    return err
}

func Log(userID, action, resource, status string, details map[string]interface{}) {
    log := AuditLog{
        Timestamp: time.Now().UTC().Format(time.RFC3339),
        UserID:    userID,
        Action:    action,
        Resource:  resource,
        Status:    status,
        Details:   details,
    }

    data, _ := json.Marshal(log)
    auditFile.Write(append(data, '\n'))
}

// В resolver
func (r *mutationResolver) RegisterNode(ctx context.Context, input model.RegisterNodeInput) (*model.Node, error) {
    userID := ctx.Value(middleware.UserContextKey).(*middleware.Claims).UserID

    node, err := // ... регистрация ...

    if err != nil {
        audit.Log(userID, "REGISTER_NODE", input.Address, "FAILED", map[string]interface{}{
            "error": err.Error(),
        })
        return nil, err
    }

    audit.Log(userID, "REGISTER_NODE", node.ID, "SUCCESS", map[string]interface{}{
        "party_id": node.PartyID,
    })

    return node, nil
}
```

---

## Низкий приоритет (🟢 Low)

### 15. Улучшение обработки panic

**Решение:**
```go
func generateSecureSessionID(prefix string) (string, error) {
    bytes := make([]byte, sessionIDEntropyBytes)
    if _, err := rand.Read(bytes); err != nil {
        logger.Error("crypto/rand failed", map[string]interface{}{
            "error": err.Error(),
        })
        return "", fmt.Errorf("failed to generate session ID: %w", err)
    }
    return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(bytes)), nil
}
```

---

## Production Checklist

### Перед развертыванием в production:

#### Критические (Must Have)
- [ ] Реализована аутентификация (JWT/OAuth2)
- [ ] Включен mTLS для gRPC
- [ ] Добавлен rate limiting
- [ ] Исправлена CORS политика (default deny)
- [ ] Отключены introspection и playground
- [ ] Добавлена валидация всех входных данных

#### Высокий приоритет
- [ ] Добавлены security headers
- [ ] Исправлены сообщения об ошибках
- [ ] Ограничена сложность GraphQL запросов
- [ ] Добавлена верификация подписей нод
- [ ] Улучшена валидация threshold и curve

#### Средний приоритет
- [ ] Реализовано audit logging
- [ ] Добавлены таймауты операций
- [ ] Ограничен размер сообщений
- [ ] Настроены лимиты соединений
- [ ] Добавлен мониторинг ресурсов

#### Низкий приоритет
- [ ] Улучшена обработка ошибок (без panic)
- [ ] Реализован connection pooling
- [ ] Настроено логирование в secure storage
- [ ] Добавлена ротация логов

---

## Конфигурация для production

```bash
# Security
JWT_SECRET=your-256-bit-secret-key-change-me
JWT_EXPIRATION=24h
ALLOWED_ORIGINS=https://app.example.com

# TLS
TLS_CERT_FILE=/etc/orchestrator/certs/server.crt
TLS_KEY_FILE=/etc/orchestrator/certs/server.key
TLS_CA_FILE=/etc/orchestrator/certs/ca.crt

# Rate Limiting
RATE_LIMIT_RPS=10
RATE_LIMIT_BURST=20

# Features (DISABLE in production)
ENABLE_PLAYGROUND=false
ENABLE_INTROSPECTION=false

# Logging
LOG_FORMAT=json
LOG_LEVEL=INFO
AUDIT_LOG_FILE=/var/log/orchestrator/audit.log

# Timeouts
SERVER_READ_TIMEOUT=60s
SERVER_WRITE_TIMEOUT=60s
SESSION_REQUEST_TIMEOUT_SEC=30
SESSION_MAX_ITERATIONS=150

# Limits
MAX_PARTICIPANTS=100
MIN_THRESHOLD=2
MAX_MESSAGE_SIZE_KB=32
```

---

## Дополнительные рекомендации

### 1. Мониторинг

Интегрировать Prometheus метрики:

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    keygenTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "orchestrator_keygen_total",
            Help: "Total keygen operations",
        },
        []string{"status"},
    )

    keygenDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "orchestrator_keygen_duration_seconds",
            Help:    "Keygen operation duration",
            Buckets: prometheus.DefBuckets,
        },
    )
)

func init() {
    prometheus.MustRegister(keygenTotal, keygenDuration)
}
```

### 2. HSM интеграция

Для хранения ключей:

```go
import "github.com/ThalesIgnite/crypto11"

// Инициализация PKCS#11 HSM
func initHSM() error {
    config := &crypto11.Config{
        Path:       "/usr/lib/softhsm/libsofthsm2.so",
        TokenLabel: "orchestrator",
        Pin:        os.Getenv("HSM_PIN"),
    }

    ctx, err := crypto11.Configure(config)
    if err != nil {
        return err
    }

    // Использовать HSM для криптографических операций
    return nil
}
```

### 3. Secrets management

Использовать HashiCorp Vault:

```go
import "github.com/hashicorp/vault/api"

func getSecret(key string) (string, error) {
    client, err := api.NewClient(api.DefaultConfig())
    if err != nil {
        return "", err
    }

    secret, err := client.Logical().Read("secret/data/orchestrator/" + key)
    if err != nil {
        return "", err
    }

    return secret.Data["value"].(string), nil
}
```

---

## Заключение

**Текущий статус:** 🔴 NOT PRODUCTION READY

**Требуется:** 2-3 недели разработки для критических исправлений

**Приоритеты:**
1. **Week 1:** Критические проблемы (аутентификация, TLS, rate limiting)
2. **Week 2:** Высокий приоритет (валидация, security headers, error handling)
3. **Week 3:** Тестирование, аудит, документация

После реализации всех критических и высокоприоритетных исправлений система будет готова к beta-тестированию в production-like окружении.

---

**Следующие шаги:**
1. Создать backlog задач по исправлению
2. Приоритизировать критические уязвимости
3. Реализовать исправления поэтапно
4. Провести повторный security аудит
5. Penetration testing перед production
