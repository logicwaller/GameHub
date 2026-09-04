package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type replyRecord struct {
	ID           int64  `json:"id"`
	Text         string `json:"text"`
	Author       string `json:"author"`
	AuthorID     string `json:"authorId"`
	AuthorAvatar string `json:"authorAvatar"`
	CreatedAt    string `json:"createdAt"`
}

type postRecord struct {
	ID           int64         `json:"id"`
	Title        string        `json:"title"`
	Body         string        `json:"body"`
	Author       string        `json:"author"`
	AuthorID     string        `json:"authorId"`
	AuthorAvatar string        `json:"authorAvatar"`
	CreatedAt    string        `json:"createdAt"`
	Comments     []replyRecord `json:"comments"`
}

func openDatabase() (*sql.DB, error) {
	host, port, user, password, name := os.Getenv("MYSQL_HOST"), os.Getenv("MYSQL_PORT"), os.Getenv("MYSQL_USER"), os.Getenv("MYSQL_PASSWORD"), os.Getenv("MYSQL_DATABASE")
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "3306"
	}
	if user == "" {
		user = "root"
	}
	if name == "" {
		name = "gamehub"
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local", user, password, host, port, name)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := migratePhaseThree(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migratePhaseThree(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS kafka_processed_events (
			event_id VARCHAR(64) PRIMARY KEY,
			topic VARCHAR(120) NOT NULL,
			processed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS search_documents (
			entity_type VARCHAR(32) NOT NULL,
			entity_id BIGINT UNSIGNED NOT NULL,
			title VARCHAR(200) NOT NULL,
			content TEXT NOT NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (entity_type, entity_id),
			FULLTEXT KEY idx_search_documents_text (title, content)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS moderation_records (
			entity_type VARCHAR(32) NOT NULL,
			entity_id BIGINT UNSIGNED NOT NULL,
			status VARCHAR(20) NOT NULL,
			reason VARCHAR(255) NOT NULL DEFAULT '',
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (entity_type, entity_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
			user_id INT NOT NULL,
			type VARCHAR(40) NOT NULL,
			content VARCHAR(500) NOT NULL,
			is_read BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_notifications_user_created (user_id, created_at),
			CONSTRAINT fk_notifications_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS game_analytics_daily (
			game_id INT NOT NULL,
			stat_date DATE NOT NULL,
			plays INT NOT NULL DEFAULT 0,
			likes INT NOT NULL DEFAULT 0,
			favorites INT NOT NULL DEFAULT 0,
			comments INT NOT NULL DEFAULT 0,
			PRIMARY KEY (game_id, stat_date),
			CONSTRAINT fk_game_analytics_daily_game FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func registerUser(db *sql.DB, username, email, password string) (user, error) {
	result, err := db.Exec(`INSERT INTO users (username, email, password) VALUES (?, ?, ?)`, username, email, password)
	if err != nil {
		return user{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return user{}, err
	}
	return user{ID: int(id), Username: username, Email: email, Role: "user"}, nil
}

func findUser(db *sql.DB, username string) (user, string, error) {
	var u user
	var password string
	err := db.QueryRow(`SELECT id, username, email, password, role FROM users WHERE username = ?`, username).Scan(&u.ID, &u.Username, &u.Email, &password, &u.Role)
	return u, password, err
}

func findUserByID(db *sql.DB, id int) (user, error) {
	var u user
	err := db.QueryRow(`SELECT id, username, email, role FROM users WHERE id = ?`, id).Scan(&u.ID, &u.Username, &u.Email, &u.Role)
	return u, err
}

func listGames(db *sql.DB, authorID *int) ([]gameRecord, error) {
	query := `SELECT g.id, g.title, g.description, g.category, g.play_time, g.url, COALESCE(g.cover, ''), g.author_id, u.username, g.plays, g.likes, g.favorites, g.comments FROM games AS g JOIN users AS u ON u.id = g.author_id`
	args := []any{}
	if authorID != nil {
		query += ` WHERE g.author_id = ?`
		args = append(args, *authorID)
	}
	query += ` ORDER BY g.created_at DESC`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]gameRecord, 0)
	for rows.Next() {
		var item gameRecord
		if err := rows.Scan(&item.ID, &item.Title, &item.Description, &item.Category, &item.PlayTime, &item.URL, &item.Cover, &item.AuthorID, &item.Author, &item.Plays, &item.Likes, &item.Favorites, &item.Comments); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func findGame(db *sql.DB, id int) (gameRecord, error) {
	var item gameRecord
	err := db.QueryRow(`SELECT g.id, g.title, g.description, g.category, g.play_time, g.url, COALESCE(g.cover, ''), g.author_id, u.username, g.plays, g.likes, g.favorites, g.comments FROM games AS g JOIN users AS u ON u.id = g.author_id WHERE g.id = ?`, id).
		Scan(&item.ID, &item.Title, &item.Description, &item.Category, &item.PlayTime, &item.URL, &item.Cover, &item.AuthorID, &item.Author, &item.Plays, &item.Likes, &item.Favorites, &item.Comments)
	return item, err
}

func postAuthorID(db *sql.DB, postID int64) (int, error) {
	var userID sql.NullInt64
	err := db.QueryRow(`SELECT user_id FROM posts WHERE id = ?`, postID).Scan(&userID)
	if err != nil {
		return 0, err
	}
	if !userID.Valid {
		return 0, nil
	}
	return int(userID.Int64), nil
}

type dailyAnalytics struct {
	Date      string `json:"date"`
	Plays     int    `json:"plays"`
	Likes     int    `json:"likes"`
	Favorites int    `json:"favorites"`
	Comments  int    `json:"comments"`
}

func gameDailyAnalytics(db *sql.DB, authorID int, gameID *int, days int) ([]dailyAnalytics, error) {
	query := `SELECT DATE_FORMAT(a.stat_date, '%Y-%m-%d'), SUM(a.plays), SUM(a.likes), SUM(a.favorites), SUM(a.comments)
		FROM game_analytics_daily a JOIN games g ON g.id = a.game_id
		WHERE g.author_id = ? AND a.stat_date >= CURDATE() - INTERVAL ? DAY`
	args := []any{authorID, days - 1}
	if gameID != nil {
		query += ` AND a.game_id = ?`
		args = append(args, *gameID)
	}
	query += ` GROUP BY a.stat_date ORDER BY a.stat_date ASC`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dailyAnalytics, 0)
	for rows.Next() {
		var item dailyAnalytics
		if err := rows.Scan(&item.Date, &item.Plays, &item.Likes, &item.Favorites, &item.Comments); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func userNotifications(db *sql.DB, userID int) ([]map[string]any, error) {
	rows, err := db.Query(`SELECT id, type, content, is_read, DATE_FORMAT(created_at, '%Y-%m-%d %H:%i') FROM notifications WHERE user_id = ? ORDER BY created_at DESC LIMIT 30`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var eventType, content, createdAt string
		var isRead bool
		if err := rows.Scan(&id, &eventType, &content, &isRead, &createdAt); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "type": eventType, "content": content, "read": isRead, "createdAt": createdAt})
	}
	return items, rows.Err()
}

func createGame(db *sql.DB, input gameRecord, authorID int) (gameRecord, error) {
	result, err := db.Exec(`INSERT INTO games (title, description, category, play_time, url, cover, author_id) VALUES (?, ?, ?, ?, ?, ?, ?)`, input.Title, input.Description, input.Category, input.PlayTime, input.URL, input.Cover, authorID)
	if err != nil {
		return gameRecord{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return gameRecord{}, err
	}
	input.ID = int(id)
	input.AuthorID = authorID
	return input, nil
}

func listPosts(db *sql.DB) ([]postRecord, error) {
	rows, err := db.Query(`SELECT p.id, p.title, p.body, p.author_name, COALESCE(u.username, ''), p.created_at FROM posts p LEFT JOIN users u ON u.id = p.user_id ORDER BY p.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := make([]postRecord, 0)
	for rows.Next() {
		var post postRecord
		var authorID string
		var created time.Time
		if err := rows.Scan(&post.ID, &post.Title, &post.Body, &post.Author, &authorID, &created); err != nil {
			return nil, err
		}
		post.AuthorID = authorID
		if post.AuthorID == "" {
			post.AuthorID = post.Author
		}
		post.AuthorAvatar = firstRune(post.Author)
		post.CreatedAt = created.Format("2006-01-02 15:04")
		post.Comments, err = listPostReplies(db, post.ID)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

func listPostReplies(db *sql.DB, postID int64) ([]replyRecord, error) {
	rows, err := db.Query(`SELECT r.id, r.content, r.author_name, COALESCE(u.username, ''), r.created_at FROM post_replies r LEFT JOIN users u ON u.id = r.user_id WHERE r.post_id = ? ORDER BY r.created_at ASC`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	replies := make([]replyRecord, 0)
	for rows.Next() {
		var reply replyRecord
		var authorID string
		var created time.Time
		if err := rows.Scan(&reply.ID, &reply.Text, &reply.Author, &authorID, &created); err != nil {
			return nil, err
		}
		reply.AuthorID = authorID
		if reply.AuthorID == "" {
			reply.AuthorID = reply.Author
		}
		reply.AuthorAvatar = firstRune(reply.Author)
		reply.CreatedAt = created.Format("2006-01-02 15:04")
		replies = append(replies, reply)
	}
	return replies, rows.Err()
}

func findPost(db *sql.DB, postID int64) (postRecord, error) {
	posts, err := listPosts(db)
	if err != nil {
		return postRecord{}, err
	}
	for _, post := range posts {
		if post.ID == postID {
			return post, nil
		}
	}
	return postRecord{}, sql.ErrNoRows
}

func createPost(db *sql.DB, title, body, author string, userID int) (postRecord, error) {
	result, err := db.Exec(`INSERT INTO posts (user_id, author_name, title, body) VALUES (?, ?, ?, ?)`, userID, author, title, body)
	if err != nil {
		return postRecord{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return postRecord{}, err
	}
	return findPost(db, id)
}

func createPostReply(db *sql.DB, postID int64, text, author string, userID int) (postRecord, error) {
	if _, err := db.Exec(`INSERT INTO post_replies (post_id, user_id, author_name, content) VALUES (?, ?, ?, ?)`, postID, userID, author, text); err != nil {
		return postRecord{}, err
	}
	return findPost(db, postID)
}

func firstRune(value string) string {
	for _, r := range value {
		return string(r)
	}
	return "访"
}

func toggleGameRelation(db *sql.DB, table string, userID, gameID int) (bool, error) {
	if table != "game_likes" && table != "game_favorites" {
		return false, fmt.Errorf("invalid relation table")
	}
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM "+table+" WHERE user_id = ? AND game_id = ?)", userID, gameID).Scan(&exists)
	if err != nil {
		return false, err
	}
	if exists {
		_, err = db.Exec("DELETE FROM "+table+" WHERE user_id = ? AND game_id = ?", userID, gameID)
		return false, err
	}
	_, err = db.Exec("INSERT INTO "+table+" (user_id, game_id) VALUES (?, ?)", userID, gameID)
	return true, err
}

func updateGameCounter(db *sql.DB, column string, gameID int, delta int) error {
	if column != "likes" && column != "favorites" {
		return fmt.Errorf("invalid counter")
	}
	_, err := db.Exec("UPDATE games SET "+column+" = GREATEST(0, "+column+" + ?) WHERE id = ?", delta, gameID)
	return err
}

func itemCounter(db *sql.DB, gameID int, column string) int {
	if column != "plays" && column != "likes" && column != "favorites" && column != "comments" {
		return 0
	}
	var value int
	if err := db.QueryRow("SELECT "+column+" FROM games WHERE id = ?", gameID).Scan(&value); err != nil {
		return 0
	}
	return value
}

func userGameRelations(db *sql.DB, table string, userID int) ([]int, error) {
	if table != "game_likes" && table != "game_favorites" {
		return nil, fmt.Errorf("invalid relation table")
	}
	rows, err := db.Query("SELECT game_id FROM "+table+" WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]int, 0)
	for rows.Next() {
		var gameID int
		if err := rows.Scan(&gameID); err != nil {
			return nil, err
		}
		items = append(items, gameID)
	}
	return items, rows.Err()
}

func gameComments(db *sql.DB, gameID int) ([]replyRecord, error) {
	rows, err := db.Query(`SELECT c.id, c.content, c.author_name, COALESCE(u.username, ''), c.created_at FROM game_comments c LEFT JOIN users u ON u.id = c.user_id WHERE c.game_id = ? ORDER BY c.created_at DESC`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := make([]replyRecord, 0)
	for rows.Next() {
		var item replyRecord
		var authorID string
		var created time.Time
		if err := rows.Scan(&item.ID, &item.Text, &item.Author, &authorID, &created); err != nil {
			return nil, err
		}
		item.AuthorID = authorID
		if item.AuthorID == "" {
			item.AuthorID = item.Author
		}
		item.AuthorAvatar = firstRune(item.Author)
		item.CreatedAt = created.Format("2006-01-02 15:04")
		comments = append(comments, item)
	}
	return comments, rows.Err()
}

func createGameComment(db *sql.DB, gameID, userID int, author, content string) (replyRecord, error) {
	result, err := db.Exec(`INSERT INTO game_comments (game_id, user_id, author_name, content) VALUES (?, ?, ?, ?)`, gameID, userID, author, content)
	if err != nil {
		return replyRecord{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return replyRecord{}, err
	}
	return replyRecord{ID: id, Text: content, Author: author, AuthorID: author, AuthorAvatar: firstRune(author), CreatedAt: time.Now().Format("2006-01-02 15:04")}, nil
}

func deleteGame(db *sql.DB, gameID int) error {
	result, err := db.Exec(`DELETE FROM games WHERE id = ?`, gameID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func deleteGameComment(db *sql.DB, gameID, commentID int64) error {
	result, err := db.Exec(`DELETE FROM game_comments WHERE id = ? AND game_id = ?`, commentID, gameID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	_, err = db.Exec(`UPDATE games SET comments = GREATEST(0, comments - 1) WHERE id = ?`, gameID)
	return err
}

func deletePost(db *sql.DB, postID int64) error {
	result, err := db.Exec(`DELETE FROM posts WHERE id = ?`, postID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func deletePostReply(db *sql.DB, postID, replyID int64) error {
	result, err := db.Exec(`DELETE FROM post_replies WHERE id = ? AND post_id = ?`, replyID, postID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
