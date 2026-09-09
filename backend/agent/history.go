package agent

import (
	"database/sql"
	"strings"
)

// Conversation 是 Agent 会话摘要。
type Conversation struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
}

// Message 是 Agent 会话中的一条消息。
type Message struct {
	ID        int64  `json:"id"`
	Role      string `json:"role"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}

func ListConversations(db *sql.DB, userID int) ([]Conversation, error) {
	rows, err := db.Query(`SELECT id, title, DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i') FROM agent_conversations WHERE user_id = ? ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Conversation, 0)
	for rows.Next() {
		var item Conversation
		if err := rows.Scan(&item.ID, &item.Title, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func CreateConversation(db *sql.DB, userID int, title string) (Conversation, error) {
	if strings.TrimSpace(title) == "" {
		title = "新对话"
	}
	result, err := db.Exec(`INSERT INTO agent_conversations (user_id, title) VALUES (?, ?)`, userID, title)
	if err != nil {
		return Conversation{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Conversation{}, err
	}
	var item Conversation
	err = db.QueryRow(`SELECT id, title, DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i') FROM agent_conversations WHERE id = ? AND user_id = ?`, id, userID).Scan(&item.ID, &item.Title, &item.UpdatedAt)
	return item, err
}

func ConversationOwned(db *sql.DB, conversationID int64, userID int) (bool, error) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM agent_conversations WHERE id = ? AND user_id = ?)`, conversationID, userID).Scan(&exists)
	return exists, err
}

func ListMessages(db *sql.DB, conversationID int64, userID int) ([]Message, error) {
	rows, err := db.Query(`SELECT m.id, m.role, m.content, DATE_FORMAT(m.created_at, '%Y-%m-%d %H:%i') FROM agent_messages m JOIN agent_conversations c ON c.id = m.conversation_id WHERE m.conversation_id = ? AND c.user_id = ? ORDER BY m.created_at ASC, m.id ASC`, conversationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Message, 0)
	for rows.Next() {
		var item Message
		if err := rows.Scan(&item.ID, &item.Role, &item.Text, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func AppendMessage(db *sql.DB, conversationID int64, userID int, role, content string) error {
	result, err := db.Exec(`INSERT INTO agent_messages (conversation_id, role, content) SELECT ?, ?, ? FROM agent_conversations WHERE id = ? AND user_id = ?`, conversationID, role, content, conversationID, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	_, err = db.Exec(`UPDATE agent_conversations SET updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?`, conversationID, userID)
	return err
}

func UpdateConversationTitle(db *sql.DB, conversationID int64, userID int, title string) error {
	_, err := db.Exec(`UPDATE agent_conversations SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ? AND title = '新对话'`, title, conversationID, userID)
	return err
}

func DeleteConversation(db *sql.DB, conversationID int64, userID int) error {
	result, err := db.Exec(`DELETE FROM agent_conversations WHERE id = ? AND user_id = ?`, conversationID, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
