#!/bin/bash
# Quick deployment script for hackathon demo

echo "🚀 Quick Deploy Script"
echo ""

# 1. Build Move contract
echo "📦 Building Move contract..."
cd move && sui move build && cd ..

# 2. Start services
echo "🐳 Starting Docker services..."
docker run -d --name kafka -p 9092:9092 apache/kafka 2>/dev/null || echo "Kafka already running"
docker run -d --name redis -p 6379:6379 redis:alpine 2>/dev/null || echo "Redis already running"

# 3. Wait for services
echo "⏳ Waiting for services..."
sleep 5

# 4. Start gateway
echo "🚀 Starting gateway..."
echo "Set these environment variables first:"
echo "  export SUI_SIGNER_PRIVATE_KEY='your-key'"
echo "  export SUI_GAS_OBJECT_ID='your-gas-object'"
echo ""
echo "Then run: go run cmd/gateway/main.go"
