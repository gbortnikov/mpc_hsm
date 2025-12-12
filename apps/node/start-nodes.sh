#!/bin/bash

# Скрипт для запуска всех трех MPC нод

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== MPC Node Cluster Startup ===${NC}\n"
# создание бинарника
go build -o mpc_node .
# Создание директорий для ключей
mkdir -p keys/node1 keys/node2 keys/node3
mkdir -p logs

echo -e "${GREEN}Starting Node 1 on port 50051...${NC}"
./mpc_node -config config-node1.yaml > logs/node1.log 2>&1 &
NODE1_PID=$!
echo "Node 1 PID: $NODE1_PID"

sleep 2

echo -e "${GREEN}Starting Node 2 on port 50052...${NC}"
./mpc_node -config config-node2.yaml > logs/node2.log 2>&1 &
NODE2_PID=$!
echo "Node 2 PID: $NODE2_PID"

sleep 2

echo -e "${GREEN}Starting Node 3 on port 50053...${NC}"
./mpc_node -config config-node3.yaml > logs/node3.log 2>&1 &
NODE3_PID=$!
echo "Node 3 PID: $NODE3_PID"

# Сохраняем PIDs в файл
echo "$NODE1_PID" > .node1.pid
echo "$NODE2_PID" > .node2.pid
echo "$NODE3_PID" > .node3.pid

sleep 2

echo -e "\n${GREEN}=== All nodes started ===${NC}"
echo -e "Node 1: http://localhost:50051 (PID: $NODE1_PID)"
echo -e "Node 2: http://localhost:50052 (PID: $NODE2_PID)"
echo -e "Node 3: http://localhost:50053 (PID: $NODE3_PID)"
echo -e "\n${YELLOW}Logs:${NC}"
echo -e "  tail -f logs/node1.log"
echo -e "  tail -f logs/node2.log"
echo -e "  tail -f logs/node3.log"
echo -e "\n${YELLOW}Stop nodes:${NC}"
echo -e "  ./stop-nodes.sh"
echo -e "\n${YELLOW}Check status:${NC}"
echo -e "  ./check-nodes.sh"
