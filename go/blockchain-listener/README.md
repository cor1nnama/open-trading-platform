# Blockchain Listener Service

This service listens for Transfer events from an Ethereum token contract and publishes them to Kafka.

## Environment Variables

Set these environment variables before running:

```bash
export ETH_RPC="https://mainnet.infura.io/v3/YOUR_PROJECT_ID"  # Ethereum RPC endpoint
export TOKEN_CONTRACT="0x..."  # Token contract address
export TOKEN_ABI_JSON='[{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"}]'  # Contract ABI (ERC20 Transfer event)
export KAFKA_URL="localhost:9092"  # Kafka broker URL
```

## Running the Service

```bash
go run main.go
```

## What it does

1. Connects to an Ethereum node via RPC
2. Subscribes to Transfer events from the specified token contract
3. Parses transfer data (from, to, value, transaction hash)
4. Publishes events to Kafka topic "real-estate-transfers"

## Kafka Message Format

Each message contains a JSON object:
```json
{
  "from": "0x...",
  "to": "0x...", 
  "value": "1000000000000000000",
  "txHash": "0x..."
}
```

## Integration

This service can be integrated with other microservices in the trading platform to:
- Update order status based on blockchain confirmations
- Trigger settlement processes
- Update account balances
- Generate notifications 