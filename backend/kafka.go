package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

type kafkaEvent struct {
	EventID string          `json:"event_id"`
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data"`
}

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

func ensureKafkaTopics() error {
	brokers := kafkaBrokers()
	if len(brokers) == 0 {
		return nil
	}
	connection, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		return err
	}
	defer connection.Close()
	controller, err := connection.Controller()
	if err != nil {
		return err
	}
	controllerAddress := net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))
	controllerConnection, err := kafka.Dial("tcp", controllerAddress)
	if err != nil {
		return err
	}
	defer controllerConnection.Close()
	topics := []string{
		kafkaTopic(),
		topicFromEnv("KAFKA_SEARCH_TOPIC", "search.sync"),
		topicFromEnv("KAFKA_INTERACTION_TOPIC", "interaction.event"),
		topicFromEnv("KAFKA_COMMENT_TOPIC", "comment.moderation"),
		topicFromEnv("KAFKA_NOTIFICATION_TOPIC", "notification"),
	}
	configs := make([]kafka.TopicConfig, 0, len(topics)*2)
	for _, topic := range topics {
		configs = append(configs,
			kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1},
			kafka.TopicConfig{Topic: topic + ".dlq", NumPartitions: 1, ReplicationFactor: 1},
		)
	}
	return controllerConnection.CreateTopics(configs...)
}

func publishKafka(topic string, key string, payload any) error {
	brokers := kafkaBrokers()
	if len(brokers) == 0 {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		WriteTimeout: 2 * time.Second,
	}
	defer writer.Close()
	return writer.WriteMessages(context.Background(), kafka.Message{Key: []byte(key), Value: data})
}

func publishGamePlay(gameID int) error {
	return publishEvent(kafkaTopic(), strconv.Itoa(gameID), "game.play", map[string]int{"game_id": gameID})
}

func publishEvent(topic, key, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return publishKafka(topic, key, kafkaEvent{EventID: newEventID(), Type: eventType, Data: data})
}

func newEventID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", bytes)
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
			message, err := reader.FetchMessage(context.Background())
			if err != nil {
				log.Printf("Kafka 读取游玩事件失败: %v", err)
				time.Sleep(time.Second)
				continue
			}
			var envelope kafkaEvent
			if json.Unmarshal(message.Value, &envelope) != nil || envelope.EventID == "" {
				_ = reader.CommitMessages(context.Background(), message)
				continue
			}
			var event struct {
				GameID int `json:"game_id"`
			}
			if json.Unmarshal(envelope.Data, &event) != nil || event.GameID <= 0 {
				_ = reader.CommitMessages(context.Background(), message)
				continue
			}
			var processErr error
			for attempt := 0; attempt < 3; attempt++ {
				processErr = processGamePlayEvent(db, envelope.EventID, event.GameID)
				if processErr == nil {
					break
				}
				time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
			}
			if processErr != nil {
				log.Printf("处理游玩事件失败: %v", processErr)
				_ = publishKafka(kafkaTopic()+".dlq", envelope.EventID, map[string]any{"event": envelope, "error": processErr.Error()})
				_ = reader.CommitMessages(context.Background(), message)
				continue
			}
			if game, err := findGame(db, event.GameID); err == nil {
				cache.zadd("game:hot:rank", game.Plays, strconv.Itoa(event.GameID))
				cache.hset("game:stats:"+strconv.Itoa(event.GameID), "plays", game.Plays)
				cache.del("game:detail:" + strconv.Itoa(event.GameID))
				cache.del("games:list:::plays")
			}
			_ = reader.CommitMessages(context.Background(), message)
		}
	}()
}

func processGamePlayEvent(db *sql.DB, eventID string, gameID int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT IGNORE INTO kafka_processed_events (event_id, topic) VALUES (?, ?)`, eventID, kafkaTopic())
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return tx.Commit()
	}
	if _, err := tx.Exec(`INSERT INTO game_play_events (game_id) VALUES (?)`, gameID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE games SET plays = plays + 1 WHERE id = ?`, gameID); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO game_analytics_daily (game_id, stat_date, plays) VALUES (?, CURDATE(), 1) ON DUPLICATE KEY UPDATE plays = plays + 1`, gameID); err != nil {
		return err
	}
	return tx.Commit()
}

func startPhaseThreeConsumers(db *sql.DB) {
	startEventConsumer(db, topicFromEnv("KAFKA_SEARCH_TOPIC", "search.sync"), "gamehub-search-sync", syncSearchDocument)
	startEventConsumer(db, topicFromEnv("KAFKA_COMMENT_TOPIC", "comment.moderation"), "gamehub-moderation", moderateContent)
	startEventConsumer(db, topicFromEnv("KAFKA_INTERACTION_TOPIC", "interaction.event"), "gamehub-analytics", aggregateInteraction)
	startEventConsumer(db, topicFromEnv("KAFKA_NOTIFICATION_TOPIC", "notification"), "gamehub-notifications", saveNotification)
}

func topicFromEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func startEventConsumer(db *sql.DB, topic, group string, handle func(*sql.DB, kafkaEvent) error) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  kafkaBrokers(),
		Topic:    topic,
		GroupID:  group,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	go func() {
		defer reader.Close()
		for {
			message, err := reader.FetchMessage(context.Background())
			if err != nil {
				log.Printf("Kafka 读取 %s 失败: %v", topic, err)
				time.Sleep(time.Second)
				continue
			}
			var event kafkaEvent
			if err := json.Unmarshal(message.Value, &event); err != nil || event.EventID == "" {
				_ = reader.CommitMessages(context.Background(), message)
				continue
			}
			claimed, err := claimKafkaEvent(db, event.EventID, topic)
			if err != nil {
				log.Printf("记录 %s 事件状态失败: %v", topic, err)
				continue
			}
			if !claimed {
				_ = reader.CommitMessages(context.Background(), message)
				continue
			}
			var handleErr error
			for attempt := 0; attempt < 3; attempt++ {
				handleErr = handle(db, event)
				if handleErr == nil {
					break
				}
				time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
			}
			if handleErr != nil {
				log.Printf("处理 %s 事件失败: %v", topic, handleErr)
				_, _ = db.Exec(`DELETE FROM kafka_processed_events WHERE event_id = ?`, event.EventID)
				_ = publishKafka(topic+".dlq", event.EventID, map[string]any{"event": event, "error": handleErr.Error()})
			}
			_ = reader.CommitMessages(context.Background(), message)
		}
	}()
}

func claimKafkaEvent(db *sql.DB, eventID, topic string) (bool, error) {
	result, err := db.Exec(`INSERT IGNORE INTO kafka_processed_events (event_id, topic) VALUES (?, ?)`, eventID, topic)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func syncSearchDocument(db *sql.DB, event kafkaEvent) error {
	var payload struct {
		Type   string `json:"type"`
		GameID int    `json:"game_id"`
		PostID int64  `json:"post_id"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return err
	}
	if payload.GameID > 0 {
		game, err := findGame(db, payload.GameID)
		if err != nil {
			return err
		}
		_, err = db.Exec(`INSERT INTO search_documents (entity_type, entity_id, title, content) VALUES ('game', ?, ?, ?) ON DUPLICATE KEY UPDATE title=VALUES(title), content=VALUES(content)`, game.ID, game.Title, game.Description)
		return err
	}
	if payload.PostID > 0 {
		post, err := findPost(db, payload.PostID)
		if err != nil {
			return err
		}
		_, err = db.Exec(`INSERT INTO search_documents (entity_type, entity_id, title, content) VALUES ('post', ?, ?, ?) ON DUPLICATE KEY UPDATE title=VALUES(title), content=VALUES(content)`, post.ID, post.Title, post.Body)
		return err
	}
	return nil
}

func moderateContent(db *sql.DB, event kafkaEvent) error {
	var payload struct {
		Type      string `json:"type"`
		CommentID int64  `json:"comment_id"`
		PostID    int64  `json:"post_id"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return err
	}
	entityType, entityID := payload.Type, payload.CommentID
	if entityID == 0 {
		entityID = payload.PostID
	}
	if entityID == 0 {
		return nil
	}
	status, reason := "approved", ""
	if strings.Contains(strings.ToLower(string(event.Data)), "spam") {
		status, reason = "flagged", "命中演示敏感词"
	}
	_, err := db.Exec(`INSERT INTO moderation_records (entity_type, entity_id, status, reason) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE status=VALUES(status), reason=VALUES(reason)`, entityType, entityID, status, reason)
	return err
}

func aggregateInteraction(db *sql.DB, event kafkaEvent) error {
	var payload struct {
		Type     string `json:"type"`
		GameID   int    `json:"game_id"`
		Liked    *bool  `json:"liked"`
		Favorite *bool  `json:"favorite"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil || payload.GameID == 0 {
		return err
	}
	column, delta := "", 1
	switch payload.Type {
	case "like":
		column = "likes"
		if payload.Liked != nil && !*payload.Liked {
			delta = -1
		}
	case "favorite":
		column = "favorites"
		if payload.Favorite != nil && !*payload.Favorite {
			delta = -1
		}
	case "comment":
		column = "comments"
	default:
		return nil
	}
	_, err := db.Exec("INSERT INTO game_analytics_daily (game_id, stat_date, "+column+") VALUES (?, CURDATE(), ?) ON DUPLICATE KEY UPDATE "+column+" = GREATEST(0, "+column+" + VALUES("+column+"))", payload.GameID, delta)
	return err
}

func saveNotification(db *sql.DB, event kafkaEvent) error {
	var payload struct {
		UserID  int    `json:"user_id"`
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil || payload.UserID == 0 || payload.Content == "" {
		return err
	}
	_, err := db.Exec(`INSERT INTO notifications (user_id, type, content) VALUES (?, ?, ?)`, payload.UserID, payload.Type, payload.Content)
	return err
}
