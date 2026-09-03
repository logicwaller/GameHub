package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

func kafkaBrokers() []string {
	value := os.Getenv("KAFKA_BROKERS")
	if value == "" {
		value = "127.0.0.1:9092"
	}
	parts := strings.Split(value, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		if broker := strings.TrimSpace(part); broker != "" {
			brokers = append(brokers, broker)
		}
	}
	return brokers
}

func kafkaTopic() string {
	if value := os.Getenv("KAFKA_GAME_PLAY_TOPIC"); value != "" {
		return value
	}
	return "game.play"
}

func publishGamePlay(gameID int) error {
	brokers := kafkaBrokers()
	if len(brokers) == 0 {
		return nil
	}
	payload, err := json.Marshal(map[string]int{"game_id": gameID})
	if err != nil {
		return err
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        kafkaTopic(),
		Balancer:     &kafka.Hash{},
		WriteTimeout: 2 * time.Second,
	}
	defer writer.Close()
	return writer.WriteMessages(context.Background(), kafka.Message{Key: []byte(strconv.Itoa(gameID)), Value: payload})
}

func startGamePlayConsumer(db *sql.DB, cache *redisClient) {
	brokers := kafkaBrokers()
	if len(brokers) == 0 {
		return
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    kafkaTopic(),
		GroupID:  "gamehub-play-counter",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	go func() {
		defer reader.Close()
		for {
			message, err := reader.ReadMessage(context.Background())
			if err != nil {
				log.Printf("Kafka 读取游玩事件失败: %v", err)
				time.Sleep(time.Second)
				continue
			}
			var event struct {
				GameID int `json:"game_id"`
			}
			if json.Unmarshal(message.Value, &event) != nil || event.GameID <= 0 {
				continue
			}
			if _, err := db.Exec(`INSERT INTO game_play_events (game_id) VALUES (?)`, event.GameID); err != nil {
				log.Printf("记录游戏游玩事件失败: %v", err)
			}
			if _, err := db.Exec(`UPDATE games SET plays = plays + 1 WHERE id = ?`, event.GameID); err != nil {
				log.Printf("更新游戏游玩量失败: %v", err)
				continue
			}
			if game, err := findGame(db, event.GameID); err == nil {
				cache.zadd("game:hot:rank", game.Plays, strconv.Itoa(event.GameID))
				cache.del("game:detail:" + strconv.Itoa(event.GameID))
				cache.del("games:list:::plays")
			}
		}
	}()
}
