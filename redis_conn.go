package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"github.com/olivere/elastic/v7"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error .env file: %v", err)
	}

	esClient, err := elastic.NewClient(
		elastic.SetURL(os.Getenv("Elasticsearch_URL")),
		elastic.SetSniff(false),
		elastic.SetBasicAuth(os.Getenv("Elasticsearch_Username"), os.Getenv("Elasticsearch_Password")),
	)
	if err != nil {
		log.Fatalf("Elasticsearch client Error: %v", err)
	}

	var redisConfigs []redisOptions
	i := 1
	for {
		urlKey := fmt.Sprintf("Redis_URL_%d", i)
		passKey := fmt.Sprintf("Redis_Password_%d", i)
		url := os.Getenv(urlKey)
		pass := os.Getenv(passKey)
		if url == "" {
			break
		}

		config := redisOptions{
			Addr:     url,
			Password: pass,
			DB:       0,
		}
		redisConfigs = append(redisConfigs, config)

		i++
	}

	dateFormat := "2006-01-02"

	for _, config := range redisConfigs {

		client := createRedisClient(config.Addr, config.Password, config.DB)

		info, err := client.Info(context.Background()).Result()
		if err != nil {
			log.Fatalf("INFO command Fault Request: %v", err)
		}

		now := time.Now()
		dateString := now.Format(dateFormat)
		indexName := fmt.Sprintf("redis_monitoring_%s", dateString)

		doc := map[string]interface{}{
			"redis_version":     extractFieldValue(info, "redis_version"),
			"os":                extractFieldValue(info, "os"),
			"uptime_in_days":    extractFieldValue(info, "uptime_in_days"),
			"connected_clients": extractFieldValue(info, "connected_clients"),
			"maxclients":        extractFieldValue(info, "maxclients"),
			"role":              extractFieldValue(info, "role"),
			"connected_slaves":  extractFieldValue(info, "connected_slaves"),
			"timestamp":         now,
		}

		_, err = esClient.Index().
			Index(indexName).
			BodyJson(doc).
			Do(context.Background())
		if err != nil {
			log.Fatalf("Error indexing document: %v", err)
		}

		err = client.Close()
		if err != nil {
			log.Fatalf("Error at Redis: %v", err)
		}
	}

	esClient.Stop()
}

type redisOptions struct {
	Addr     string
	Password string
	DB       int
}

func createRedisClient(addr, password string, db int) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	return client
}

func extractFieldValue(info, field string) string {
	fieldIndex := strings.Index(info, field+":")
	if fieldIndex == -1 {
		return ""
	}
	lineEndIndex := strings.Index(info[fieldIndex:], "\n")
	if lineEndIndex == -1 {
		return ""
	}
	valueStartIndex := fieldIndex + len(field) + 1
	valueEndIndex := fieldIndex + lineEndIndex
	if valueEndIndex > len(info) {
		valueEndIndex = len(info)
	}
	return strings.TrimSpace(info[valueStartIndex:valueEndIndex])
}

// todo: send logs to elasticsearch to get data from grafana : finito
// todo: write the other redis url's and requirepass to .env file : finito
// todo: connect to grafana and create dashboard for operational times : in process
