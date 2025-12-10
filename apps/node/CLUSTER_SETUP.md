# 🚀 Запуск кластера из 3 MPC нод

Этот гайд описывает как запустить локальный кластер из 3 MPC нод с отдельными базами данных PostgreSQL.

## 📋 Предварительные требования

- Docker и Docker Compose установлены
- Go 1.24+ установлен
- (Опционально) grpcurl для тестирования

## 🗂️ Структура файлов

```
apps/node/
├── docker-compose.yml          # Docker Compose для 3 PostgreSQL
├── sql/
│   └── init.sql               # SQL схема для инициализации БД
├── config-node1.yaml          # Конфиг для Node 1
├── config-node2.yaml          # Конфиг для Node 2
├── config-node3.yaml          # Конфиг для Node 3
├── start-nodes.sh             # Скрипт запуска всех нод
├── stop-nodes.sh              # Скрипт остановки всех нод
└── check-nodes.sh             # Скрипт проверки статуса
```

## 🎯 Быстрый старт

### Шаг 1: Запустить базы данных

```bash
# Запустить все 3 PostgreSQL
docker-compose up -d

# Проверить что все БД запущены
docker-compose ps
```

Будут запущены:
- **Node 1 DB**: `localhost:5432` (database: mpc_node1)
- **Node 2 DB**: `localhost:5433` (database: mpc_node2)
- **Node 3 DB**: `localhost:5434` (database: mpc_node3)
- **pgAdmin**: `http://localhost:5050` (опционально)

### Шаг 2: Собрать приложение

```bash
go build -o mpc_node .
```

### Шаг 3: Запустить все ноды

```bash
./start-nodes.sh
```

Будут запущены:
- **Node 1**: `localhost:50051` (party1, DB port 5432)
- **Node 2**: `localhost:50052` (party2, DB port 5433)
- **Node 3**: `localhost:50053` (party3, DB port 5434)

### Шаг 4: Проверить статус

```bash
./check-nodes.sh
```

Вывод покажет статус каждой ноды:
```
=== MPC Nodes Status ===

Node 1 (port 50051):
  Process: ✓ Running (PID: 12345)
  gRPC:    ✓ Healthy

Node 2 (port 50052):
  Process: ✓ Running (PID: 12346)
  gRPC:    ✓ Healthy

Node 3 (port 50053):
  Process: ✓ Running (PID: 12347)
  gRPC:    ✓ Healthy
```

## 📊 Управление кластером

### Просмотр логов

```bash
# Все логи в реальном времени
tail -f logs/node*.log

# Логи конкретной ноды
tail -f logs/node1.log
tail -f logs/node2.log
tail -f logs/node3.log
```

### Остановка нод

```bash
./stop-nodes.sh
```

### Остановка баз данных

```bash
# Остановить, но сохранить данные
docker-compose stop

# Остановить и удалить контейнеры (данные сохраняются в volumes)
docker-compose down

# Полное удаление включая volumes (УДАЛИТ ВСЕ ДАННЫЕ!)
docker-compose down -v
```

### Перезапуск

```bash
# Перезапустить ноды
./stop-nodes.sh
./start-nodes.sh

# Перезапустить базы данных
docker-compose restart
```

## 🔍 Тестирование

### Проверка здоровья нод

```bash
# Node 1
grpcurl -plaintext localhost:50051 mpc.MPCNodeService/HealthCheck

# Node 2
grpcurl -plaintext localhost:50052 mpc.MPCNodeService/HealthCheck

# Node 3
grpcurl -plaintext localhost:50053 mpc.MPCNodeService/HealthCheck
```

### Получение информации о нодах

```bash
# Node 1
grpcurl -plaintext localhost:50051 mpc.MPCNodeService/GetNodeInfo

# Node 2
grpcurl -plaintext localhost:50052 mpc.MPCNodeService/GetNodeInfo

# Node 3
grpcurl -plaintext localhost:50053 mpc.MPCNodeService/GetNodeInfo
```

### Подключение к базам данных

```bash
# Node 1 Database
psql -h localhost -p 5432 -U postgres -d mpc_node1

# Node 2 Database
psql -h localhost -p 5433 -U postgres -d mpc_node2

# Node 3 Database
psql -h localhost -p 5434 -U postgres -d mpc_node3
```

Пароль: `postgres`

### Использование pgAdmin

1. Открыть http://localhost:5050
2. Войти:
   - Email: `admin@mpc.local`
   - Password: `admin`
3. Добавить серверы:
   - **Node 1**: Host: `postgres-node1`, Port: `5432`
   - **Node 2**: Host: `postgres-node2`, Port: `5432`
   - **Node 3**: Host: `postgres-node3`, Port: `5432`

## 📝 Конфигурация нод

### Node 1 (config-node1.yaml)
```yaml
node:
  id: node1
  party_id: party1
  port: 50051
database:
  port: 5432
  database: mpc_node1
```

### Node 2 (config-node2.yaml)
```yaml
node:
  id: node2
  party_id: party2
  port: 50052
database:
  port: 5433
  database: mpc_node2
```

### Node 3 (config-node3.yaml)
```yaml
node:
  id: node3
  party_id: party3
  port: 50053
database:
  port: 5434
  database: mpc_node3
```

## 🔧 Расширенные настройки

### Запуск отдельной ноды

```bash
# Node 1
./mpc_node -config config-node1.yaml

# Node 2
./mpc_node -config config-node2.yaml

# Node 3
./mpc_node -config config-node3.yaml
```

### Запуск с debug логами

```bash
./mpc_node -config config-node1.yaml -debug
```

### Переопределение параметров

```bash
# Изменить порт ноды
./mpc_node -config config-node1.yaml -port 50061

# Изменить party ID
./mpc_node -config config-node1.yaml -party-id custom_party
```

## 🛠️ Troubleshooting

### Ошибка: "address already in use"

```bash
# Проверить что занимает порт
lsof -i :50051
lsof -i :5432

# Остановить ноды
./stop-nodes.sh
```

### Ошибка подключения к БД

```bash
# Проверить что контейнеры запущены
docker-compose ps

# Проверить логи контейнера
docker-compose logs postgres-node1

# Перезапустить базы
docker-compose restart
```

### Ноды не отвечают

```bash
# Проверить процессы
ps aux | grep mpc_node

# Проверить логи
tail -100 logs/node1.log

# Перезапустить
./stop-nodes.sh
./start-nodes.sh
```

### Очистка всех данных

```bash
# Остановить все
./stop-nodes.sh
docker-compose down -v

# Удалить логи и ключи
rm -rf logs keys .node*.pid

# Пересоздать
mkdir -p logs keys/node1 keys/node2 keys/node3
docker-compose up -d
./start-nodes.sh
```

## 📊 Мониторинг

### Просмотр использования ресурсов

```bash
# Docker контейнеры
docker stats

# Процессы нод
ps aux | grep mpc_node
```

### Проверка дискового пространства

```bash
# Volumes Docker
docker system df -v

# Логи
du -sh logs/
```

## 🎓 Следующие шаги

После запуска кластера вы можете:

1. **Тестировать keygen** — запустить генерацию ключей между нодами
2. **Тестировать signing** — создать подпись используя threshold схему
3. **Мониторинг** — добавить метрики и алерты
4. **Production** — настроить TLS и безопасность

## 📚 Полезные команды

```bash
# Быстрый рестарт всего
docker-compose restart && ./stop-nodes.sh && ./start-nodes.sh

# Просмотр всех логов
tail -f logs/*.log

# Проверка всех портов
netstat -tlnp | grep -E "(5043|5044|5045|5432|5433|5434)"

# Экспорт базы данных
docker exec mpc-postgres-node1 pg_dump -U postgres mpc_node1 > backup.sql

# Импорт базы данных
cat backup.sql | docker exec -i mpc-postgres-node1 psql -U postgres mpc_node1
```

---

Готово! Теперь у вас запущен полноценный кластер из 3 MPC нод. 🎉
