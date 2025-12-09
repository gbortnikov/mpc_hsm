# MPC Orchestrator API Documentation

## Содержание

1. [Обзор](#обзор)
2. [Быстрый старт](#быстрый-старт)
3. [Конфигурация](#конфигурация)
4. [GraphQL API](#graphql-api)
5. [Примеры использования](#примеры-использования)
6. [Обработка ошибок](#обработка-ошибок)
7. [Безопасность](#безопасность)

---

## Обзор

MPC Orchestrator — это GraphQL сервис для координации распределенных операций MPC (Multi-Party Computation):
- **Keygen** — распределенная генерация ключей (DKG)
- **Signing** — пороговое подписание транзакций (TSS)

### Архитектура

```
┌─────────────┐
│   Client    │
│  (GraphQL)  │
└──────┬──────┘
       │
       v
┌──────────────────┐
│  Orchestrator    │
│   (Port 8081)    │
└──────┬───────────┘
       │ gRPC
       v
┌──────────────────┐
│   MPC Nodes      │
│ (party1, party2) │
└──────────────────┘
```

### Основные возможности

- ✅ Регистрация и управление MPC нодами
- ✅ Распределенная генерация ключей (threshold signatures)
- ✅ Пороговое подписание транзакций
- ✅ Health checks и мониторинг нод
- ✅ Структурированное логирование
- ✅ Конфигурация через переменные окружения
- ✅ Graceful shutdown

---

## Быстрый старт

### Запуск оркестратора

```bash
cd apps/orchestrator

# Запуск с настройками по умолчанию
go run server.go

# Или с кастомной конфигурацией
PORT=8082 \
SESSION_MAX_ITERATIONS=200 \
LOG_FORMAT=json \
go run server.go
```

### Проверка работоспособности

```bash
# Health check
curl http://localhost:8081/health

# GraphQL Playground
open http://localhost:8081/
```

---

## Конфигурация

Все настройки задаются через переменные окружения:

### Настройки сервера

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `PORT` | `8081` | Порт HTTP сервера |
| `ENABLE_PLAYGROUND` | `true` | Включить GraphQL Playground |
| `ENABLE_INTROSPECTION` | `true` | Включить GraphQL introspection |
| `SERVER_READ_TIMEOUT` | `30s` | Таймаут чтения запросов |
| `SERVER_WRITE_TIMEOUT` | `30s` | Таймаут записи ответов |
| `MAX_QUERY_CACHE_SIZE` | `1000` | Размер кэша GraphQL запросов |
| `MAX_APQ_CACHE_SIZE` | `100` | Размер кэша APQ |
| `WEBSOCKET_PING_INTERVAL` | `10s` | Интервал WebSocket ping |
| `ALLOWED_ORIGIN` | `` | CORS allowed origin (пусто = все) |

### Настройки MPC сессий

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `SESSION_MAX_ITERATIONS` | `100` | Максимум итераций обмена сообщениями |
| `SESSION_ITERATION_SLEEP_MS` | `100` | Пауза между итерациями (мс) |
| `SESSION_INIT_SLEEP_MS` | `1000` | Пауза после инициализации (мс) |
| `SESSION_NO_MESSAGE_TIMEOUT` | `30` | Таймаут при отсутствии сообщений |
| `SESSION_REQUEST_TIMEOUT_SEC` | `10` | Таймаут gRPC запросов (сек) |

### Настройки логирования

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `LOG_FORMAT` | `text` | Формат логов: `text` или `json` |
| `LOG_LEVEL` | `INFO` | Уровень: `DEBUG`, `INFO`, `WARN`, `ERROR` |

### Пример конфигурации

```bash
# .env файл для production
PORT=8081
ENABLE_PLAYGROUND=false
ENABLE_INTROSPECTION=false
LOG_FORMAT=json
LOG_LEVEL=INFO
SESSION_MAX_ITERATIONS=150
SESSION_ITERATION_SLEEP_MS=50
ALLOWED_ORIGIN=https://example.com
```

---

## GraphQL API

### Endpoints

- **GraphQL API**: `POST http://localhost:8081/query`
- **GraphQL Playground**: `GET http://localhost:8081/`
- **Health Check**: `GET http://localhost:8081/health`

### Schema Overview

```graphql
type Query {
  # Получить список всех нод
  nodes: [Node!]!

  # Получить ноду по ID
  node(id: ID!): Node
}

type Mutation {
  # Зарегистрировать новую ноду
  registerNode(input: RegisterNodeInput!): Node!

  # Удалить ноду
  deleteNode(id: ID!): Node

  # Запустить генерацию ключей
  startKeygen(input: StartKeygenInput!): KeygenResult!

  # Запустить подписание
  startSigning(input: StartSigningInput!): SigningResult!
}
```

---

## Типы данных

### Node

```graphql
type Node {
  id: ID!              # Внутренний ID (node_1, node_2, ...)
  partyId: String!     # Party ID для MPC протокола
  address: String!     # gRPC адрес (localhost:5000)
  publicKey: String!   # Публичный ключ ноды
  status: NodeStatus!  # Статус: ONLINE, OFFLINE, BUSY, ERROR
  lastSeen: String!    # Время последней проверки (RFC3339)
}

enum NodeStatus {
  ONLINE
  OFFLINE
  BUSY
  ERROR
}
```

### KeygenResult

```graphql
type KeygenResult {
  sessionId: String!    # ID сессии
  publicKey: String!    # Сгенерированный публичный ключ
  address: String!      # Ethereum адрес
  threshold: Int!       # Порог подписей
  totalParties: Int!    # Всего участников
}
```

### SigningResult

```graphql
type SigningResult {
  sessionId: String!  # ID сессии
  signature: String!  # Полная подпись (hex)
  r: String!          # Компонент R (hex)
  s: String!          # Компонент S (hex)
  v: Int!             # Recovery ID
}
```

---

## Примеры использования

### 1. Регистрация нод

**Запрос:**
```graphql
mutation RegisterNodes {
  node1: registerNode(input: {
    address: "localhost:5001"
  }) {
    id
    partyId
    address
    status
  }

  node2: registerNode(input: {
    address: "localhost:5002"
  }) {
    id
    partyId
    address
    status
  }

  node3: registerNode(input: {
    address: "localhost:5003"
  }) {
    id
    partyId
    address
    status
  }
}
```

**Ответ:**
```json
{
  "data": {
    "node1": {
      "id": "node_1",
      "partyId": "party1",
      "address": "localhost:5001",
      "status": "ONLINE"
    },
    "node2": {
      "id": "node_2",
      "partyId": "party2",
      "address": "localhost:5002",
      "status": "ONLINE"
    },
    "node3": {
      "id": "node_3",
      "partyId": "party3",
      "address": "localhost:5003",
      "status": "ONLINE"
    }
  }
}
```

**cURL:**
```bash
curl -X POST http://localhost:8081/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { registerNode(input: {address: \"localhost:5001\"}) { id partyId status } }"
  }'
```

---

### 2. Проверка статуса нод

**Запрос:**
```graphql
query GetAllNodes {
  nodes {
    id
    partyId
    address
    publicKey
    status
    lastSeen
  }
}
```

**Ответ:**
```json
{
  "data": {
    "nodes": [
      {
        "id": "node_1",
        "partyId": "party1",
        "address": "localhost:5001",
        "publicKey": "0x04abc...",
        "status": "ONLINE",
        "lastSeen": "2024-01-15T10:30:00Z"
      },
      {
        "id": "node_2",
        "partyId": "party2",
        "address": "localhost:5002",
        "publicKey": "0x04def...",
        "status": "ONLINE",
        "lastSeen": "2024-01-15T10:30:00Z"
      }
    ]
  }
}
```

---

### 3. Генерация ключей (2-of-3 threshold)

**Запрос:**
```graphql
mutation GenerateKey {
  startKeygen(input: {
    participants: ["party1", "party2", "party3"]
    threshold: 2
    curve: "secp256k1"
  }) {
    sessionId
    publicKey
    address
    threshold
    totalParties
  }
}
```

**Ответ:**
```json
{
  "data": {
    "startKeygen": {
      "sessionId": "keygen_a1b2c3d4e5f6",
      "publicKey": "0x04a8b5c2d1e4f3...",
      "address": "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb6",
      "threshold": 2,
      "totalParties": 3
    }
  }
}
```

**cURL:**
```bash
curl -X POST http://localhost:8081/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { startKeygen(input: {participants: [\"party1\", \"party2\", \"party3\"], threshold: 2}) { sessionId publicKey address } }"
  }'
```

**Пояснение:**
- **threshold: 2** — для подписания нужно минимум 2 участника
- **totalParties: 3** — всего 3 участника в группе
- **curve: "secp256k1"** — эллиптическая кривая (Ethereum/Bitcoin)

---

### 4. Подписание транзакции

**Запрос (hex-encoded сообщение):**
```graphql
mutation SignTransaction {
  startSigning(input: {
    address: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb6"
    message: "0x1234567890abcdef"
    participants: ["party1", "party2"]
  }) {
    sessionId
    signature
    r
    s
    v
  }
}
```

**Запрос (UTF-8 строка):**
```graphql
mutation SignMessage {
  startSigning(input: {
    address: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb6"
    message: "Hello, MPC World!"
    participants: ["party1", "party3"]
  }) {
    sessionId
    signature
    r
    s
    v
  }
}
```

**Ответ:**
```json
{
  "data": {
    "startSigning": {
      "sessionId": "signing_f6e5d4c3b2a1",
      "signature": "0x8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b2c1d0e9f8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f",
      "r": "0x8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b2c1d0e9f8a7b",
      "s": "0x6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b2c1d0e9f8a7b6c5d",
      "v": 28
    }
  }
}
```

**cURL:**
```bash
curl -X POST http://localhost:8081/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { startSigning(input: {address: \"0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb6\", message: \"0x1234567890abcdef\", participants: [\"party1\", \"party2\"]}) { signature r s v } }"
  }'
```

**Пояснение компонентов подписи:**
- **r, s** — криптографические компоненты ECDSA подписи
- **v** — recovery ID (27 или 28 для Ethereum)
- **signature** — полная подпись в формате hex

---

### 5. Удаление ноды

**Запрос:**
```graphql
mutation RemoveNode {
  deleteNode(id: "node_3") {
    id
    partyId
    status
  }
}
```

**Ответ:**
```json
{
  "data": {
    "deleteNode": {
      "id": "node_3",
      "partyId": "party3",
      "status": "OFFLINE"
    }
  }
}
```

---

## Сложные сценарии

### 1. Полный цикл: регистрация → keygen → signing

```graphql
# Шаг 1: Регистрируем 3 ноды
mutation Step1_Register {
  n1: registerNode(input: {address: "localhost:5001"}) { id partyId }
  n2: registerNode(input: {address: "localhost:5002"}) { id partyId }
  n3: registerNode(input: {address: "localhost:5003"}) { id partyId }
}

# Шаг 2: Генерируем ключ с threshold 2-of-3
mutation Step2_Keygen {
  startKeygen(input: {
    participants: ["party1", "party2", "party3"]
    threshold: 2
  }) {
    sessionId
    address
    publicKey
  }
}

# Шаг 3: Подписываем транзакцию (нужно минимум 2 участника)
mutation Step3_Sign {
  startSigning(input: {
    address: "0x742d35Cc..." # из шага 2
    message: "0xabcdef123456"
    participants: ["party1", "party2"] # любые 2 из 3
  }) {
    signature
  }
}
```

### 2. Rotating participants (разные участники для signing)

```graphql
# Первая подпись с party1 + party2
mutation Sign1 {
  sig1: startSigning(input: {
    address: "0x742d35Cc..."
    message: "Transaction 1"
    participants: ["party1", "party2"]
  }) { signature }
}

# Вторая подпись с party2 + party3
mutation Sign2 {
  sig2: startSigning(input: {
    address: "0x742d35Cc..."
    message: "Transaction 2"
    participants: ["party2", "party3"]
  }) { signature }
}

# Третья подпись с party1 + party3
mutation Sign3 {
  sig3: startSigning(input: {
    address: "0x742d35Cc..."
    message: "Transaction 3"
    participants: ["party1", "party3"]
  }) { signature }
}
```

Все 3 подписи будут валидны для одного и того же адреса, так как threshold = 2.

---

## Обработка ошибок

### Формат ошибок GraphQL

```json
{
  "errors": [
    {
      "message": "требуется минимум 2 участника для keygen, получено: 1",
      "path": ["startKeygen"],
      "extensions": {
        "code": "BAD_USER_INPUT"
      }
    }
  ],
  "data": null
}
```

### Типичные ошибки

| Ошибка | Причина | Решение |
|--------|---------|---------|
| `нода не найдена` | Неверный ID ноды | Проверьте `query { nodes { id } }` |
| `не удалось подключиться к ноде` | Нода недоступна | Проверьте, что нода запущена |
| `требуется минимум 2 участника` | Недостаточно участников | Добавьте больше нод |
| `threshold не может быть больше количества участников` | threshold > totalParties | Уменьшите threshold |
| `keygen не завершён успешно` | Ошибка в протоколе MPC | Проверьте логи нод |

### HTTP статусы

- **200 OK** — запрос обработан (даже если есть GraphQL ошибки)
- **400 Bad Request** — неверный синтаксис GraphQL
- **500 Internal Server Error** — внутренняя ошибка сервера

---

## Безопасность

### Рекомендации для production

#### 1. TLS для gRPC соединений

Сейчас используется `grpc.WithInsecure()`. В продакшене используйте mTLS:

```go
// В nodeclient/client.go
creds, err := credentials.NewClientTLSFromFile("cert.pem", "")
conn, err := grpc.Dial(address, grpc.WithTransportCredentials(creds))
```

#### 2. Аутентификация API

Добавьте middleware для проверки токенов:

```go
// В server.go
mux.Handle("/query", authMiddleware(srv))

func authMiddleware(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    token := r.Header.Get("Authorization")
    if !validateToken(token) {
      http.Error(w, "Unauthorized", http.StatusUnauthorized)
      return
    }
    next.ServeHTTP(w, r)
  })
}
```

#### 3. CORS настройки

```bash
# Ограничьте allowed origins
ALLOWED_ORIGIN=https://your-frontend.com
```

#### 4. Отключите introspection в production

```bash
ENABLE_INTROSPECTION=false
ENABLE_PLAYGROUND=false
```

#### 5. Rate limiting

Используйте nginx или middleware для ограничения запросов:

```nginx
limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;

location /query {
  limit_req zone=api burst=20;
  proxy_pass http://localhost:8081;
}
```

---

## Мониторинг и логи

### Структурированные логи (JSON)

```bash
LOG_FORMAT=json go run server.go
```

**Пример:**
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "message": "Keygen успешно завершён",
  "fields": {
    "session_id": "keygen_abc123",
    "public_key": "0x04a8b5...",
    "address": "0x742d35Cc...",
    "threshold": 2
  }
}
```

### Health check endpoint

```bash
curl http://localhost:8081/health
# {"status":"healthy"}
```

### Метрики для Prometheus (будущее улучшение)

```
# TYPE orchestrator_keygen_total counter
orchestrator_keygen_total{status="success"} 42
orchestrator_keygen_total{status="failed"} 3

# TYPE orchestrator_signing_duration_seconds histogram
orchestrator_signing_duration_seconds_bucket{le="1.0"} 35
orchestrator_signing_duration_seconds_bucket{le="5.0"} 48
```

---

## Troubleshooting

### Проблема: Keygen зависает

**Симптомы:** Запрос не возвращается, в логах "слишком много итераций без сообщений"

**Решение:**
1. Проверьте, что все ноды доступны: `query { nodes { status } }`
2. Увеличьте таймауты:
   ```bash
   SESSION_MAX_ITERATIONS=200 \
   SESSION_NO_MESSAGE_TIMEOUT=50 \
   go run server.go
   ```

### Проблема: Ошибка "нода отклонила сообщение"

**Симптомы:** В логах `"нода отклонила keygen сообщение"`

**Решение:**
1. Проверьте логи самой ноды
2. Убедитесь, что signing public keys корректны
3. Перезапустите ноды и переинициализируйте keygen

### Проблема: Подпись невалидна

**Симптомы:** Подпись не проходит верификацию

**Решение:**
1. Проверьте, что использован правильный `address` (из keygen)
2. Убедитесь, что достаточно участников (>= threshold)
3. Проверьте формат сообщения (hex должен начинаться с `0x`)

---

## API Changelog

### v1.0.0 (Current)
- ✅ Базовые операции: registerNode, startKeygen, startSigning
- ✅ Health checks
- ✅ Конфигурация через env vars
- ✅ Структурированное логирование
- ✅ Graceful shutdown

### Roadmap (v1.1.0)
- 🔄 Session status queries
- 🔄 Operation cancellation
- 🔄 Persistent storage (PostgreSQL)
- 🔄 GraphQL subscriptions для real-time прогресса
- 🔄 Prometheus metrics

---

## Дополнительные ресурсы

- **GraphQL Playground**: http://localhost:8081/ (в dev режиме)
- **Schema**: См. `apps/orchestrator/graph/schema.graphqls`
- **Исходный код**: `apps/orchestrator/`

---

## Поддержка

Для вопросов и багов создавайте issue в репозитории проекта.
