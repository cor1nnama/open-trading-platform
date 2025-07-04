# Blockchain Indexer Service

A comprehensive blockchain indexer that scans historical blocks, stores transfer events in SQLite, and provides a REST API for querying blockchain data.

## Features

- **Historical Indexing**: Scans all past blocks for Transfer events
- **Real-time Updates**: Listens for new blocks and indexes them immediately
- **SQLite Storage**: Stores all transfer events in a local database
- **REST API**: HTTP endpoints for querying transfer data
- **Kafka Integration**: Publishes events to Kafka for other services
- **Account Balance Calculation**: Calculates token balances for any address

## Environment Variables

```bash
export ETH_RPC="https://mainnet.infura.io/v3/YOUR_PROJECT_ID"  # Ethereum RPC endpoint
export TOKEN_CONTRACT="0x..."  # Token contract address
export TOKEN_ABI_JSON='[{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"}]'  # Contract ABI
export KAFKA_URL="localhost:9092"  # Kafka broker URL
export DB_PATH="blockchain_events.db"  # SQLite database path (optional, defaults to blockchain_events.db)
```

## Running the Service

```bash
go run main.go
```

The service will:
1. Create SQLite database and tables
2. Index all historical blocks (in batches of 1000)
3. Start listening for new blocks
4. Start HTTP server on port 8080

## REST API Endpoints

### Health Check
```bash
GET /health
```
Returns service status.

### Get Transfers by Address
```bash
GET /transfers/{address}
```
Returns all transfers (incoming and outgoing) for a specific address.

**Example:**
```bash
curl http://localhost:8080/transfers/0x742d35Cc6634C0532925a3b8D4C9db96C4b4d8b6
```

### Get Transfer by Transaction Hash
```bash
GET /transfer/{txHash}
```
Returns details of a specific transfer transaction.

**Example:**
```bash
curl http://localhost:8080/transfer/0x1234567890abcdef...
```

### Get Recent Transfers
```bash
GET /transfers?limit=100
```
Returns the most recent transfers (default: 100, max: configurable).

**Example:**
```bash
curl http://localhost:8080/transfers?limit=50
```

### Get Account Balance
```bash
GET /balance/{address}
```
Calculates and returns the current token balance for an address.

**Example:**
```bash
curl http://localhost:8080/balance/0x742d35Cc6634C0532925a3b8D4C9db96C4b4d8b6
```

## Database Schema

### Transfers Table
```sql
CREATE TABLE transfers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    value TEXT NOT NULL,
    tx_hash TEXT UNIQUE NOT NULL,
    block_number INTEGER NOT NULL,
    block_time INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## Integration with Trading Platform

This indexer can be integrated with your trading platform to:

1. **Order Settlement Verification**: Check if orders were settled on-chain
2. **Account Balance Tracking**: Monitor user token balances
3. **Transaction History**: Provide transfer history for users
4. **Real-time Notifications**: Use Kafka events for real-time updates
5. **Analytics**: Analyze trading patterns and volumes

## Architecture

```
Ethereum Blockchain
       ↓
   Indexer Service
       ↓
   ┌─────────────┐
   │   SQLite    │  ← Historical data storage
   │  Database   │
   └─────────────┘
       ↓
   ┌─────────────┐
   │   REST API  │  ← Query interface
   └─────────────┘
       ↓
   ┌─────────────┐
   │    Kafka    │  ← Real-time events
   └─────────────┘
       ↓
   Other Services (Order Service, User Service, etc.)
```

## Performance Considerations

- **Batch Processing**: Indexes blocks in batches of 1000 for efficiency
- **Rate Limiting**: 100ms delay between batches to avoid RPC limits
- **Database Indexes**: Optimized indexes for fast queries
- **Resume Capability**: Automatically resumes from last indexed block

## Monitoring

The service logs:
- Indexing progress (blocks processed)
- New blocks detected
- Database operations
- API requests
- Errors and warnings

## Scaling Considerations

For production use, consider:
- Using PostgreSQL instead of SQLite for better concurrency
- Implementing connection pooling
- Adding metrics and monitoring
- Using a message queue for better reliability
- Implementing retry logic for failed operations 