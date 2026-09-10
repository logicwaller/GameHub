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
	return db, nil
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
	query := `SELECT g.id, g.title, g.description, g.primary_type, g.play_time, g.url, COALESCE(g.cover, ''), g.author_id, u.username, g.plays, g.likes, g.favorites, g.comments FROM games AS g JOIN users AS u ON u.id = g.author_id`
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
		if err := rows.Scan(&item.ID, &item.Title, &item.Description, &item.PrimaryType, &item.PlayTime, &item.URL, &item.Cover, &item.AuthorID, &item.Author, &item.Plays, &item.Likes, &item.Favorites, &item.Comments); err != nil {
			return nil, err
		}
		item.Tags, err = gameTags(db, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func searchGameIDs(db *sql.DB, query string) (map[int]bool, error) {
	rows, err := db.Query(`SELECT entity_id FROM search_documents WHERE entity_type = 'game' AND (title LIKE ? OR content LIKE ?)`, "%"+query+"%", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make(map[int]bool)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

func rebuildSearchDocuments(db *sql.DB) error {
	if _, err := db.Exec(`INSERT INTO search_documents (entity_type, entity_id, title, content) SELECT 'game', id, title, description FROM games ON DUPLICATE KEY UPDATE title=VALUES(title), content=VALUES(content)`); err != nil {
		return err
	}
	_, err := db.Exec(`INSERT INTO search_documents (entity_type, entity_id, title, content) SELECT 'post', id, title, body FROM posts ON DUPLICATE KEY UPDATE title=VALUES(title), content=VALUES(content)`)
	return err
}

func findGame(db *sql.DB, id int) (gameRecord, error) {
	var item gameRecord
	err := db.QueryRow(`SELECT g.id, g.title, g.description, g.primary_type, g.play_time, g.url, COALESCE(g.cover, ''), g.author_id, u.username, g.plays, g.likes, g.favorites, g.comments FROM games AS g JOIN users AS u ON u.id = g.author_id WHERE g.id = ?`, id).
		Scan(&item.ID, &item.Title, &item.Description, &item.PrimaryType, &item.PlayTime, &item.URL, &item.Cover, &item.AuthorID, &item.Author, &item.Plays, &item.Likes, &item.Favorites, &item.Comments)
	if err != nil {
		return item, err
	}
	item.Tags, err = gameTags(db, item.ID)
	return item, err
}

func gameTags(db *sql.DB, gameID int) ([]string, error) {
	rows, err := db.Query(`SELECT t.name FROM tags t JOIN game_tags gt ON gt.tag_id = t.id WHERE gt.game_id = ? ORDER BY t.name`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := make([]string, 0)
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
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

func backfillDailyAnalytics(db *sql.DB) error {
	const backfillName = "historical_daily_v1"

	var completed bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM analytics_backfill_state WHERE name = ?)`, backfillName).Scan(&completed); err != nil {
		return err
	}
	if completed {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO game_analytics_daily (game_id, stat_date, plays)
		SELECT game_id, DATE(played_at), COUNT(*) FROM game_play_events
		GROUP BY game_id, DATE(played_at)
		ON DUPLICATE KEY UPDATE plays = VALUES(plays)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO game_analytics_daily (game_id, stat_date, likes)
		SELECT game_id, DATE(created_at), COUNT(*) FROM game_likes
		GROUP BY game_id, DATE(created_at)
		ON DUPLICATE KEY UPDATE likes = GREATEST(likes, VALUES(likes))`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO game_analytics_daily (game_id, stat_date, favorites)
		SELECT game_id, DATE(created_at), COUNT(*) FROM game_favorites
		GROUP BY game_id, DATE(created_at)
		ON DUPLICATE KEY UPDATE favorites = GREATEST(favorites, VALUES(favorites))`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO game_analytics_daily (game_id, stat_date, comments)
		SELECT game_id, DATE(created_at), COUNT(*) FROM game_comments
		GROUP BY game_id, DATE(created_at)
		ON DUPLICATE KEY UPDATE comments = GREATEST(comments, VALUES(comments))`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO analytics_backfill_state (name) VALUES (?)`, backfillName); err != nil {
		return err
	}
	return tx.Commit()
}

func userNotifications(db *sql.DB, userID int) ([]map[string]any, error) {
	rows, err := db.Query(`SELECT id, type, content, target_type, target_id, is_read, DATE_FORMAT(created_at, '%Y-%m-%d %H:%i') FROM notifications WHERE user_id = ? ORDER BY created_at DESC LIMIT 30`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var eventType, content, targetType, createdAt string
		var targetID sql.NullInt64
		var isRead bool
		if err := rows.Scan(&id, &eventType, &content, &targetType, &targetID, &isRead, &createdAt); err != nil {
			return nil, err
		}
		item := map[string]any{"id": id, "type": eventType, "content": content, "read": isRead, "createdAt": createdAt, "targetType": targetType}
		if targetID.Valid {
			item["targetId"] = targetID.Int64
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func markNotificationRead(db *sql.DB, userID int, notificationID int64) error {
	result, err := db.Exec(`UPDATE notifications SET is_read = TRUE WHERE id = ? AND user_id = ?`, notificationID, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func markAllNotificationsRead(db *sql.DB, userID int) error {
	_, err := db.Exec(`UPDATE notifications SET is_read = TRUE WHERE user_id = ? AND is_read = FALSE`, userID)
	return err
}

func deleteAllNotifications(db *sql.DB, userID int) error {
	_, err := db.Exec(`DELETE FROM notifications WHERE user_id = ?`, userID)
	return err
}

func createGame(db *sql.DB, input gameRecord, authorID int) (gameRecord, error) {
	tx, err := db.Begin()
	if err != nil {
		return gameRecord{}, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT INTO games (title, description, primary_type, play_time, url, cover, author_id) VALUES (?, ?, ?, ?, ?, ?, ?)`, input.Title, input.Description, input.PrimaryType, input.PlayTime, input.URL, input.Cover, authorID)
	if err != nil {
		return gameRecord{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return gameRecord{}, err
	}
	input.ID = int(id)
	input.AuthorID = authorID
	for _, tag := range input.Tags {
		if _, err := tx.Exec(`INSERT INTO tags (name) VALUES (?) ON DUPLICATE KEY UPDATE name = VALUES(name)`, tag); err != nil {
			return gameRecord{}, err
		}
		var tagID int
		if err := tx.QueryRow(`SELECT id FROM tags WHERE name = ?`, tag).Scan(&tagID); err != nil {
			return gameRecord{}, err
		}
		if _, err := tx.Exec(`INSERT IGNORE INTO game_tags (game_id, tag_id) VALUES (?, ?)`, input.ID, tagID); err != nil {
			return gameRecord{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return gameRecord{}, err
	}
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

func createPostReply(db *sql.DB, postID int64, text, author string, userID int) (postRecord, int64, error) {
	result, err := db.Exec(`INSERT INTO post_replies (post_id, user_id, author_name, content) VALUES (?, ?, ?, ?)`, postID, userID, author, text)
	if err != nil {
		return postRecord{}, 0, err
	}
	replyID, err := result.LastInsertId()
	if err != nil {
		return postRecord{}, 0, err
	}
	post, err := findPost(db, postID)
	return post, replyID, err
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
	_, _ = db.Exec(`DELETE FROM search_documents WHERE entity_type = 'game' AND entity_id = ?`, gameID)
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
	_, _ = db.Exec(`DELETE FROM search_documents WHERE entity_type = 'post' AND entity_id = ?`, postID)
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
