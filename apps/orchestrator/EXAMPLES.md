# MPC Orchestrator - Примеры запросов

Коллекция готовых к использованию примеров GraphQL запросов и интеграций.

## Содержание

1. [GraphQL Queries](#graphql-queries)
2. [cURL Examples](#curl-examples)
3. [JavaScript/TypeScript](#javascripttypescript)
4. [Python](#python)
5. [Go](#go)
6. [Postman Collection](#postman-collection)

---

## GraphQL Queries

### Базовые операции

#### 1. Регистрация одной ноды

```graphql
mutation RegisterSingleNode {
  registerNode(input: {
    address: "localhost:5001"
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

#### 2. Регистрация нескольких нод (batch)

```graphql
mutation RegisterMultipleNodes {
  node1: registerNode(input: {address: "localhost:5001"}) {
    id
    partyId
    status
  }
  node2: registerNode(input: {address: "localhost:5002"}) {
    id
    partyId
    status
  }
  node3: registerNode(input: {address: "localhost:5003"}) {
    id
    partyId
    status
  }
}
```

#### 3. Получить все ноды

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

#### 4. Получить конкретную ноду

```graphql
query GetNode {
  node(id: "node_1") {
    id
    partyId
    address
    status
  }
}
```

#### 5. Удалить ноду

```graphql
mutation DeleteNode {
  deleteNode(id: "node_3") {
    id
    partyId
    status
  }
}
```

---

### Keygen операции

#### 1. Простой keygen (2-of-3)

```graphql
mutation SimpleKeygen {
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

#### 2. Keygen с минимальным threshold (2-of-2)

```graphql
mutation MinimalKeygen {
  startKeygen(input: {
    participants: ["party1", "party2"]
    threshold: 2
  }) {
    sessionId
    publicKey
    address
  }
}
```

#### 3. Keygen с высоким threshold (4-of-5)

```graphql
mutation HighThresholdKeygen {
  startKeygen(input: {
    participants: ["party1", "party2", "party3", "party4", "party5"]
    threshold: 4
  }) {
    sessionId
    publicKey
    address
    threshold
    totalParties
  }
}
```

#### 4. Keygen со всеми зарегистрированными нодами

```graphql
mutation KeygenAllNodes {
  # Без указания participants - используются все ноды
  startKeygen(input: {
    threshold: 2
  }) {
    sessionId
    publicKey
    address
  }
}
```

---

### Signing операции

#### 1. Подписание hex-encoded сообщения

```graphql
mutation SignHexMessage {
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

#### 2. Подписание текстового сообщения

```graphql
mutation SignTextMessage {
  startSigning(input: {
    address: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb6"
    message: "Hello, MPC World!"
    participants: ["party1", "party3"]
  }) {
    sessionId
    signature
  }
}
```

#### 3. Подписание Ethereum транзакции

```graphql
mutation SignEthereumTx {
  startSigning(input: {
    address: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb6"
    # Serialized Ethereum transaction
    message: "0xf86c098504a817c800825208943535353535353535353535353535353535353535880de0b6b3a76400008025a028ef61340bd939bc2195fe537567866003e1a15d3c71ff63e1590620aa636276a067cbe9d8997f761aecb703304b3800ccf555c9f3dc64214b297fb1966a3b6d83"
    participants: ["party1", "party2", "party3"]
  }) {
    signature
    r
    s
    v
  }
}
```

#### 4. Подписание со всеми доступными участниками

```graphql
mutation SignWithAllParties {
  startSigning(input: {
    address: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb6"
    message: "0xabcdef123456"
    # Без participants - используются все доступные ноды
  }) {
    sessionId
    signature
  }
}
```

---

### Комбинированные операции

#### 1. Полный workflow в одном запросе

```graphql
mutation CompleteWorkflow {
  # Шаг 1: Регистрация
  n1: registerNode(input: {address: "localhost:5001"}) { partyId }
  n2: registerNode(input: {address: "localhost:5002"}) { partyId }
  n3: registerNode(input: {address: "localhost:5003"}) { partyId }
}

# Затем выполнить отдельно keygen
mutation KeygenStep {
  startKeygen(input: {
    participants: ["party1", "party2", "party3"]
    threshold: 2
  }) {
    address
  }
}

# И наконец signing
mutation SigningStep {
  startSigning(input: {
    address: "0x..." # из предыдущего шага
    message: "Transaction data"
    participants: ["party1", "party2"]
  }) {
    signature
  }
}
```

#### 2. Health check всех нод

```graphql
query HealthCheck {
  nodes {
    id
    partyId
    address
    status
    lastSeen
  }
}
```

---

## cURL Examples

### Регистрация ноды

```bash
curl -X POST http://localhost:8081/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { registerNode(input: {address: \"localhost:5001\"}) { id partyId status } }"
  }'
```

### Keygen

```bash
curl -X POST http://localhost:8081/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { startKeygen(input: {participants: [\"party1\", \"party2\", \"party3\"], threshold: 2}) { sessionId address publicKey } }"
  }'
```

### Signing

```bash
curl -X POST http://localhost:8081/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { startSigning(input: {address: \"0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb6\", message: \"0x1234567890abcdef\", participants: [\"party1\", \"party2\"]}) { signature r s v } }"
  }'
```

### Получить список нод

```bash
curl -X POST http://localhost:8081/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "query { nodes { id partyId status } }"
  }'
```

### Health check

```bash
curl http://localhost:8081/health
```

---

## JavaScript/TypeScript

### Установка зависимостей

```bash
npm install graphql-request graphql
```

### Базовый клиент

```typescript
import { GraphQLClient, gql } from 'graphql-request'

const ORCHESTRATOR_URL = 'http://localhost:8081/query'
const client = new GraphQLClient(ORCHESTRATOR_URL)

// Типы
interface Node {
  id: string
  partyId: string
  address: string
  status: 'ONLINE' | 'OFFLINE' | 'BUSY' | 'ERROR'
}

interface KeygenResult {
  sessionId: string
  publicKey: string
  address: string
  threshold: number
  totalParties: number
}

interface SigningResult {
  sessionId: string
  signature: string
  r: string
  s: string
  v: number
}
```

### Регистрация ноды

```typescript
async function registerNode(address: string): Promise<Node> {
  const mutation = gql`
    mutation RegisterNode($address: String!) {
      registerNode(input: { address: $address }) {
        id
        partyId
        address
        status
      }
    }
  `

  const data = await client.request<{ registerNode: Node }>(mutation, { address })
  return data.registerNode
}

// Использование
const node = await registerNode('localhost:5001')
console.log('Node registered:', node.partyId)
```

### Keygen

```typescript
async function startKeygen(
  participants: string[],
  threshold: number
): Promise<KeygenResult> {
  const mutation = gql`
    mutation StartKeygen($participants: [String!]!, $threshold: Int!) {
      startKeygen(input: { participants: $participants, threshold: $threshold }) {
        sessionId
        publicKey
        address
        threshold
        totalParties
      }
    }
  `

  const data = await client.request<{ startKeygen: KeygenResult }>(mutation, {
    participants,
    threshold
  })
  return data.startKeygen
}

// Использование
const result = await startKeygen(['party1', 'party2', 'party3'], 2)
console.log('Generated address:', result.address)
console.log('Public key:', result.publicKey)
```

### Signing

```typescript
async function startSigning(
  address: string,
  message: string,
  participants: string[]
): Promise<SigningResult> {
  const mutation = gql`
    mutation StartSigning(
      $address: String!
      $message: String!
      $participants: [String!]!
    ) {
      startSigning(
        input: {
          address: $address
          message: $message
          participants: $participants
        }
      ) {
        sessionId
        signature
        r
        s
        v
      }
    }
  `

  const data = await client.request<{ startSigning: SigningResult }>(mutation, {
    address,
    message,
    participants
  })
  return data.startSigning
}

// Использование
const signature = await startSigning(
  '0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb6',
  '0x1234567890abcdef',
  ['party1', 'party2']
)
console.log('Signature:', signature.signature)
```

### Получить список нод

```typescript
async function getNodes(): Promise<Node[]> {
  const query = gql`
    query GetNodes {
      nodes {
        id
        partyId
        address
        status
      }
    }
  `

  const data = await client.request<{ nodes: Node[] }>(query)
  return data.nodes
}

// Использование
const nodes = await getNodes()
console.log('Online nodes:', nodes.filter(n => n.status === 'ONLINE').length)
```

### Полный пример workflow

```typescript
async function completeWorkflow() {
  console.log('🚀 Starting MPC workflow...')

  // 1. Регистрация нод
  console.log('\n1️⃣ Registering nodes...')
  const addresses = ['localhost:5001', 'localhost:5002', 'localhost:5003']
  const nodes = await Promise.all(addresses.map(addr => registerNode(addr)))
  const partyIds = nodes.map(n => n.partyId)
  console.log('✅ Registered:', partyIds.join(', '))

  // 2. Keygen
  console.log('\n2️⃣ Generating key (2-of-3)...')
  const keygenResult = await startKeygen(partyIds, 2)
  console.log('✅ Generated address:', keygenResult.address)
  console.log('   Public key:', keygenResult.publicKey.slice(0, 20) + '...')

  // 3. Signing
  console.log('\n3️⃣ Signing transaction...')
  const signingResult = await startSigning(
    keygenResult.address,
    '0x1234567890abcdef',
    [partyIds[0], partyIds[1]]
  )
  console.log('✅ Signature:', signingResult.signature.slice(0, 20) + '...')
  console.log('   R:', signingResult.r.slice(0, 20) + '...')
  console.log('   S:', signingResult.s.slice(0, 20) + '...')
  console.log('   V:', signingResult.v)

  console.log('\n🎉 Workflow completed successfully!')
}

completeWorkflow().catch(console.error)
```

---

## Python

### Установка зависимостей

```bash
pip install requests
```

### Базовый клиент

```python
import requests
from typing import List, Dict, Any, Optional

class MPCOrchestratorClient:
    def __init__(self, url: str = "http://localhost:8081/query"):
        self.url = url

    def _request(self, query: str, variables: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """Выполнить GraphQL запрос"""
        response = requests.post(
            self.url,
            json={"query": query, "variables": variables or {}}
        )
        response.raise_for_status()
        result = response.json()

        if "errors" in result:
            raise Exception(f"GraphQL error: {result['errors']}")

        return result["data"]

    def register_node(self, address: str) -> Dict[str, Any]:
        """Зарегистрировать ноду"""
        query = """
        mutation RegisterNode($address: String!) {
            registerNode(input: {address: $address}) {
                id
                partyId
                address
                status
            }
        }
        """
        data = self._request(query, {"address": address})
        return data["registerNode"]

    def get_nodes(self) -> List[Dict[str, Any]]:
        """Получить список нод"""
        query = """
        query GetNodes {
            nodes {
                id
                partyId
                address
                status
            }
        }
        """
        data = self._request(query)
        return data["nodes"]

    def start_keygen(self, participants: List[str], threshold: int) -> Dict[str, Any]:
        """Запустить генерацию ключа"""
        query = """
        mutation StartKeygen($participants: [String!]!, $threshold: Int!) {
            startKeygen(input: {participants: $participants, threshold: $threshold}) {
                sessionId
                publicKey
                address
                threshold
                totalParties
            }
        }
        """
        data = self._request(query, {
            "participants": participants,
            "threshold": threshold
        })
        return data["startKeygen"]

    def start_signing(
        self,
        address: str,
        message: str,
        participants: List[str]
    ) -> Dict[str, Any]:
        """Запустить подписание"""
        query = """
        mutation StartSigning($address: String!, $message: String!, $participants: [String!]!) {
            startSigning(input: {
                address: $address,
                message: $message,
                participants: $participants
            }) {
                sessionId
                signature
                r
                s
                v
            }
        }
        """
        data = self._request(query, {
            "address": address,
            "message": message,
            "participants": participants
        })
        return data["startSigning"]

    def delete_node(self, node_id: str) -> Dict[str, Any]:
        """Удалить ноду"""
        query = """
        mutation DeleteNode($id: ID!) {
            deleteNode(id: $id) {
                id
                partyId
                status
            }
        }
        """
        data = self._request(query, {"id": node_id})
        return data["deleteNode"]
```

### Примеры использования

```python
# Создаем клиент
client = MPCOrchestratorClient("http://localhost:8081/query")

# Регистрируем ноды
print("Registering nodes...")
addresses = ["localhost:5001", "localhost:5002", "localhost:5003"]
nodes = [client.register_node(addr) for addr in addresses]
party_ids = [node["partyId"] for node in nodes]
print(f"Registered: {', '.join(party_ids)}")

# Генерируем ключ
print("\nGenerating key...")
keygen_result = client.start_keygen(party_ids, threshold=2)
print(f"Generated address: {keygen_result['address']}")
print(f"Public key: {keygen_result['publicKey'][:20]}...")

# Подписываем сообщение
print("\nSigning transaction...")
signing_result = client.start_signing(
    address=keygen_result['address'],
    message="0x1234567890abcdef",
    participants=[party_ids[0], party_ids[1]]
)
print(f"Signature: {signing_result['signature'][:20]}...")
print(f"R: {signing_result['r'][:20]}...")
print(f"S: {signing_result['s'][:20]}...")
print(f"V: {signing_result['v']}")

# Проверяем статус нод
print("\nChecking node status...")
nodes = client.get_nodes()
online_count = sum(1 for n in nodes if n["status"] == "ONLINE")
print(f"Online nodes: {online_count}/{len(nodes)}")
```

### Полный пример с обработкой ошибок

```python
def complete_mpc_workflow():
    """Полный MPC workflow с обработкой ошибок"""
    try:
        client = MPCOrchestratorClient()

        # 1. Регистрация
        print("🚀 Starting MPC workflow...")
        print("\n1️⃣ Registering nodes...")
        addresses = ["localhost:5001", "localhost:5002", "localhost:5003"]
        nodes = []
        for addr in addresses:
            try:
                node = client.register_node(addr)
                nodes.append(node)
                print(f"   ✅ Registered {node['partyId']} at {addr}")
            except Exception as e:
                print(f"   ❌ Failed to register {addr}: {e}")
                raise

        party_ids = [n["partyId"] for n in nodes]

        # 2. Keygen
        print("\n2️⃣ Generating key (2-of-3 threshold)...")
        keygen_result = client.start_keygen(party_ids, 2)
        print(f"   ✅ Generated address: {keygen_result['address']}")
        print(f"   📝 Session ID: {keygen_result['sessionId']}")

        # 3. Signing
        print("\n3️⃣ Signing transaction...")
        signing_result = client.start_signing(
            keygen_result['address'],
            "0x1234567890abcdef",
            [party_ids[0], party_ids[1]]
        )
        print(f"   ✅ Signature: {signing_result['signature'][:66]}...")
        print(f"   📝 Session ID: {signing_result['sessionId']}")

        print("\n🎉 Workflow completed successfully!")
        return signing_result

    except Exception as e:
        print(f"\n❌ Error: {e}")
        raise

if __name__ == "__main__":
    complete_mpc_workflow()
```

---

## Go

### Базовый клиент

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type OrchestratorClient struct {
    URL string
}

type GraphQLRequest struct {
    Query     string                 `json:"query"`
    Variables map[string]interface{} `json:"variables,omitempty"`
}

type GraphQLResponse struct {
    Data   json.RawMessage `json:"data"`
    Errors []struct {
        Message string `json:"message"`
    } `json:"errors,omitempty"`
}

func NewClient(url string) *OrchestratorClient {
    return &OrchestratorClient{URL: url}
}

func (c *OrchestratorClient) request(query string, variables map[string]interface{}, result interface{}) error {
    reqBody := GraphQLRequest{
        Query:     query,
        Variables: variables,
    }

    jsonData, err := json.Marshal(reqBody)
    if err != nil {
        return err
    }

    resp, err := http.Post(c.URL, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return err
    }

    var gqlResp GraphQLResponse
    if err := json.Unmarshal(body, &gqlResp); err != nil {
        return err
    }

    if len(gqlResp.Errors) > 0 {
        return fmt.Errorf("GraphQL error: %s", gqlResp.Errors[0].Message)
    }

    return json.Unmarshal(gqlResp.Data, result)
}

type Node struct {
    ID      string `json:"id"`
    PartyID string `json:"partyId"`
    Address string `json:"address"`
    Status  string `json:"status"`
}

type KeygenResult struct {
    SessionID    string `json:"sessionId"`
    PublicKey    string `json:"publicKey"`
    Address      string `json:"address"`
    Threshold    int    `json:"threshold"`
    TotalParties int    `json:"totalParties"`
}

type SigningResult struct {
    SessionID string `json:"sessionId"`
    Signature string `json:"signature"`
    R         string `json:"r"`
    S         string `json:"s"`
    V         int    `json:"v"`
}

func (c *OrchestratorClient) RegisterNode(address string) (*Node, error) {
    query := `
        mutation RegisterNode($address: String!) {
            registerNode(input: {address: $address}) {
                id
                partyId
                address
                status
            }
        }
    `

    var result struct {
        RegisterNode Node `json:"registerNode"`
    }

    err := c.request(query, map[string]interface{}{"address": address}, &result)
    return &result.RegisterNode, err
}

func (c *OrchestratorClient) StartKeygen(participants []string, threshold int) (*KeygenResult, error) {
    query := `
        mutation StartKeygen($participants: [String!]!, $threshold: Int!) {
            startKeygen(input: {participants: $participants, threshold: $threshold}) {
                sessionId
                publicKey
                address
                threshold
                totalParties
            }
        }
    `

    var result struct {
        StartKeygen KeygenResult `json:"startKeygen"`
    }

    err := c.request(query, map[string]interface{}{
        "participants": participants,
        "threshold":    threshold,
    }, &result)

    return &result.StartKeygen, err
}

func (c *OrchestratorClient) StartSigning(address, message string, participants []string) (*SigningResult, error) {
    query := `
        mutation StartSigning($address: String!, $message: String!, $participants: [String!]!) {
            startSigning(input: {
                address: $address,
                message: $message,
                participants: $participants
            }) {
                sessionId
                signature
                r
                s
                v
            }
        }
    `

    var result struct {
        StartSigning SigningResult `json:"startSigning"`
    }

    err := c.request(query, map[string]interface{}{
        "address":      address,
        "message":      message,
        "participants": participants,
    }, &result)

    return &result.StartSigning, err
}

func main() {
    client := NewClient("http://localhost:8081/query")

    // Регистрируем ноды
    fmt.Println("Registering nodes...")
    addresses := []string{"localhost:5001", "localhost:5002", "localhost:5003"}
    var partyIDs []string

    for _, addr := range addresses {
        node, err := client.RegisterNode(addr)
        if err != nil {
            fmt.Printf("Error registering %s: %v\n", addr, err)
            return
        }
        fmt.Printf("✅ Registered %s\n", node.PartyID)
        partyIDs = append(partyIDs, node.PartyID)
    }

    // Keygen
    fmt.Println("\nGenerating key...")
    keygen, err := client.StartKeygen(partyIDs, 2)
    if err != nil {
        fmt.Printf("Error in keygen: %v\n", err)
        return
    }
    fmt.Printf("✅ Generated address: %s\n", keygen.Address)

    // Signing
    fmt.Println("\nSigning transaction...")
    signing, err := client.StartSigning(keygen.Address, "0x1234567890abcdef", partyIDs[:2])
    if err != nil {
        fmt.Printf("Error in signing: %v\n", err)
        return
    }
    fmt.Printf("✅ Signature: %s...\n", signing.Signature[:66])

    fmt.Println("\n🎉 Workflow completed!")
}
```

---

## Postman Collection

### Import в Postman

Создайте новую коллекцию и добавьте следующие запросы:

**Base URL**: `http://localhost:8081/query`
**Method**: `POST`
**Headers**: `Content-Type: application/json`

### 1. Register Node

```json
{
  "query": "mutation RegisterNode($address: String!) { registerNode(input: {address: $address}) { id partyId status } }",
  "variables": {
    "address": "localhost:5001"
  }
}
```

### 2. Get All Nodes

```json
{
  "query": "query { nodes { id partyId address status lastSeen } }"
}
```

### 3. Start Keygen

```json
{
  "query": "mutation StartKeygen($participants: [String!]!, $threshold: Int!) { startKeygen(input: {participants: $participants, threshold: $threshold}) { sessionId publicKey address threshold totalParties } }",
  "variables": {
    "participants": ["party1", "party2", "party3"],
    "threshold": 2
  }
}
```

### 4. Start Signing

```json
{
  "query": "mutation StartSigning($address: String!, $message: String!, $participants: [String!]!) { startSigning(input: {address: $address, message: $message, participants: $participants}) { sessionId signature r s v } }",
  "variables": {
    "address": "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb6",
    "message": "0x1234567890abcdef",
    "participants": ["party1", "party2"]
  }
}
```

---

## Tips & Best Practices

### 1. Переиспользование соединений

```typescript
// ✅ Хорошо: один клиент на приложение
const client = new GraphQLClient(ORCHESTRATOR_URL)

// ❌ Плохо: создание нового клиента для каждого запроса
function badExample() {
  const client = new GraphQLClient(ORCHESTRATOR_URL)
  return client.request(...)
}
```

### 2. Обработка ошибок

```typescript
try {
  const result = await startKeygen(['party1', 'party2'], 2)
} catch (error) {
  if (error instanceof ClientError) {
    console.error('GraphQL errors:', error.response.errors)
  } else {
    console.error('Network error:', error.message)
  }
}
```

### 3. Таймауты

```python
# Python: добавьте timeout
response = requests.post(url, json=payload, timeout=60)
```

```typescript
// TypeScript: используйте AbortController
const controller = new AbortController()
setTimeout(() => controller.abort(), 60000)

fetch(url, {
  signal: controller.signal,
  // ...
})
```

### 4. Retry логика

```typescript
async function retryRequest<T>(
  fn: () => Promise<T>,
  maxRetries = 3
): Promise<T> {
  for (let i = 0; i < maxRetries; i++) {
    try {
      return await fn()
    } catch (error) {
      if (i === maxRetries - 1) throw error
      await new Promise(resolve => setTimeout(resolve, 1000 * (i + 1)))
    }
  }
  throw new Error('Max retries reached')
}

// Использование
const result = await retryRequest(() => startKeygen(['party1', 'party2'], 2))
```

---

Больше информации см. в [API.md](./API.md)
