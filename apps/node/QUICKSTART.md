# ⚡ Quick Start — Запуск 3 нод за 3 команды

## 📦 Шаг 1: Запустить базы данных (Docker)

```bash
docker-compose up -d
```

Ждем 5 секунд пока БД инициализируются...

## 🏗️ Шаг 2: Собрать приложение (если не собрано)

```bash
go build -o mpc_node .
```

## 🚀 Шаг 3: Запустить все 3 ноды

```bash
./start-nodes.sh
```

---

## ✅ Готово!

Кластер запущен:
- **Node 1** — `localhost:50051` (DB: 5432)
- **Node 2** — `localhost:50052` (DB: 5433)
- **Node 3** — `localhost:50053` (DB: 5434)

---

## 🔍 Проверка

```bash
# Проверить статус
./check-nodes.sh

# Посмотреть логи
tail -f logs/node1.log
```

---

## 🛑 Остановка

```bash
# Остановить ноды
./stop-nodes.sh

# Остановить базы данных
docker-compose down
```

---

## 📚 Полная документация

- [CLUSTER_SETUP.md](CLUSTER_SETUP.md) — Подробная инструкция
- [REFACTORING.md](REFACTORING.md) — Описание рефакторинга

---

## 🧪 Тестирование (требуется grpcurl)

```bash
# Health check всех нод
grpcurl -plaintext localhost:50051 mpc.MPCNodeService/HealthCheck
grpcurl -plaintext localhost:50052 mpc.MPCNodeService/HealthCheck
grpcurl -plaintext localhost:50053 mpc.MPCNodeService/HealthCheck

# Node info
grpcurl -plaintext localhost:50051 mpc.MPCNodeService/GetNodeInfo
```

---

## 🎯 Что дальше?

1. **Keygen** — сгенерировать распределенный ключ между 3 нодами
2. **Signing** — создать threshold подпись (2-из-3)
3. **Production** — настроить TLS и безопасность

---

**Время запуска:** ~30 секунд
**Требования:** Docker, Go 1.24+
