# GraphQL API - MPC Оркестратор

## Endpoint

```
POST /graphql
```

## Типы данных

### NodeStatus (Статус ноды)
- `ONLINE` - нода онлайн
- `OFFLINE` - нода оффлайн
- `BUSY` - нода занята
- `ERROR` - ошибка

### SessionType (Тип сессии)
- `KEYGEN` - генерация ключей
- `SIGNING` - подписание
- `RESHARING` - перераспределение долей

### SessionStatus (Статус сессии)
- `PENDING` - ожидание
- `IN_PROGRESS` - в процессе
- `COMPLETED` - завершена
- `FAILED` - ошибка

---

## Queries (Запросы)

### Получить все ноды

```graphql
query {
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

### Получить ноду по ID

```graphql
query {
  node(id: "node-123") {
    id
    partyId
    address
    publicKey
    status
    lastSeen
  }
}
```

---

## Mutations (Мутации)

### Регистрация ноды

```graphql
mutation {
  registerNode(input: {
    partyId: "party-1"
    address: "localhost:8081"
    publicKey: "04abc123..."
  }) {
    id
    partyId
    address
    publicKey
    status
    lastSeen
  }
}
```

### Удаление ноды

```graphql
mutation {
  deleteNode(id: "node-123") {
    id
    partyId
    status
  }
}
```

### Создание сессии

```graphql
mutation {
  createSession(input: {
    type: KEYGEN
    participants: ["party-1", "party-2", "party-3"]
    threshold: 2
  }) {
    id
    type
    status
    participants
    threshold
    createdAt
    completedAt
  }
}
```

---

## Примеры использования с curl

### Получить все ноды

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ nodes { id partyId address status lastSeen } }"}'
```

### Получить ноду по ID

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ node(id: \"node-123\") { id partyId address publicKey status } }"}'
```

### Регистрация ноды

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation($input: RegisterNodeInput!) { registerNode(input: $input) { id partyId status } }",
    "variables": {
      "input": {
        "partyId": "party-1",
        "address": "localhost:8081",
        "publicKey": "04abc123..."
      }
    }
  }'
```

### Создание сессии генерации ключей

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation($input: CreateSessionInput!) { createSession(input: $input) { id type status participants threshold } }",
    "variables": {
      "input": {
        "type": "KEYGEN",
        "participants": ["party-1", "party-2", "party-3"],
        "threshold": 2
      }
    }
  }'
```

### Создание сессии подписания

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation($input: CreateSessionInput!) { createSession(input: $input) { id type status participants threshold } }",
    "variables": {
      "input": {
        "type": "SIGNING",
        "participants": ["party-1", "party-2"],
        "threshold": 2
      }
    }
  }'
```

### Удаление ноды

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { deleteNode(id: \"node-123\") { id partyId status } }"
  }'
```
