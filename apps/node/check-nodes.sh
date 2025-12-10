#!/bin/bash

# Скрипт для проверки статуса нод

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=== MPC Nodes Status ===${NC}\n"

check_node() {
    local name=$1
    local port=$2
    local pid_file=$3

    echo -e "${YELLOW}$name (port $port):${NC}"

    # Проверяем процесс
    if [ -f "$pid_file" ]; then
        PID=$(cat $pid_file)
        if kill -0 $PID 2>/dev/null; then
            echo -e "  Process: ${GREEN}✓ Running${NC} (PID: $PID)"
        else
            echo -e "  Process: ${RED}✗ Not running${NC}"
            return 1
        fi
    else
        echo -e "  Process: ${RED}✗ Not running${NC}"
        return 1
    fi

    # Проверяем gRPC
    if command -v grpcurl &> /dev/null; then
        if grpcurl -plaintext localhost:$port mpc.MPCNodeService/HealthCheck &> /dev/null; then
            echo -e "  gRPC:    ${GREEN}✓ Healthy${NC}"
        else
            echo -e "  gRPC:    ${RED}✗ Unreachable${NC}"
        fi
    else
        echo -e "  gRPC:    ${YELLOW}? (grpcurl not installed)${NC}"
    fi

    echo ""
}

check_node "Node 1" "50051" ".node1.pid"
check_node "Node 2" "50052" ".node2.pid"
check_node "Node 3" "50053" ".node3.pid"

echo -e "${YELLOW}Databases:${NC}"
if docker ps | grep -q mpc-postgres-node1; then
    echo -e "  Node 1 DB: ${GREEN}✓ Running${NC} (port 5432)"
else
    echo -e "  Node 1 DB: ${RED}✗ Not running${NC}"
fi

if docker ps | grep -q mpc-postgres-node2; then
    echo -e "  Node 2 DB: ${GREEN}✓ Running${NC} (port 5433)"
else
    echo -e "  Node 2 DB: ${RED}✗ Not running${NC}"
fi

if docker ps | grep -q mpc-postgres-node3; then
    echo -e "  Node 3 DB: ${GREEN}✓ Running${NC} (port 5434)"
else
    echo -e "  Node 3 DB: ${RED}✗ Not running${NC}"
fi
