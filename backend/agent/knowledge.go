package agent

import (
	"database/sql"
	"strings"
)

// KnowledgeItem 是知识库中的一条记录。
type KnowledgeItem struct {
	ID          int64
	GameName    string
	ContentType string
	Title       string
	Content     string
}

// SearchKnowledge 根据用户问题检索相关知识。
func SearchKnowledge(db *sql.DB, question string) []KnowledgeItem {
	gameName := matchGameName(db, question)
	if gameName != "" {
		return searchByGameName(db, gameName)
	}
	return searchByFullText(db, question)
}

// matchGameName 从数据库中读取所有游戏名，匹配用户问题。
func matchGameName(db *sql.DB, question string) string {
	rows, err := db.Query(`SELECT title FROM games`)
	if err != nil {
		return ""
	}
	defer rows.Close()

	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			continue
		}
		if title != "" && strings.Contains(question, title) {
			return title
		}
	}
	return ""
}

func searchByGameName(db *sql.DB, gameName string) []KnowledgeItem {
	rows, err := db.Query(
		`SELECT id, game_name, content_type, title, content
		 FROM game_knowledge
		 WHERE game_name = ?
		 ORDER BY priority DESC
		 LIMIT 5`,
		gameName,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanKnowledge(rows)
}

func searchByFullText(db *sql.DB, question string) []KnowledgeItem {
	rows, err := db.Query(
		`SELECT id, game_name, content_type, title, content
		 FROM game_knowledge
		 WHERE MATCH(title, content) AGAINST(? IN NATURAL LANGUAGE MODE)
		 LIMIT 3`,
		question,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanKnowledge(rows)
}

func scanKnowledge(rows *sql.Rows) []KnowledgeItem {
	items := make([]KnowledgeItem, 0)
	for rows.Next() {
		var item KnowledgeItem
		if err := rows.Scan(&item.ID, &item.GameName, &item.ContentType, &item.Title, &item.Content); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}
// BuildSearchQuery 把历史消息和当前问题拼成检索用的查询串
func BuildSearchQuery(history []string, question string) string {
	// 取最近3轮对话，拼成检索关键词
	// 例如：["邺山彼处是什么", "陈宇是谁"] → "邺山彼处是什么 陈宇是谁"
	if len(history) == 0 {
		return question
	}
	
	// 取最近2条历史 + 当前问题
	recent := history
	if len(recent) > 2 {
		recent = recent[len(recent)-2:]
	}
	
	return strings.Join(append(recent, question), " ")
}
// BuildSystemPrompt 根据检索到的知识构建 System Prompt。
func BuildSystemPrompt(knowledge []KnowledgeItem) string {
	prompt := `你是GameHub游戏网站的AI助手"游小助"。
你的职责是回答用户关于游戏的问题，包括游戏推荐、攻略技巧、游戏介绍等。
回答要友好、专业、有条理。
回答问题时直接回答就行，不要提及资料来源以及知识库。`

	if len(knowledge) == 0 {
		return prompt
	}

	prompt += "\n\n【以下是本站知识库中的相关资料，请优先参考】\n"
	for _, item := range knowledge {
		prompt += "\n---\n【" + item.Title + "】\n" + item.Content + "\n"
	}
	return prompt
}
// BuildSearchQuery 把历史问题和当前问题拼成检索用的查询串

