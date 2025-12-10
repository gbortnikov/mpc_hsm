#!/bin/bash

# Скрипт для остановки всех нод

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}=== Stopping MPC Nodes ===${NC}\n"

# Останавливаем ноды по PID
if [ -f ".node1.pid" ]; then
    PID=$(cat .node1.pid)
    if kill -0 $PID 2>/dev/null; then
        echo -e "${GREEN}Stopping Node 1 (PID: $PID)...${NC}"
        kill $PID
    else
        echo -e "${RED}Node 1 not running${NC}"
    fi
    rm .node1.pid
fi

if [ -f ".node2.pid" ]; then
    PID=$(cat .node2.pid)
    if kill -0 $PID 2>/dev/null; then
        echo -e "${GREEN}Stopping Node 2 (PID: $PID)...${NC}"
        kill $PID
    else
        echo -e "${RED}Node 2 not running${NC}"
    fi
    rm .node2.pid
fi

if [ -f ".node3.pid" ]; then
    PID=$(cat .node3.pid)
    if kill -0 $PID 2>/dev/null; then
        echo -e "${GREEN}Stopping Node 3 (PID: $PID)...${NC}"
        kill $PID
    else
        echo -e "${RED}Node 3 not running${NC}"
    fi
    rm .node3.pid
fi

sleep 1

echo -e "\n${GREEN}All nodes stopped${NC}"
