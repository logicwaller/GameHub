package agent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// UserResolver 从主程序的鉴权上下文中解析当前用户 ID。
type UserResolver func(*gin.Context) (int, error)

// RegisterRoutes 注册 Agent 会话和问答接口。
func RegisterRoutes(group *gin.RouterGroup, db *sql.DB, resolveUser UserResolver, client *Client) {
	group.GET("/agent/conversations", func(c *gin.Context) {
		userID, err := resolveUser(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		items, err := ListConversations(db, userID)
		if err != nil {
			log.Printf("Agent 读取会话失败(user_id=%d): %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取对话历史失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})

	group.POST("/agent/conversations", func(c *gin.Context) {
		var input struct {
			Title string `json:"title"`
		}
		if err := c.ShouldBindJSON(&input); err != nil && err != io.EOF {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请求参数无效"})
			return
		}
		userID, err := resolveUser(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		conversation, err := CreateConversation(db, userID, input.Title)
		if err != nil {
			log.Printf("Agent 创建会话失败(user_id=%d): %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "创建对话失败"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"conversation": conversation})
	})

	group.GET("/agent/conversations/:id/messages", func(c *gin.Context) {
		conversationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "对话 ID 无效"})
			return
		}
		userID, err := resolveUser(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		items, err := ListMessages(db, conversationID, userID)
		if err != nil {
			log.Printf("Agent 读取消息失败(conversation_id=%d,user_id=%d): %v", conversationID, userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取消息失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})

	group.DELETE("/agent/conversations/:id", func(c *gin.Context) {
		conversationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "对话 ID 无效"})
			return
		}
		userID, err := resolveUser(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		if err := DeleteConversation(db, conversationID, userID); err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "对话不存在"})
			return
		} else if err != nil {
			log.Printf("Agent 删除会话失败(conversation_id=%d,user_id=%d): %v", conversationID, userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "删除对话失败"})
			return
		}
		c.Status(http.StatusNoContent)
	})

	group.POST("/agent/chat", func(c *gin.Context) {
		var input struct {
			ConversationID int64  `json:"conversationId"`
			Question       string `json:"question"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "参数无效"})
			return
		}
		input.Question = strings.TrimSpace(input.Question)
		if input.Question == "" || len([]rune(input.Question)) > 4000 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "问题不能为空且不能超过 4000 个字符"})
			return
		}
		userID, err := resolveUser(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		if input.ConversationID == 0 {
			conversation, createErr := CreateConversation(db, userID, "新对话")
			if createErr != nil {
				log.Printf("Agent 自动创建会话失败(user_id=%d): %v", userID, createErr)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "创建对话失败"})
				return
			}
			input.ConversationID = conversation.ID
		} else {
			owned, ownerErr := ConversationOwned(db, input.ConversationID, userID)
			if ownerErr != nil {
				log.Printf("Agent 检查会话归属失败(conversation_id=%d,user_id=%d): %v", input.ConversationID, userID, ownerErr)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "检查对话失败"})
				return
			}
			if !owned {
				c.JSON(http.StatusNotFound, gin.H{"message": "对话不存在"})
				return
			}
		}
		if err := AppendMessage(db, input.ConversationID, userID, "user", input.Question); err != nil {
			log.Printf("Agent 保存问题失败(conversation_id=%d,user_id=%d): %v", input.ConversationID, userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "保存问题失败"})
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		chunkChan, err := client.StreamChat(c.Request.Context(), input.Question)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		var answer strings.Builder
		for chunk := range chunkChan {
			answer.WriteString(chunk)
			data, _ := json.Marshal(chunk)
			_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
		}
		if answer.Len() > 0 {
			_ = AppendMessage(db, input.ConversationID, userID, "assistant", answer.String())
			if len([]rune(input.Question)) <= 40 {
				_ = UpdateConversationTitle(db, input.ConversationID, userID, input.Question)
			}
		}
		_, _ = fmt.Fprint(c.Writer, "data: [DONE]\n\n")
		c.Writer.Flush()
	})
}
