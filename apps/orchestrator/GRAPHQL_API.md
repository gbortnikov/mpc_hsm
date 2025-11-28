# GraphQL API - MPC Оркестратор

## Endpoint

```
POST /graphql
```

## Queries (Запросы)

### Health Check (Проверка состояния)

```graphql
query {
  healthCheck {
    status
    timestamp
    version
    uptime
    nodes {
      total
      online
      offline
    }
    sessions {
      active
      pending
      completed
      failed
    }
  }
}
```

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

### Получить онлайн ноды

```graphql
query {
  onlineNodes {
    id
    partyId
    address
    status
  }
}
```

### Получить все сессии

```graphql
query {
  sessions {
    id
    type
    status
    participants
    threshold
    createdAt
    completedAt
    result {
      success
      message
      data
    }
  }
}
```

### Получить сессию по ID

```graphql
query {
  session(id: "session-123") {
    id
    type
    status
    participants
    threshold
    createdAt
    completedAt
    result {
      success
      message
      data
    }
  }
}
```

### Получить активные сессии

```graphql
query {
  activeSessions {
    id
    type
    status
    participants
    threshold
  }
}
```

### Получить ожидающие сообщения

```graphql
query {
  pendingMessages(partyId: "party-1") {
    id
    sessionId
    fromParty
    toParties
    isBroadcast
    round
    payload
    timestamp
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
  unregisterNode(id: "node-123")
}
```

### Обновление статуса ноды

```graphql
mutation {
  updateNodeStatus(id: "node-123", status: ONLINE) {
    id
    status
    lastSeen
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
  }
}
```

### Присоединение к сессии

```graphql
mutation {
  joinSession(sessionId: "session-123", partyId: "party-1") {
    id
    status
    participants
  }
}
```

### Отмена сессии

```graphql
mutation {
  cancelSession(id: "session-123")
}
```

### Запуск генерации ключей

```graphql
mutation {
  startKeygen(sessionId: "session-123") {
    id
    type
    status
  }
}
```

### Запуск подписания

```graphql
mutation {
  startSigning(input: {
    keyId: "key-123"
    message: "0xabcdef..."
    participants: ["party-1", "party-2"]
  }) {
    id
    type
    status
    participants
  }
}
```

### Отправка сообщения

```graphql
mutation {
  sendMessage(input: {
    sessionId: "session-123"
    toParties: ["party-2", "party-3"]
    payload: "base64encodedpayload..."
  }) {
    id
    sessionId
    fromParty
    toParties
    isBroadcast
    round
    timestamp
  }
}
```

### Подтверждение получения сообщения

```graphql
mutation {
  acknowledgeMessage(messageId: "msg-123", partyId: "party-1")
}
```

---

## Subscriptions (Подписки)

### Изменение статуса ноды

```graphql
subscription {
  nodeStatusChanged {
    id
    partyId
    status
    lastSeen
  }
}
```

### Обновление сессии

```graphql
subscription {
  sessionUpdated(sessionId: "session-123") {
    id
    status
    result {
      success
      message
      data
    }
  }
}
```

### Поток сообщений

```graphql
subscription {
  messages(partyId: "party-1", sessionId: "session-123") {
    id
    fromParty
    round
    payload
    timestamp
  }
}
```

---

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

## Примеры использования с curl

### Health Check

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ healthCheck { status version uptime nodes { total online } sessions { active completed } } }"}'
```

### Query

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ nodes { id partyId status } }"}'
```

### Mutation

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
