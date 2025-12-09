#!/bin/bash

# Скрипт для генерации TLS сертификатов для mTLS между оркестратором и нодами
# Использование: ./scripts/generate_certs.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERTS_DIR="$SCRIPT_DIR/../certs"

echo "🔐 Генерация TLS сертификатов для MPC Orchestrator"
echo "=================================================="

# Создаем директорию для сертификатов
mkdir -p "$CERTS_DIR"
cd "$CERTS_DIR"

# Цвета для вывода
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 1. Генерируем CA (Certificate Authority)
echo -e "${BLUE}[1/5]${NC} Генерация CA сертификата..."
openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
  -keyout ca.key -out ca.crt \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=MPC-Orchestrator/CN=MPC-CA" \
  2>/dev/null

echo -e "${GREEN}✓${NC} CA сертификат создан: ca.crt, ca.key"

# 2. Генерируем клиентский сертификат (для оркестратора)
echo -e "${BLUE}[2/5]${NC} Генерация клиентского сертификата для оркестратора..."
openssl req -newkey rsa:4096 -nodes \
  -keyout client.key -out client.csr \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=MPC-Orchestrator/CN=orchestrator-client" \
  2>/dev/null

openssl x509 -req -in client.csr -days 365 \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out client.crt \
  2>/dev/null

rm client.csr
echo -e "${GREEN}✓${NC} Клиентский сертификат создан: client.crt, client.key"

# 3. Генерируем серверный сертификат для node1
echo -e "${BLUE}[3/5]${NC} Генерация серверного сертификата для node1..."
openssl req -newkey rsa:4096 -nodes \
  -keyout node1.key -out node1.csr \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=MPC-Node/CN=node1.example.com" \
  2>/dev/null

# Создаем конфигурацию для SAN (Subject Alternative Names)
cat > node1.ext <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = node1.example.com
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF

openssl x509 -req -in node1.csr -days 365 \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out node1.crt -extfile node1.ext \
  2>/dev/null

rm node1.csr node1.ext
echo -e "${GREEN}✓${NC} Серверный сертификат node1 создан: node1.crt, node1.key"

# 4. Генерируем серверный сертификат для node2
echo -e "${BLUE}[4/5]${NC} Генерация серверного сертификата для node2..."
openssl req -newkey rsa:4096 -nodes \
  -keyout node2.key -out node2.csr \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=MPC-Node/CN=node2.example.com" \
  2>/dev/null

cat > node2.ext <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = node2.example.com
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF

openssl x509 -req -in node2.csr -days 365 \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out node2.crt -extfile node2.ext \
  2>/dev/null

rm node2.csr node2.ext
echo -e "${GREEN}✓${NC} Серверный сертификат node2 создан: node2.crt, node2.key"

# 5. Генерируем серверный сертификат для node3
echo -e "${BLUE}[5/5]${NC} Генерация серверного сертификата для node3..."
openssl req -newkey rsa:4096 -nodes \
  -keyout node3.key -out node3.csr \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=MPC-Node/CN=node3.example.com" \
  2>/dev/null

cat > node3.ext <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = node3.example.com
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF

openssl x509 -req -in node3.csr -days 365 \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out node3.crt -extfile node3.ext \
  2>/dev/null

rm node3.csr node3.ext ca.srl
echo -e "${GREEN}✓${NC} Серверный сертификат node3 создан: node3.crt, node3.key"

# Установка правильных прав доступа
chmod 600 *.key
chmod 644 *.crt

echo ""
echo "=================================================="
echo -e "${GREEN}✅ Все сертификаты успешно созданы!${NC}"
echo "=================================================="
echo ""
echo "Структура сертификатов:"
echo "  CA:"
echo "    - ca.crt (публичный CA сертификат)"
echo "    - ca.key (приватный ключ CA)"
echo ""
echo "  Orchestrator (клиент):"
echo "    - client.crt"
echo "    - client.key"
echo ""
echo "  Nodes (серверы):"
echo "    - node1.crt / node1.key"
echo "    - node2.crt / node2.key"
echo "    - node3.crt / node3.key"
echo ""
echo "📋 Следующие шаги:"
echo ""
echo "1. Скопируйте серверные сертификаты на ноды:"
echo "   scp certs/ca.crt certs/node1.{crt,key} node1:/path/to/node/certs/"
echo "   scp certs/ca.crt certs/node2.{crt,key} node2:/path/to/node/certs/"
echo "   scp certs/ca.crt certs/node3.{crt,key} node3:/path/to/node/certs/"
echo ""
echo "2. Настройте оркестратор в .env:"
echo "   TLS_ENABLED=true"
echo "   TLS_CERT_FILE=certs/client.crt"
echo "   TLS_KEY_FILE=certs/client.key"
echo "   TLS_CA_FILE=certs/ca.crt"
echo ""
echo "3. Настройте ноды для использования TLS в их конфигурации"
echo ""
echo "⚠️  ВАЖНО: Сертификаты действительны 365 дней"
echo ""
