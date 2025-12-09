# MPC Orchestrator

GraphQL сервис для координации распределенных MPC операций (Multi-Party Computation).

## 🚀 Быстрый старт

### Предварительные требования

- Go 1.21+
- Запущенные MPC ноды (см. `apps/node/`)

### Установка и запуск

```bash
# Перейдите в директорию оркестратора
cd apps/orchestrator

# Установите зависимости
go mod download

# Запустите сервер
go run server.go
```

Оркестратор будет доступен на `http://localhost:8081`

### Проверка работоспособности

```bash
# Health check
curl http://localhost:8081/health

# Откройте GraphQL Playground
open http://localhost:8081/
```

## 📖 Документация

Полная документация API: [API.md](./API.md)

## 🎯 Основные возможности

### 1. Управление нодами

```graphql
# Регистрация ноды
mutation {
  registerNode(input: {address: "localhost:5001"}) {
    id
    partyId
    status
  }
}

# Список нод
query {
  nodes {
    id
    partyId
    status
  }
}
```

### 2. Генерация ключей (DKG)

```graphql
mutation {
  startKeygen(input: {
    participants: ["party1", "party2", "party3"]
    threshold: 2
    curve: "secp256k1"
  }) {
    sessionId
    publicKey
    address
  }
}
```

### 3. Подписание транзакций (TSS)

```graphql
mutation {
  startSigning(input: {
    address: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb6"
    message: "0x1234567890abcdef"
    participants: ["party1", "party2"]
  }) {
    signature
    r
    s
    v
  }
}
```

## ⚙️ Конфигурация

Настройка через переменные окружения:

```bash
# Основные настройки
PORT=8081                        # HTTP порт
ENABLE_PLAYGROUND=true           # GraphQL Playground
LOG_FORMAT=json                  # Формат логов (text/json)

# MPC сессии
SESSION_MAX_ITERATIONS=100       # Максимум итераций
SESSION_ITERATION_SLEEP_MS=100   # Пауза между итерациями (мс)

# Запуск с конфигурацией
PORT=8082 LOG_FORMAT=json go run server.go
```

Полный список настроек см. в [API.md](./API.md#конфигурация)

## 🏗️ Архитектура

```
┌─────────────────────────┐
│   GraphQL Client        │
│   (Web/CLI/App)         │
└───────────┬─────────────┘
            │ HTTP/GraphQL
            v
┌─────────────────────────┐
│   Orchestrator          │
│   ├── Resolver          │
│   ├── NodeManager       │
│   └── Config            │
└───────────┬─────────────┘
            │ gRPC
            v
┌─────────────────────────┐
│   MPC Nodes             │
│   ├── Party 1           │
│   ├── Party 2           │
│   └── Party 3           │
└─────────────────────────┘
```

## 📁 Структура проекта

```
apps/orchestrator/
├── server.go               # Главный файл сервера
├── config/
│   └── config.go          # Управление конфигурацией
├── logger/
│   └── logger.go          # Структурированное логирование
├── graph/
│   ├── schema.graphqls    # GraphQL схема
│   ├── resolver.go        # Корневой resolver
│   ├── schema.resolvers.go # Бизнес-логика
│   ├── helpers.go         # Вспомогательные функции
│   └── model/
│       └── models_gen.go  # Сгенерированные типы
├── nodeclient/
│   └── client.go          # gRPC клиент для нод
├── API.md                 # Документация API
└── README.md              # Этот файл
```

## 🔧 Разработка

### Генерация GraphQL кода

После изменения `schema.graphqls`:

```bash
go run github.com/99designs/gqlgen generate
```

### Запуск тестов

```bash
go test ./...
```

### Форматирование кода

```bash
go fmt ./...
```

## 🛡️ Безопасность

### Настройка TLS/mTLS (ОБЯЗАТЕЛЬНО для production)

Оркестратор поддерживает защищённое mTLS соединение с нодами для защиты критичных данных (key shares, подписи).

#### Быстрый старт

1. **Сгенерируйте сертификаты:**
   ```bash
   cd apps/orchestrator
   ./scripts/generate_certs.sh
   ```

   Скрипт создаст:
   - CA сертификат (`certs/ca.crt`)
   - Клиентский сертификат для оркестратора (`certs/client.crt`, `certs/client.key`)
   - Серверные сертификаты для 3 нод (`certs/node1-3.crt`)

2. **Скопируйте сертификаты на ноды:**
   ```bash
   # Для каждой ноды скопируйте соответствующий сертификат
   cp certs/ca.crt ../node/certs/
   cp certs/node1.crt ../node/certs/server.crt
   cp certs/node1.key ../node/certs/server.key
   ```

3. **Активируйте TLS в оркестраторе:**
   ```bash
   # Создайте .env файл
   cp .env.example .env

   # Отредактируйте .env
   TLS_ENABLED=true
   TLS_CERT_FILE=certs/client.crt
   TLS_KEY_FILE=certs/client.key
   TLS_CA_FILE=certs/ca.crt
   ```

4. **Запустите с TLS:**
   ```bash
   go run server.go
   ```

#### Проверка TLS соединения

```bash
# В логах должно быть:
# INFO: Подключение к ноде с mTLS address=localhost:5001

# Для отладки можно временно отключить TLS:
TLS_ENABLED=false go run server.go
# WARNING: Подключение к ноде БЕЗ TLS (INSECURE)
```

### Дополнительные рекомендации для production

1. **Отключите playground и introspection:**
   ```bash
   ENABLE_PLAYGROUND=false
   ENABLE_INTROSPECTION=false
   ```

2. **Настройте CORS:**
   ```bash
   ALLOWED_ORIGINS=https://your-domain.com
   ```

3. **Добавьте аутентификацию:**
   Реализуйте JWT middleware для GraphQL API

4. **Rate limiting:**
   Используйте reverse proxy (nginx) с ограничением запросов

Подробный аудит безопасности: [SECURITY_AUDIT.md](./SECURITY_AUDIT.md)

## 📊 Мониторинг

### Логи

Структурированные JSON логи:

```bash
LOG_FORMAT=json LOG_LEVEL=INFO go run server.go
```

Пример:
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "message": "Keygen успешно завершён",
  "fields": {
    "session_id": "keygen_abc123",
    "address": "0x742d35Cc..."
  }
}
```

### Health Check

```bash
curl http://localhost:8081/health
# {"status":"healthy"}
```

## 🐛 Troubleshooting

### Оркестратор не запускается

```bash
# Проверьте, что порт свободен
lsof -i :8081

# Проверьте логи
go run server.go
```

### Keygen зависает

```bash
# Проверьте статус нод
curl -X POST http://localhost:8081/query \
  -H "Content-Type: application/json" \
  -d '{"query": "query { nodes { id status } }"}'

# Увеличьте таймауты
SESSION_MAX_ITERATIONS=200 go run server.go
```

### Ошибка подключения к ноде

```bash
# Убедитесь, что нода запущена
curl http://localhost:5001/health

# Проверьте gRPC соединение
grpcurl -plaintext localhost:5001 list
```

Больше информации: [API.md#troubleshooting](./API.md#troubleshooting)

## 📝 Примеры использования

### Пример 1: Базовый workflow

```bash
# 1. Регистрируем 3 ноды
curl -X POST http://localhost:8081/query -H "Content-Type: application/json" -d '
{
  "query": "mutation { n1: registerNode(input: {address: \"localhost:5001\"}) { partyId } n2: registerNode(input: {address: \"localhost:5002\"}) { partyId } n3: registerNode(input: {address: \"localhost:5003\"}) { partyId } }"
}'

# 2. Генерируем ключ (2-of-3)
curl -X POST http://localhost:8081/query -H "Content-Type: application/json" -d '
{
  "query": "mutation { startKeygen(input: {participants: [\"party1\", \"party2\", \"party3\"], threshold: 2}) { address publicKey } }"
}'

# 3. Подписываем транзакцию
curl -X POST http://localhost:8081/query -H "Content-Type: application/json" -d '
{
  "query": "mutation { startSigning(input: {address: \"0x742d35Cc...\", message: \"0xabcdef\", participants: [\"party1\", \"party2\"]}) { signature } }"
}'
```

### Пример 2: TypeScript клиент

```typescript
import { GraphQLClient, gql } from 'graphql-request'

const client = new GraphQLClient('http://localhost:8081/query')

// Регистрация ноды
const registerNode = gql`
  mutation RegisterNode($address: String!) {
    registerNode(input: { address: $address }) {
      id
      partyId
      status
    }
  }
`

const result = await client.request(registerNode, {
  address: 'localhost:5001'
})

console.log('Node registered:', result.registerNode)

// Keygen
const keygen = gql`
  mutation StartKeygen($participants: [String!]!, $threshold: Int!) {
    startKeygen(input: { participants: $participants, threshold: $threshold }) {
      sessionId
      address
      publicKey
    }
  }
`

const keygenResult = await client.request(keygen, {
  participants: ['party1', 'party2', 'party3'],
  threshold: 2
})

console.log('Key generated:', keygenResult.startKeygen.address)
```

### Пример 3: Python клиент

```python
import requests

ORCHESTRATOR_URL = "http://localhost:8081/query"

def graphql_request(query, variables=None):
    response = requests.post(
        ORCHESTRATOR_URL,
        json={"query": query, "variables": variables}
    )
    return response.json()

# Регистрация ноды
register_mutation = """
mutation RegisterNode($address: String!) {
    registerNode(input: {address: $address}) {
        id
        partyId
        status
    }
}
"""

result = graphql_request(register_mutation, {"address": "localhost:5001"})
print("Node registered:", result["data"]["registerNode"])

# Keygen
keygen_mutation = """
mutation StartKeygen($participants: [String!]!, $threshold: Int!) {
    startKeygen(input: {participants: $participants, threshold: $threshold}) {
        sessionId
        address
        publicKey
    }
}
"""

keygen_result = graphql_request(keygen_mutation, {
    "participants": ["party1", "party2", "party3"],
    "threshold": 2
})
print("Key generated:", keygen_result["data"]["startKeygen"]["address"])
```

Больше примеров: [API.md#примеры-использования](./API.md#примеры-использования)

## 🗺️ Roadmap

### v1.1.0
- [ ] GraphQL subscriptions для real-time прогресса
- [ ] Persistent storage (PostgreSQL)
- [ ] Session status queries
- [ ] Operation cancellation
- [ ] Prometheus metrics

### v1.2.0
- [ ] Multi-signature wallet management
- [ ] Key resharing support
- [ ] Advanced access control
- [ ] Audit logging

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

См. LICENSE файл в корне проекта

## 📧 Контакты

Для вопросов и поддержки создавайте issue в репозитории.

---

**Документация**: [API.md](./API.md) | **Схема**: [schema.graphqls](./graph/schema.graphqls)
