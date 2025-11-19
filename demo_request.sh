#!/bin/bash

# Demo script to test TSS distributed key generation via GraphQL

echo "======================================"
echo "TSS Distributed Key Generation Demo"
echo "======================================"
echo ""
echo "Generating ECDSA key with 3 parties (threshold 2-of-3)..."
echo ""

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
  http://localhost:8080/query | jq '.'

echo ""
echo "======================================"
echo "Demo completed!"
echo "======================================"
