package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/segmentio/kafka-go"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Ethereum struct {
		RPCURL        string `yaml:"rpc_url"`
		TokenContract string `yaml:"token_contract"`
		TokenABIJSON  string `yaml:"token_abi_json"`
	} `yaml:"ethereum"`
	Kafka struct {
		URL   string `yaml:"url"`
		Topic string `yaml:"topic"`
	} `yaml:"kafka"`
	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`
}

func loadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func main() {
	// Load config.yaml
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Connect to Ethereum node
	client, err := ethclient.Dial(cfg.Ethereum.RPCURL)
	if err != nil {
		log.Fatalf("failed to connect to Ethereum RPC: %v", err)
	}
	tokenAddr := common.HexToAddress(cfg.Ethereum.TokenContract)

	// Parse contract ABI
	contractABI, err := abi.JSON(strings.NewReader(cfg.Ethereum.TokenABIJSON))
	if err != nil {
		log.Fatalf("invalid ABI: %v", err)
	}
	transferSig := contractABI.Events["Transfer"].ID

	// Set up Kafka writer
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{cfg.Kafka.URL},
		Topic:   cfg.Kafka.Topic,
	})
	defer writer.Close()

	// Subscribe to transfer events
	query := ethereum.FilterQuery{
		Addresses: []common.Address{tokenAddr},
		Topics:    [][]common.Hash{{transferSig}},
		FromBlock: big.NewInt(0),
	}
	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatalf("failed to subscribe to logs: %v", err)
	}

	// Process incoming logs
	for {
		select {
		case err := <-sub.Err():
			log.Printf("subscription error: %v", err)
			time.Sleep(time.Second * 5)
		case vLog := <-logs:
			var transfer struct {
				From  common.Address
				To    common.Address
				Value *big.Int
			}
			if len(vLog.Topics) < 3 {
				log.Printf("invalid transfer event: expected 3 topics, got %d", len(vLog.Topics))
				continue
			}
			transfer.From = common.HexToAddress(vLog.Topics[1].Hex())
			transfer.To = common.HexToAddress(vLog.Topics[2].Hex())
			if err := contractABI.UnpackIntoInterface(&transfer, "Transfer", vLog.Data); err != nil {
				log.Printf("ABI unpack error: %v", err)
				continue
			}
			// Publish event to Kafka
			value := fmt.Sprintf(`{"from":"%s","to":"%s","value":"%s","txHash":"%s"}`,
				transfer.From.Hex(), transfer.To.Hex(), transfer.Value.String(), vLog.TxHash.Hex())
			msg := kafka.Message{Key: []byte(vLog.TxHash.Hex()), Value: []byte(value)}
			if err := writer.WriteMessages(context.Background(), msg); err != nil {
				log.Printf("failed to write Kafka message: %v", err)
			}
		}
	}
}
