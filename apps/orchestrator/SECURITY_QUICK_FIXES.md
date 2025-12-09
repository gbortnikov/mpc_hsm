# Security Quick Fixes - Приоритетный список

## 🚨 КРИТИЧНО - Исправить немедленно

### 1. Добавить JWT аутентификацию (2 дня)

```bash
# Установить
go get github.com/golang-jwt/jwt/v5

# Создать middleware/auth.go
# См. SECURITY_AUDIT.md раздел 1
```

**ENV:**
```bash
JWT_SECRET=your-secret-key-min-256-bits
```

---

### 2. Включить mTLS для gRPC (3 дня)

```bash
# Сгенерировать сертификаты
./scripts/generate_certs.sh

# Обновить nodeclient/client.go
# См. SECURITY_AUDIT.md раздел 2
```

**Файлы:**
- `certs/ca.crt`, `certs/ca.key`
- `certs/client.crt`, `certs/client.key`
- `certs/node.crt`, `certs/node.key`

---

### 3. Добавить rate limiting (1 день)

```bash
go get golang.org/x/time/rate
```

**Code:**
```go
// middleware/ratelimit.go
rateLimiter := middleware.NewRateLimiter(10, 20)
mux.Handle("/query", rateLimiter.Middleware()(srv))
```

---

## 🔥 ВЫСОКИЙ - Исправить на этой неделе

### 4. Исправить CORS (1 час)

**config/config.go:**
```go
// По умолчанию ЗАПРЕЩАТЬ все origins
if allowedOrigin == "" {
    return false  // ✅
}
```

---

### 5. Добавить security headers (1 час)

```go
// middleware/security.go
mux.Handle("/query", middleware.SecurityHeaders(srv))
```

---

### 6. Улучшить валидацию (2 часа)

```go
const (
    MinThreshold = 2      // Было: 1
    MaxParticipants = 100 // Не было
)

// Добавить validateKeygenInput()
```

---

### 7. Валидация кривой (1 час)

```go
var allowedCurves = map[string]bool{
    "secp256k1": true,
    "secp256r1": true,
}
```

---

### 8. Отключить introspection по умолчанию (5 минут)

**config/config.go:**
```go
EnableIntrospection: getEnvBool("ENABLE_INTROSPECTION", false), // ✅
EnablePlayground:    getEnvBool("ENABLE_PLAYGROUND", false),    // ✅
```

---

### 9. Исправить ошибки (3 часа)

```go
// Не возвращать детали ошибок клиенту
return nil, NewAppError(ErrInternal, "Operation failed", err)

// Логировать детали server-side
logger.Error("Detailed error", map[string]interface{}{
    "error": err.Error(),
})
```

---

## ⚠️ СРЕДНИЙ - Исправить в течение месяца

### 10. Добавить таймауты (2 часа)

```go
ctx, cancel := context.WithTimeout(ctx, operationTimeout)
defer cancel()
```

---

### 11. Лимиты размера сообщений (1 час)

```go
const MaxMessageSize = 32 * 1024

if len(messageBytes) > MaxMessageSize {
    return nil, fmt.Errorf("message too large")
}
```

---

### 12. Audit logging (1 день)

```go
// audit/audit.go
audit.Log(userID, "REGISTER_NODE", nodeID, "SUCCESS", details)
```

---

## 📋 Чеклист перед production

```
Критично:
[ ] JWT аутентификация включена
[ ] mTLS для gRPC настроен
[ ] Rate limiting работает
[ ] CORS default = deny
[ ] Introspection отключен
[ ] Playground отключен

Высокий:
[ ] Security headers добавлены
[ ] Threshold >= 2
[ ] Curve whitelist проверяется
[ ] Ошибки не содержат деталей
[ ] Логи не содержат sensitive data

Средний:
[ ] Операционные таймауты настроены
[ ] Размер сообщений ограничен
[ ] Audit logging включен
[ ] Мониторинг настроен
```

---

## ENV для production

```bash
# Security
JWT_SECRET=<256-bit-key>
ALLOWED_ORIGINS=https://app.example.com

# Features
ENABLE_PLAYGROUND=false
ENABLE_INTROSPECTION=false

# Limits
RATE_LIMIT_RPS=10
MAX_PARTICIPANTS=100
MIN_THRESHOLD=2
MAX_MESSAGE_SIZE_KB=32

# Logging
LOG_FORMAT=json
LOG_LEVEL=INFO
```

---

## Тестирование безопасности

### 1. Проверка аутентификации

```bash
# Без токена - должно отклонить
curl -X POST http://localhost:8081/query \
  -H "Content-Type: application/json" \
  -d '{"query": "query { nodes { id } }"}'

# С токеном - должно работать
curl -X POST http://localhost:8081/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{"query": "query { nodes { id } }"}'
```

### 2. Проверка rate limiting

```bash
# Отправить 100 запросов быстро - должен заблокировать
for i in {1..100}; do
  curl -X POST http://localhost:8081/query &
done
```

### 3. Проверка валидации

```bash
# Threshold = 1 - должно отклонить
curl -X POST http://localhost:8081/query \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{"query": "mutation { startKeygen(input: {participants: [\"p1\", \"p2\"], threshold: 1}) { address } }"}'

# Неправильная curve - должно отклонить
curl -X POST http://localhost:8081/query \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{"query": "mutation { startKeygen(input: {participants: [\"p1\", \"p2\"], threshold: 2, curve: \"invalid\"}) { address } }"}'
```

### 4. Проверка TLS

```bash
# Должно использовать TLS
openssl s_client -connect localhost:8081 -showcerts

# Для gRPC
grpcurl -insecure localhost:5001 list  # Должно не работать
grpcurl -cacert ca.crt localhost:5001 list  # Должно работать
```

---

## Приоритеты по времени

### Week 1 (Критично)
**День 1-2:** JWT аутентификация
**День 3-4:** mTLS для gRPC
**День 5:** Rate limiting + тесты

### Week 2 (Высокий)
**День 1:** CORS + Security headers + Валидация
**День 2:** Introspection/Playground настройки
**День 3-4:** Исправление ошибок и логирования
**День 5:** Тестирование и документация

### Week 3 (Средний + Testing)
**День 1:** Таймауты + лимиты
**День 2:** Audit logging
**День 3-4:** Integration testing
**День 5:** Security penetration testing

---

## Ресурсы

- **OWASP Top 10:** https://owasp.org/www-project-top-ten/
- **GraphQL Security:** https://cheatsheetseries.owasp.org/cheatsheets/GraphQL_Cheat_Sheet.html
- **Go Security:** https://go.dev/doc/security/
- **mTLS Guide:** https://smallstep.com/hello-mtls/

---

**Status:** 🔴 NOT PRODUCTION READY → 🟡 BETA (после Week 2) → 🟢 PRODUCTION (после Week 3)
