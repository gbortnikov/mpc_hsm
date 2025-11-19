# MPC HSM - Distributed Key Generation Demo

Демонстрация распределенной генерации ECDSA ключей с использованием Threshold Signature Scheme (TSS) через GraphQL API.

## Описание

Этот проект демонстрирует:
- **Распределенную генерацию ключей** с использованием библиотеки `tss-lib` от BNB Chain
- **3 участника** (parties) для совместной генерации ключа
- **Порог 2-из-3** - для создания подписи требуется минимум 2 из 3 участников
- **ECDSA** на кривой secp256k1 (используется в Bitcoin, Ethereum)
- **GraphQL API** для удобного взаимодействия

## Технологии

- **Go 1.25**
- **gqlgen** - GraphQL сервер для Go
- **tss-lib v2** - Threshold Signature Scheme библиотека от BNB Chain
- **ECDSA** на кривой secp256k1

## Структура проекта

```
mpc_hsm/
├── server.go                  # GraphQL сервер
├── graph/
│   ├── schema.graphqls        # GraphQL схема
│   ├── schema.resolvers.go    # GraphQL резолверы
│   ├── resolver.go            # Базовый резолвер
│   └── model/
│       └── models_gen.go      # Сгенерированные модели
├── tss/
│   └── keygen.go              # TSS координатор для генерации ключей
└── demo_request.sh            # Демо скрипт
```

## Установка и запуск

### 1. Установка зависимостей

```bash
go mod download
```

### 2. Сборка проекта

```bash
go build -o mpc_hsm_server .
```

### 3. Запуск сервера

```bash
./mpc_hsm_server
```

Сервер запустится на `http://localhost:8080`

### 4. Запуск демо

```bash
./demo_request.sh
```

## GraphQL API

### Endpoint

- **Playground**: http://localhost:8080/
- **Query endpoint**: http://localhost:8080/query

### Мутация: generateDistributedKey

Генерирует распределенный ECDSA ключ с использованием TSS протокола.

#### Запрос

```graphql
mutation GenerateKey($input: KeygenInput!) {
  generateDistributedKey(input: $input) {
    publicKey
    privateKey
    address
    partyIds
    shares {
      partyId
      shareId
      shareValue
    }
    threshold
    totalShares
    success
    message
  }
}
```

#### Переменные

```json
{
  "input": {
    "threshold": 2,
    "totalParties": 3
  }
}
```

#### Параметры

- `threshold` (Int!) - Минимальное количество участников для создания подписи
- `totalParties` (Int!) - Общее количество участников

#### Ответ

```json
{
  "data": {
    "generateDistributedKey": {
      "publicKey": "06b7a71691413b789d44da1177e44694b9043e80e9c6982fb32fcb125cec29f085713c7c498d57d9ec4270bb12a0423e59bf8616e448cefc534e78946cc8d2bb",
      "partyIds": [
        "party_1",
        "party_2",
        "party_3"
      ],
      "threshold": 2,
      "totalShares": 3,
      "success": true,
      "message": "Successfully generated distributed key with 3 parties and threshold 2"
    }
  }
}
```

#### Поля ответа

- `publicKey` (String!) - Публичный ключ в hex формате (координаты X и Y)
- `privateKey` (String!) - **[ДЕМО]** Восстановленный приватный ключ (только для демонстрации!)
- `address` (String!) - Ethereum адрес, полученный из публичного ключа
- `partyIds` ([String!]!) - Идентификаторы всех участников
- `shares` ([PartyShare!]!) - **[ДЕМО]** Секретные доли для каждого участника (только для демонстрации!)
  - `partyId` (String!) - Идентификатор участника
  - `shareId` (String!) - ID доли в hex формате
  - `shareValue` (String!) - Секретное значение доли в hex формате
- `threshold` (Int!) - Используемый порог
- `totalShares` (Int!) - Общее количество долей
- `success` (Boolean!) - Статус успешности операции
- `message` (String!) - Сообщение о результате

## Как это работает

### 1. Фазы протокола

#### Offline фаза (Pre-параметры)
```go
coordinator.GeneratePreParams()
```
Генерация больших простых чисел и параметров Paillier для каждого участника. Может выполняться заранее.

#### Online фаза

1. **Инициализация участников**
   ```go
   coordinator.InitializeParties()
   ```
   - Создание уникальных PartyID для каждого участника
   - Создание контекста для обмена сообщениями
   - Инициализация локальных party объектов

2. **Выполнение протокола**
   ```go
   coordinator.RunKeygen()
   ```
   - Запуск всех участников параллельно
   - Маршрутизация сообщений между участниками
   - Обработка broadcast и point-to-point сообщений

3. **Верификация долей**
   ```go
   coordinator.VerifyShares()
   ```
   - Проверка корректности сгенерированных долей
   - Проверка целостности публичного ключа

### 2. Архитектура координатора

`KeygenCoordinator` управляет всеми тремя участниками локально, симулируя распределенную систему:

```go
type KeygenCoordinator struct {
    parties       []*keygen.LocalParty  // Все участники
    partyIDs      []*tss.PartyID        // Идентификаторы
    outCh         chan tss.Message      // Канал исходящих сообщений
    endCh         chan *keygen.LocalPartySaveData  // Результаты
    errCh         chan *tss.Error       // Ошибки
    threshold     int                   // Порог подписи
    totalParties  int                   // Количество участников
}
```

### 3. Маршрутизация сообщений

Координатор обрабатывает два типа сообщений:

- **Broadcast**: Отправляются всем участникам
- **Point-to-point**: Отправляются конкретному получателю

```go
func (kc *KeygenCoordinator) routeMessages() {
    for msg := range kc.outCh {
        if msg.GetTo() == nil {
            // Broadcast всем
        } else {
            // Point-to-point конкретному получателю
        }
    }
}
```

## Безопасность

### ⚠️ КРИТИЧЕСКОЕ ПРЕДУПРЕЖДЕНИЕ

**НИКОГДА не используйте эту реализацию в продакшене!**

Эта демонстрация содержит следующие небезопасные практики:

1. **Восстановление приватного ключа** - приватный ключ восстанавливается из долей и выводится в логах. В реальном MPC/TSS приватный ключ **НИКОГДА** не должен существовать в полном виде!

2. **Вывод секретных долей** - все секретные доли (shares) возвращаются через API. В реальной системе каждый участник должен хранить только свою долю и **НИКОГДА** не раскрывать её!

3. **Все участники в одном процессе** - симуляция распределенной системы в одном процессе компрометирует всю идею MPC.

4. **Отсутствие защиты долей** - доли ключа хранятся в памяти без шифрования и передаются открытым текстом.

### Текущая реализация (Демо)

⚠️ **Внимание**: Текущая реализация создана **исключительно для обучения и демонстрации** работы TSS протокола.

Основные проблемы безопасности:
- Приватный ключ восстанавливается и выводится (функция `ReconstructPrivateKey`)
- Секретные доли всех участников возвращаются через API (поле `shares`)
- Все участники работают в одном процессе
- Доли не защищены и не персистентны
- Отсутствует аутентификация и TLS
- Секретные данные передаются открытым текстом

### Продакшн требования

Для реального использования требуется:

1. **УДАЛИТЬ функцию ReconstructPrivateKey** - приватный ключ не должен восстанавливаться!
2. **Изолированные участники** - каждый party в отдельном процессе/машине/HSM
3. **Безопасный транспорт** - TLS 1.3 с взаимной аутентификацией
4. **Защищенное хранилище** - HSM или зашифрованное хранилище для долей
5. **Надежная широковещательная рассылка** - с проверкой хэшей
6. **Обработка таймаутов** - защита от зависания
7. **Аудит безопасности** - перед использованием в продакшене

## Протокол TSS

Реализация основана на работе:
**"Fast Multiparty Threshold ECDSA with Fast Trustless Setup"**
*Rosario Gennaro and Steven Goldfeder (CCS 2018)*

### Свойства протокола

- ✅ **Без доверенного дилера** - ключ никогда не существует целиком
- ✅ **Threshold подписи** - t+1 из n участников могут подписать
- ✅ **Приватность** - секретные доли не раскрываются
- ✅ **Устойчивость к сговору** - до t участников могут быть скомпрометированы
- ✅ **Resharing** - можно перераспределить доли без изменения ключа

## Примеры использования

### cURL

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation GenerateKey($input: KeygenInput!) { generateDistributedKey(input: $input) { publicKey privateKey address partyIds shares { partyId shareId shareValue } threshold totalShares success message } }",
    "variables": {
      "input": {
        "threshold": 2,
        "totalParties": 3
      }
    }
  }' \
  http://localhost:8080/query
```

### GraphQL Playground

1. Откройте http://localhost:8080/
2. Вставьте запрос:

```graphql
mutation {
  generateDistributedKey(input: {
    threshold: 2,
    totalParties: 3
  }) {
    publicKey
    privateKey
    address
    partyIds
    shares {
      partyId
      shareId
      shareValue
    }
    threshold
    totalShares
    success
    message
  }
}
```

3. Нажмите "Play"

## Следующие шаги

Для расширения функционала можно добавить:

1. **Signing** - создание распределенных подписей
2. **Resharing** - перераспределение долей при изменении участников
3. **Persistent storage** - сохранение долей ключей
4. **Network transport** - реальная сеть между участниками
5. **EdDSA support** - поддержка кривой Ed25519
6. **Web interface** - UI для управления

## Лицензия

MIT

## Ссылки

- [tss-lib GitHub](https://github.com/bnb-chain/tss-lib)
- [Gennaro-Goldfeder Paper](https://eprint.iacr.org/2019/114.pdf)
- [gqlgen](https://gqlgen.com/)
