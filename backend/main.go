package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
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

func adminMiddleware(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, exists := c.Get("username")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "请先登录"})
			c.Abort()
			return
		}
		u, _, err := findUser(db, username.(string))
		if err != nil || u.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"message": "需要管理员权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func main() {
	// 获取env，默认使用同一目录下的.env文件
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// 启动MySQL!!!!!
	db, err := openDatabase()
	if err != nil {
		panic("MySQL 连接失败: " + err.Error())
	}
	defer db.Close()

	// 启动redis
	cache := newRedisClient()

	// 启动kafaka
	startGamePlayConsumer(db, cache)
	if games, err := listGames(db, nil); err == nil {
		for _, game := range games {
			cache.zadd("game:hot:rank", game.Plays, strconv.Itoa(game.ID))
		}
	}

	r := gin.Default()
	r.Use(rateLimit(cache))
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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
		redisStatus := "ok"
		if _, err := cache.command("PING"); err != nil {
			redisStatus = "unavailable"
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "mysql", "redis": redisStatus})
	})
	r.GET("/api/games", func(c *gin.Context) {
		query := strings.TrimSpace(c.Query("q"))
		category := strings.TrimSpace(c.Query("category"))
		sortBy := c.DefaultQuery("sort", "plays")
		cacheKey := "games:list:" + query + ":" + category + ":" + sortBy
		var cached []gameRecord
		if query == "" && category == "" && (sortBy == "plays" || sortBy == "likes") && cache.getJSON(cacheKey, &cached) {
			c.JSON(http.StatusOK, gin.H{"items": cached, "cached": true})
			return
		}
		items, err := listGames(db, nil)
		if err != nil {
			log.Printf("GET /api/games query failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取游戏失败"})
			return
		}
		if query != "" {
			filtered := items[:0]
			for _, item := range items {
				if strings.Contains(strings.ToLower(item.Title), strings.ToLower(query)) || strings.Contains(strings.ToLower(item.Description), strings.ToLower(query)) {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		if category != "" && category != "全部" {
			filtered := items[:0]
			for _, item := range items {
				if item.Category == category {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		sort.SliceStable(items, func(i, j int) bool {
			if sortBy == "likes" {
				return items[i].Likes > items[j].Likes
			}
			return items[i].Plays > items[j].Plays
		})
		if query == "" && category == "" && (sortBy == "plays" || sortBy == "likes") {
			cache.setJSON(cacheKey, items, 10*time.Minute)
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	r.GET("/api/games/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "游戏 ID 无效"})
			return
		}
		cacheKey := fmt.Sprintf("game:detail:%d", id)
		var cached gameRecord
		if cache.getJSON(cacheKey, &cached) {
			c.JSON(http.StatusOK, gin.H{"game": cached, "cached": true})
			return
		}
		item, err := findGame(db, id)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "游戏不存在"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取游戏失败"})
			return
		}
		cache.setJSON(cacheKey, item, time.Hour)
		c.JSON(http.StatusOK, gin.H{"game": item})
	})
	r.GET("/api/games/hot", func(c *gin.Context) {
		ids := cache.zrange("game:hot:rank", 20)
		items := make([]gameRecord, 0, len(ids))
		for _, value := range ids {
			id, err := strconv.Atoi(value)
			if err != nil {
				continue
			}
			item, err := findGame(db, id)
			if err == nil {
				items = append(items, item)
			}
		}
		if len(items) == 0 {
			items, _ = listGames(db, nil)
			if len(items) > 20 {
				items = items[:20]
			}
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	r.POST("/api/games/:id/play", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "游戏 ID 无效"})
			return
		}
		if _, err := findGame(db, id); err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "游戏不存在"})
			return
		}
		if err := publishGamePlay(id); err != nil {
			log.Printf("Kafka 不可用，回退为同步记录游玩量: %v", err)
			_, _ = db.Exec(`INSERT INTO game_play_events (game_id) VALUES (?)`, id)
			if _, err := db.Exec(`UPDATE games SET plays = plays + 1 WHERE id = ?`, id); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "记录游玩失败"})
				return
			}
		}
		item, _ := findGame(db, id)
		if item.Plays > 0 {
			cache.zadd("game:hot:rank", item.Plays, strconv.Itoa(id))
		}
		cache.del(fmt.Sprintf("game:detail:%d", id))
		cache.del("games:list:::plays")
		cache.del("games:list:::likes")
		c.JSON(http.StatusAccepted, gin.H{"plays": item.Plays, "queued": true})
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
		cache.del(fmt.Sprintf("game:detail:%d", id))
		cache.del("games:list:::plays")
		cache.del("games:list:::likes")
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
		cache.del(fmt.Sprintf("game:detail:%d", id))
		cache.del("games:list:::plays")
		cache.del("games:list:::likes")
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
		cache.del(fmt.Sprintf("game:detail:%d", id))
		cache.del("games:list:::plays")
		cache.del("games:list:::likes")
		c.JSON(http.StatusCreated, gin.H{"comment": comment})
	})
	admin := r.Group("/api/admin", authMiddleware(), adminMiddleware(db))
	admin.DELETE("/games/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "游戏 ID 无效"})
			return
		}
		if err := deleteGame(db, id); err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "游戏不存在"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "删除游戏失败"})
			return
		}
		cache.del(fmt.Sprintf("game:detail:%d", id))
		cache.del("games:list:::plays")
		cache.del("games:list:::likes")
		c.Status(http.StatusNoContent)
	})
	admin.DELETE("/games/:id/comments/:commentID", func(c *gin.Context) {
		gameID, err1 := strconv.ParseInt(c.Param("id"), 10, 64)
		commentID, err2 := strconv.ParseInt(c.Param("commentID"), 10, 64)
		if err1 != nil || err2 != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "评论 ID 无效"})
			return
		}
		if err := deleteGameComment(db, gameID, commentID); err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "评论不存在"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "删除评论失败"})
			return
		}
		cache.del(fmt.Sprintf("game:detail:%d", gameID))
		cache.del("games:list:::plays")
		cache.del("games:list:::likes")
		c.Status(http.StatusNoContent)
	})
	admin.DELETE("/posts/:id", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "帖子 ID 无效"})
			return
		}
		if err := deletePost(db, id); err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "帖子不存在"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "删除帖子失败"})
			return
		}
		c.Status(http.StatusNoContent)
	})
	admin.DELETE("/posts/:id/replies/:replyID", func(c *gin.Context) {
		postID, err1 := strconv.ParseInt(c.Param("id"), 10, 64)
		replyID, err2 := strconv.ParseInt(c.Param("replyID"), 10, 64)
		if err1 != nil || err2 != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "回复 ID 无效"})
			return
		}
		if err := deletePostReply(db, postID, replyID); err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "回复不存在"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "删除回复失败"})
			return
		}
		c.Status(http.StatusNoContent)
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
		cache.del("games:list:::plays")
		cache.del("games:list:::likes")
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
