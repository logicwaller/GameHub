package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const defaultJWTSecret = "gamehub-phase-one-local-secret"

type user struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"-"`
	Role     string `json:"role"`
}

type gameRecord struct {
	ID          int    `json:"id"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description" binding:"required"`
	Category    string `json:"category" binding:"required"`
	PlayTime    string `json:"playTime" binding:"required"`
	URL         string `json:"url" binding:"required,url"`
	Cover       string `json:"cover"`
	AuthorID    int    `json:"authorId"`
	Author      string `json:"author"`
	Plays       int    `json:"plays"`
	Likes       int    `json:"likes"`
	Favorites   int    `json:"favorites"`
	Comments    int    `json:"comments"`
}

func jwtSecret() string {
	if value := os.Getenv("JWT_SECRET"); value != "" {
		return value
	}
	return defaultJWTSecret
}

func tokenFor(u user, lifetime time.Duration) string {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{"sub": u.ID, "username": u.Username, "role": u.Role, "exp": time.Now().Add(lifetime).Unix()})
	enc := base64.RawURLEncoding
	input := enc.EncodeToString(header) + "." + enc.EncodeToString(payload)
	h := hmac.New(sha256.New, []byte(jwtSecret()))
	_, _ = h.Write([]byte(input))
	return input + "." + enc.EncodeToString(h.Sum(nil))
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.SplitN(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "), ".", 3)
		if len(parts) != 3 {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "请先登录"})
			c.Abort()
			return
		}
		enc := base64.RawURLEncoding
		mac := hmac.New(sha256.New, []byte(jwtSecret()))
		_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
		signature, err := enc.DecodeString(parts[2])
		var payload struct {
			Username string `json:"username"`
			Exp      int64  `json:"exp"`
		}
		data, decodeErr := enc.DecodeString(parts[1])
		parseErr := json.Unmarshal(data, &payload)
		if err != nil || decodeErr != nil || parseErr != nil || !hmac.Equal(signature, mac.Sum(nil)) || payload.Username == "" || payload.Exp < time.Now().Unix() {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "登录已失效，请重新登录"})
			c.Abort()
			return
		}
		c.Set("username", payload.Username)
		c.Next()
	}
}

func main() {
	db, err := openDatabase()
	if err != nil {
		panic("MySQL 连接失败: " + err.Error())
	}
	defer db.Close()
	if err := initSchema(db); err != nil {
		panic("初始化数据库表失败: " + err.Error())
	}

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})
	r.GET("/api/health", func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "mysql"})
	})
	r.GET("/api/games", func(c *gin.Context) {
		items, err := listGames(db, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取游戏失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	r.GET("/api/games/:id/comments", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "游戏 ID 无效"})
			return
		}
		comments, err := gameComments(db, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取评论失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": comments})
	})
	r.GET("/api/posts", func(c *gin.Context) {
		posts, err := listPosts(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取论坛失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": posts})
	})
	r.GET("/api/posts/:id", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "帖子 ID 无效"})
			return
		}
		post, err := findPost(db, id)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "帖子不存在"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取帖子失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"post": post})
	})

	r.POST("/api/auth/register", func(c *gin.Context) {
		var input struct {
			Username string `json:"username" binding:"required,min=2,max=24"`
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required,min=6"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请填写有效的用户名、邮箱和密码"})
			return
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "密码处理失败"})
			return
		}
		u, err := registerUser(db, input.Username, input.Email, string(hashed))
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"message": "用户名或邮箱已存在"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"user": u, "token": tokenFor(u, 2*time.Hour), "refresh_token": tokenFor(u, 7*24*time.Hour)})
	})
	r.POST("/api/auth/login", func(c *gin.Context) {
		var input struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请输入用户名和密码"})
			return
		}
		u, hashed, err := findUser(db, input.Username)
		if err != nil || bcrypt.CompareHashAndPassword([]byte(hashed), []byte(input.Password)) != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户名或密码错误"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": u, "token": tokenFor(u, 2*time.Hour), "refresh_token": tokenFor(u, 7*24*time.Hour)})
	})

	secured := r.Group("/api", authMiddleware())
	secured.GET("/me", func(c *gin.Context) {
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "用户不存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": u})
	})
	secured.GET("/games/analytics", func(c *gin.Context) {
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		items, err := listGames(db, &u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取数据失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	secured.POST("/games/:id/like", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "游戏 ID 无效"})
			return
		}
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		liked, err := toggleGameRelation(db, "game_likes", u.ID, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "点赞失败"})
			return
		}
		if liked {
			err = updateGameCounter(db, "likes", id, 1)
		} else {
			err = updateGameCounter(db, "likes", id, -1)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新点赞数失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"liked": liked})
	})
	secured.POST("/games/:id/favorite", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "游戏 ID 无效"})
			return
		}
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		favorite, err := toggleGameRelation(db, "game_favorites", u.ID, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "收藏失败"})
			return
		}
		if favorite {
			err = updateGameCounter(db, "favorites", id, 1)
		} else {
			err = updateGameCounter(db, "favorites", id, -1)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新收藏数失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"favorite": favorite})
	})
	secured.POST("/games/:id/comments", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "游戏 ID 无效"})
			return
		}
		var input struct {
			Text string `json:"text" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "评论内容不能为空"})
			return
		}
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		comment, err := createGameComment(db, id, u.ID, u.Username, input.Text)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "发表评论失败"})
			return
		}
		if _, err = db.Exec(`UPDATE games SET comments = comments + 1 WHERE id = ?`, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新评论数失败"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"comment": comment})
	})
	secured.POST("/games", func(c *gin.Context) {
		var input gameRecord
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请填写完整的游戏信息"})
			return
		}
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		input, err = createGame(db, input, u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "创建游戏失败"})
			return
		}
		input.Author = u.Username
		c.JSON(http.StatusCreated, gin.H{"game": input})
	})
	secured.POST("/posts", func(c *gin.Context) {
		var input struct {
			Title string `json:"title" binding:"required,max=200"`
			Body  string `json:"body" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请填写帖子标题和内容"})
			return
		}
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		post, err := createPost(db, input.Title, input.Body, u.Username, u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "发布帖子失败"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"post": post})
	})
	secured.POST("/posts/:id/replies", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "帖子 ID 无效"})
			return
		}
		var input struct {
			Text string `json:"text" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "回复内容不能为空"})
			return
		}
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		post, err := createPostReply(db, id, input.Text, u.Username, u.ID)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "帖子不存在"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "回复失败"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"post": post})
	})
	r.GET("/api/users/:username", func(c *gin.Context) {
		username := c.Param("username")
		u, _, err := findUser(db, username)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "用户不存在"})
			return
		}
		items, _ := listGames(db, &u.ID)
		c.JSON(http.StatusOK, gin.H{"user": u, "published": items})
	})
	_ = r.Run(":8080")
}
