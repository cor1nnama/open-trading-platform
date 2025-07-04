package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/segmentio/kafka-go"
)

type TransferEvent struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"`
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	BlockTime   uint64 `json:"blockTime"`
}

type Indexer struct {
	client       *ethclient.Client
	db           *sql.DB
	contractAddr common.Address
	contractABI  abi.ABI
	writer       *kafka.Writer
}

func main() {
	// Load environment variables
	rpcURL := os.Getenv("ETH_RPC")
	tokenAddr := common.HexToAddress(os.Getenv("TOKEN_CONTRACT"))
	kafkaURL := os.Getenv("KAFKA_URL")
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "blockchain_events.db"
	}

	// Connect to Ethereum
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("Failed to connect to Ethereum: %v", err)
	}

	// Parse contract ABI
	abiJSON := os.Getenv("TOKEN_ABI_JSON")
	contractABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		log.Fatalf("Invalid ABI: %v", err)
	}

	// Initialize database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create tables
	if err := createTables(db); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	// Set up Kafka writer
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{kafkaURL},
		Topic:   "real-estate-transfers",
	})
	defer writer.Close()

	// Create indexer
	indexer := &Indexer{
		client:       client,
		db:           db,
		contractAddr: tokenAddr,
		contractABI:  contractABI,
		writer:       writer,
	}

	// Start indexing in background
	go indexer.startIndexing()

	// Start HTTP server
	router := gin.Default()
	setupRoutes(router, indexer)

	log.Println("Starting HTTP server on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func createTables(db *sql.DB) error {
	// Create transfers table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS transfers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_address TEXT NOT NULL,
			to_address TEXT NOT NULL,
			value TEXT NOT NULL,
			tx_hash TEXT UNIQUE NOT NULL,
			block_number INTEGER NOT NULL,
			block_time INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create index for faster queries
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_transfers_from ON transfers(from_address);
		CREATE INDEX IF NOT EXISTS idx_transfers_to ON transfers(to_address);
		CREATE INDEX IF NOT EXISTS idx_transfers_block ON transfers(block_number);
		CREATE INDEX IF NOT EXISTS idx_transfers_tx_hash ON transfers(tx_hash);
	`)
	return err
}

func (idx *Indexer) startIndexing() {
	// Get the last indexed block
	lastBlock, err := idx.getLastIndexedBlock()
	if err != nil {
		log.Printf("Error getting last indexed block: %v", err)
		lastBlock = 0 // Start from beginning if error
	}

	// Get current block
	currentBlock, err := idx.client.BlockNumber(context.Background())
	if err != nil {
		log.Fatalf("Failed to get current block: %v", err)
	}

	log.Printf("Starting indexing from block %d to %d", lastBlock+1, currentBlock)

	// Index blocks in batches
	batchSize := uint64(1000)
	for fromBlock := lastBlock + 1; fromBlock <= currentBlock; fromBlock += batchSize {
		toBlock := fromBlock + batchSize - 1
		if toBlock > currentBlock {
			toBlock = currentBlock
		}

		if err := idx.indexBlockRange(fromBlock, toBlock); err != nil {
			log.Printf("Error indexing blocks %d-%d: %v", fromBlock, toBlock, err)
			continue
		}

		log.Printf("Indexed blocks %d-%d", fromBlock, toBlock)
		time.Sleep(100 * time.Millisecond) // Rate limiting
	}

	// Start listening for new blocks
	idx.listenForNewBlocks()
}

func (idx *Indexer) indexBlockRange(fromBlock, toBlock uint64) error {
	transferSig := idx.contractABI.Events["Transfer"].ID

	query := ethereum.FilterQuery{
		Addresses: []common.Address{idx.contractAddr},
		Topics:    [][]common.Hash{{transferSig}},
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
	}

	logs, err := idx.client.FilterLogs(context.Background(), query)
	if err != nil {
		return err
	}

	for _, vLog := range logs {
		if err := idx.processTransferEvent(vLog); err != nil {
			log.Printf("Error processing transfer event: %v", err)
			continue
		}
	}

	return nil
}

func (idx *Indexer) processTransferEvent(vLog types.Log) error {
	if len(vLog.Topics) < 3 {
		return fmt.Errorf("invalid transfer event: expected 3 topics, got %d", len(vLog.Topics))
	}

	var transfer struct {
		From  common.Address
		To    common.Address
		Value *big.Int
	}

	transfer.From = common.HexToAddress(vLog.Topics[1].Hex())
	transfer.To = common.HexToAddress(vLog.Topics[2].Hex())

	if err := idx.contractABI.UnpackIntoInterface(&transfer, "Transfer", vLog.Data); err != nil {
		return err
	}

	// Get block info
	block, err := idx.client.BlockByHash(context.Background(), vLog.BlockHash)
	if err != nil {
		return err
	}

	// Store in database
	_, err = idx.db.Exec(`
		INSERT OR IGNORE INTO transfers (from_address, to_address, value, tx_hash, block_number, block_time)
		VALUES (?, ?, ?, ?, ?, ?)
	`, transfer.From.Hex(), transfer.To.Hex(), transfer.Value.String(), vLog.TxHash.Hex(), vLog.BlockNumber, block.Time())

	if err != nil {
		return err
	}

	// Publish to Kafka
	event := TransferEvent{
		From:        transfer.From.Hex(),
		To:          transfer.To.Hex(),
		Value:       transfer.Value.String(),
		TxHash:      vLog.TxHash.Hex(),
		BlockNumber: vLog.BlockNumber,
		BlockTime:   block.Time(),
	}

	eventJSON, _ := json.Marshal(event)
	msg := kafka.Message{
		Key:   []byte(vLog.TxHash.Hex()),
		Value: eventJSON,
	}

	return idx.writer.WriteMessages(context.Background(), msg)
}

func (idx *Indexer) getLastIndexedBlock() (uint64, error) {
	var lastBlock uint64
	err := idx.db.QueryRow("SELECT COALESCE(MAX(block_number), 0) FROM transfers").Scan(&lastBlock)
	return lastBlock, err
}

func (idx *Indexer) listenForNewBlocks() {
	headers := make(chan *types.Header)
	sub, err := idx.client.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		log.Fatalf("Failed to subscribe to new blocks: %v", err)
	}

	for {
		select {
		case err := <-sub.Err():
			log.Printf("Subscription error: %v", err)
			time.Sleep(5 * time.Second)
		case header := <-headers:
			if err := idx.indexBlockRange(header.Number.Uint64(), header.Number.Uint64()); err != nil {
				log.Printf("Error indexing new block %d: %v", header.Number.Uint64(), err)
			}
		}
	}
}

func setupRoutes(router *gin.Engine, idx *Indexer) {
	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	// Get transfers by address
	router.GET("/transfers/:address", func(c *gin.Context) {
		address := c.Param("address")
		transfers, err := idx.getTransfersByAddress(address)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, transfers)
	})

	// Get transfers by transaction hash
	router.GET("/transfer/:txHash", func(c *gin.Context) {
		txHash := c.Param("txHash")
		transfer, err := idx.getTransferByTxHash(txHash)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if transfer == nil {
			c.JSON(404, gin.H{"error": "Transfer not found"})
			return
		}
		c.JSON(200, transfer)
	})

	// Get recent transfers
	router.GET("/transfers", func(c *gin.Context) {
		limitStr := c.DefaultQuery("limit", "100")
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			limit = 100
		}
		transfers, err := idx.getRecentTransfers(limit)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, transfers)
	})

	// Get account balance
	router.GET("/balance/:address", func(c *gin.Context) {
		address := c.Param("address")
		balance, err := idx.getAccountBalance(address)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"address": address, "balance": balance})
	})
}

func (idx *Indexer) getTransfersByAddress(address string) ([]TransferEvent, error) {
	rows, err := idx.db.Query(`
		SELECT from_address, to_address, value, tx_hash, block_number, block_time
		FROM transfers
		WHERE from_address = ? OR to_address = ?
		ORDER BY block_number DESC
	`, address, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transfers []TransferEvent
	for rows.Next() {
		var t TransferEvent
		err := rows.Scan(&t.From, &t.To, &t.Value, &t.TxHash, &t.BlockNumber, &t.BlockTime)
		if err != nil {
			return nil, err
		}
		transfers = append(transfers, t)
	}
	return transfers, nil
}

func (idx *Indexer) getTransferByTxHash(txHash string) (*TransferEvent, error) {
	var t TransferEvent
	err := idx.db.QueryRow(`
		SELECT from_address, to_address, value, tx_hash, block_number, block_time
		FROM transfers
		WHERE tx_hash = ?
	`, txHash).Scan(&t.From, &t.To, &t.Value, &t.TxHash, &t.BlockNumber, &t.BlockTime)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (idx *Indexer) getRecentTransfers(limit int) ([]TransferEvent, error) {
	rows, err := idx.db.Query(`
		SELECT from_address, to_address, value, tx_hash, block_number, block_time
		FROM transfers
		ORDER BY block_number DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transfers []TransferEvent
	for rows.Next() {
		var t TransferEvent
		err := rows.Scan(&t.From, &t.To, &t.Value, &t.TxHash, &t.BlockNumber, &t.BlockTime)
		if err != nil {
			return nil, err
		}
		transfers = append(transfers, t)
	}
	return transfers, nil
}

func (idx *Indexer) getAccountBalance(address string) (string, error) {
	// Calculate balance by summing all incoming transfers minus outgoing transfers
	var balance big.Int

	// Sum incoming transfers
	var incoming big.Int
	err := idx.db.QueryRow(`
		SELECT COALESCE(SUM(CAST(value AS INTEGER)), 0)
		FROM transfers
		WHERE to_address = ?
	`, address).Scan(&incoming)
	if err != nil {
		return "0", err
	}

	// Sum outgoing transfers
	var outgoing big.Int
	err = idx.db.QueryRow(`
		SELECT COALESCE(SUM(CAST(value AS INTEGER)), 0)
		FROM transfers
		WHERE from_address = ?
	`, address).Scan(&outgoing)
	if err != nil {
		return "0", err
	}

	balance.Sub(&incoming, &outgoing)
	return balance.String(), nil
}
