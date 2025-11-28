#!/bin/bash

# Демонстрация распределённой генерации ключей
# Запускает 3 MPC ноды и оркестратор

set -e

cd "$(dirname "$0")"

echo "=== MPC HSM Distributed Demo ==="
echo ""

# Убиваем предыдущие процессы
pkill -f "mpc_node" 2>/dev/null || true
pkill -f "orchestrator" 2>/dev/null || true
sleep 1

# Запускаем 3 ноды
echo "Запуск MPC нод..."

cd node
./mpc_node --node-id=node_1 --party-id=party_1 --port=50051 &
NODE1_PID=$!
echo "  Нода 1 запущена (PID: $NODE1_PID, порт: 50051)"

./mpc_node --node-id=node_2 --party-id=party_2 --port=50052 &
NODE2_PID=$!
echo "  Нода 2 запущена (PID: $NODE2_PID, порт: 50052)"

./mpc_node --node-id=node_3 --party-id=party_3 --port=50053 &
NODE3_PID=$!
echo "  Нода 3 запущена (PID: $NODE3_PID, порт: 50053)"

sleep 2

# Запускаем оркестратор
echo ""
echo "Запуск оркестратора..."
cd ../orchestrator
./orchestrator &
ORCH_PID=$!
echo "  Оркестратор запущен (PID: $ORCH_PID, порт: 8081)"

sleep 2

echo ""
echo "=== Все компоненты запущены ==="
echo ""
echo "GraphQL Playground: http://localhost:8081"
echo ""
echo "Пример использования:"
echo ""
echo "1. Регистрация нод:"
cat << 'EOF'
mutation {
  node1: registerNode(input: {partyId: "party_1", address: "localhost:50051", publicKey: ""}) { id partyId status }
  node2: registerNode(input: {partyId: "party_2", address: "localhost:50052", publicKey: ""}) { id partyId status }
  node3: registerNode(input: {partyId: "party_3", address: "localhost:50053", publicKey: ""}) { id partyId status }
}
EOF

echo ""
echo "2. Создание сессии keygen:"
cat << 'EOF'
mutation {
  createSession(input: {
    type: KEYGEN
    participants: ["party_1", "party_2", "party_3"]
    threshold: 2
  }) {
    id
    status
    participants
    threshold
  }
}
EOF

echo ""
echo "3. Запуск keygen:"
cat << 'EOF'
mutation {
  startKeygen(sessionId: "session_1") {
    id
    status
  }
}
EOF

echo ""
echo "Для остановки нажмите Ctrl+C"
echo ""

# Ожидаем завершения
cleanup() {
    echo ""
    echo "Остановка компонентов..."
    kill $NODE1_PID $NODE2_PID $NODE3_PID $ORCH_PID 2>/dev/null || true
    echo "Готово."
}

trap cleanup EXIT
wait
