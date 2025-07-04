# Blockchain Services

This directory contains two complementary blockchain services for the Open Trading Platform:

## Services Overview

### 1. Blockchain Listener (`blockchain-listener/`)
**Purpose**: Real-time event streaming
- Listens for new Transfer events as they happen
- Publishes events to Kafka for immediate consumption
- Lightweight and fast for real-time applications

### 2. Blockchain Indexer (`blockchain-indexer/`)
**Purpose**: Historical data and querying
- Indexes all historical Transfer events
- Stores data in SQLite database
- Provides REST API for querying blockchain data
- Calculates account balances
- Also publishes to Kafka for real-time updates

## Architecture

```
Ethereum Blockchain
       ↓
   ┌─────────────────┐    ┌─────────────────┐
   │   Listener      │    │    Indexer      │
   │  (Real-time)    │    │  (Historical)   │
   └─────────────────┘    └─────────────────┘
       ↓                        ↓
   ┌─────────────────┐    ┌─────────────────┐
   │     Kafka       │    │   SQLite DB     │
   │  (Events)       │    │  (Storage)      │
   └─────────────────┘    └─────────────────┘
       ↓                        ↓
   ┌─────────────────┐    ┌─────────────────┐
   │  Other Services │    │   REST API      │
   │  (Consumers)    │    │  (Queries)      │
   └─────────────────┘    └─────────────────┘
```

## When to Use Each Service

### Use Blockchain Listener When:
- You need immediate notifications of new transfers
- You want to react to events in real-time
- You don't need historical data
- You want minimal resource usage

### Use Blockchain Indexer When:
- You need to query historical transfer data
- You want to calculate account balances
- You need transaction history for users
- You want to verify order settlements
- You need analytics and reporting

### Use Both When:
- You need both real-time updates and historical queries
- You're building a complete trading platform
- You want redundancy and reliability

## Quick Start

### 1. Set Environment Variables
```bash
export ETH_RPC="https://mainnet.infura.io/v3/YOUR_PROJECT_ID"
export TOKEN_CONTRACT="0x..."  # Your token contract address
export TOKEN_ABI_JSON='[{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"}]'
export KAFKA_URL="localhost:9092"
```

### 2. Start the Listener (Real-time)
```bash
cd blockchain-listener
go run main.go
```

### 3. Start the Indexer (Historical + API)
```bash
cd blockchain-indexer
go run main.go
```

### 4. Test the Services

**Test Listener (Kafka events):**
```bash
# Use a Kafka consumer to see real-time events
kafka-console-consumer --bootstrap-server localhost:9092 --topic real-estate-transfers
```

**Test Indexer (REST API):**
```bash
# Get recent transfers
curl http://localhost:8080/transfers?limit=10

# Get transfers for an address
curl http://localhost:8080/transfers/0x742d35Cc6634C0532925a3b8D4C9db96C4b4d8b6

# Get account balance
curl http://localhost:8080/balance/0x742d35Cc6634C0532925a3b8D4C9db96C4b4d8b6
```

## Integration with Trading Platform

### Order Service Integration
```go
// Check if order was settled on-chain
func (s *OrderService) CheckSettlement(orderID string) error {
    // Query indexer API
    resp, err := http.Get(fmt.Sprintf("http://localhost:8080/transfer/%s", txHash))
    if err != nil {
        return err
    }
    // Process settlement status
}
```

### User Service Integration
```go
// Get user's token balance
func (s *UserService) GetBalance(userAddress string) (string, error) {
    resp, err := http.Get(fmt.Sprintf("http://localhost:8080/balance/%s", userAddress))
    if err != nil {
        return "0", err
    }
    // Parse balance response
}
```

### Real-time Notifications
```go
// Listen for new transfers
func (s *NotificationService) ListenForTransfers() {
    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers: []string{"localhost:9092"},
        Topic:   "real-estate-transfers",
    })
    // Process real-time events
}
```

## Production Deployment

### Docker Compose Example
```yaml
version: '3.8'
services:
  blockchain-listener:
    build: ./blockchain-listener
    environment:
      - ETH_RPC=${ETH_RPC}
      - TOKEN_CONTRACT=${TOKEN_CONTRACT}
      - TOKEN_ABI_JSON=${TOKEN_ABI_JSON}
      - KAFKA_URL=kafka:9092
    depends_on:
      - kafka

  blockchain-indexer:
    build: ./blockchain-indexer
    environment:
      - ETH_RPC=${ETH_RPC}
      - TOKEN_CONTRACT=${TOKEN_CONTRACT}
      - TOKEN_ABI_JSON=${TOKEN_ABI_JSON}
      - KAFKA_URL=kafka:9092
      - DB_PATH=/data/blockchain_events.db
    ports:
      - "8080:8080"
    volumes:
      - blockchain_data:/data
    depends_on:
      - kafka

  kafka:
    image: confluentinc/cp-kafka:latest
    environment:
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
    depends_on:
      - zookeeper

  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181

volumes:
  blockchain_data:
```

## Monitoring and Health Checks

### Listener Health
- Monitor Kafka message production
- Check Ethereum connection status
- Log subscription errors

### Indexer Health
- REST API health endpoint: `GET /health`
- Monitor indexing progress
- Check database connection
- Track API response times

## Performance Considerations

### Listener
- Minimal resource usage
- Real-time processing
- No storage overhead

### Indexer
- Initial indexing can take time (depending on blockchain size)
- Database storage requirements
- API response caching for frequently accessed data
- Consider using PostgreSQL for production scale

## Security Considerations

- Secure RPC endpoints
- Validate contract addresses
- Rate limit API endpoints
- Use HTTPS in production
- Implement authentication for API endpoints
- Secure database access

## Troubleshooting

### Common Issues

1. **RPC Connection Failed**
   - Check ETH_RPC environment variable
   - Verify network connectivity
   - Check RPC provider limits

2. **Kafka Connection Failed**
   - Verify KAFKA_URL
   - Check Kafka broker status
   - Ensure topic exists

3. **Indexing Slow**
   - Increase batch size
   - Reduce rate limiting
   - Use faster RPC endpoint

4. **API Timeouts**
   - Optimize database queries
   - Add indexes
   - Implement caching