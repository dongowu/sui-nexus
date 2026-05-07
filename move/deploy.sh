#!/bin/bash
set -e

echo "🚀 Sui Move Contract Deployment Script"
echo "========================================"

# Check if sui CLI is installed
if ! command -v sui &> /dev/null; then
    echo "❌ Sui CLI not found. Install it first:"
    echo "   cargo install --locked --git https://github.com/MystenLabs/sui.git --branch testnet sui"
    exit 1
fi

# Check if in correct directory
if [ ! -f "Move.toml" ]; then
    echo "❌ Move.toml not found. Run this script from the move/ directory"
    exit 1
fi

echo ""
echo "📦 Building Move contract..."
sui move build

echo ""
echo "🌐 Deploying to Sui Testnet..."
DEPLOY_OUTPUT=$(sui client publish --gas-budget 100000000 --json)

PACKAGE_ID=$(echo $DEPLOY_OUTPUT | jq -r '.objectChanges[] | select(.type=="published") | .packageId')

if [ -z "$PACKAGE_ID" ]; then
    echo "❌ Deployment failed"
    exit 1
fi

echo ""
echo "✅ Deployment successful!"
echo "📝 Package ID: $PACKAGE_ID"
echo ""
echo "💾 Saving to .env file..."
echo "SUI_MEMORY_PACKAGE_ID=$PACKAGE_ID" >> ../.env

echo ""
echo "🎉 Done! Add this to your environment:"
echo "   export SUI_MEMORY_PACKAGE_ID=\"$PACKAGE_ID\""
