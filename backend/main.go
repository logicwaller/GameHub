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

	"gamehub/backend/agent"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

const defaultJWTSecret = "gamehub-phase-one-local-secret"

var agentClient *agent.Client

type user struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"-"`
	Role     string `json:"role"`
}

type gameRecord struct {
	ID          int      `json:"id"`
	Title       string   `json:"title" binding:"required"`
	Description string   `json:"description" binding:"required"`
	PrimaryType string   `json:"primaryType" binding:"required"`
	Tags        []string `json:"tags"`
	PlayTime    string   `json:"playTime" binding:"required"`
	URL         string   `json:"url" binding:"required,url"`
	Cover       string   `json:"cover"`
	AuthorID    int      `json:"authorId"`
	Author      string   `json:"author"`
	Plays       int      `json:"plays"`
	Likes       int      `json:"likes"`
	Favorites   int      `json:"favorites"`
	Comments    int      `json:"comments"`
}

var primaryGameTypes = map[string]bool{
	"ARG/WIG": true,
	"现实互动解谜":  true,
	"网页互动游戏":  true,
	"网页解谜":    true,
	"互动叙事":    true,
}

func normalizeGameTags(tags []string) ([]string, error) {
	if len(tags) > 12 {
		return nil, fmt.Errorf("标签最多 12 个")
	}
	items := make([]string, 0, len(tags))
	seen := make(map[string]bool)
	for _, tag := range tags {
		name := strings.TrimSpace(tag)
		if name == "" {
			continue
		}
		if len([]rune(name)) > 24 {
			return nil, fmt.Errorf("单个标签不能超过 24 个字符")
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, name)
	}
	return items, nil
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

func authMiddleware(cache *redisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		parts := strings.SplitN(rawToken, ".", 3)
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
		tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(rawToken)))
		if revoked, err := cache.command("EXISTS", "auth:blacklist:"+tokenHash); err == nil && revoked == int64(1) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "登录已退出，请重新登录"})
			c.Abort()
			return
		}
		c.Set("username", payload.Username)
		c.Set("token", rawToken)
		c.Set("token_exp", payload.Exp)
		c.Next()
	}
}

func authRateLimit(cache *redisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		if count, err := cache.increment("auth:limit:"+c.ClientIP(), time.Minute); err == nil && count > 10 {
			c.JSON(http.StatusTooManyRequests, gin.H{"message": "登录或注册请求过于频繁，请稍后再试"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func idempotency(cache *redisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if key == "" {
			c.Next()
			return
		}
		if len(key) > 120 || !cache.setWithTTL("idempotency:"+c.ClientIP()+":"+key, "1", 2*time.Minute) {
			c.JSON(http.StatusConflict, gin.H{"message": "请勿重复提交请求"})
			c.Abort()
			return
		}
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

func emitEvent(envKey, fallbackTopic, key string, payload any) {
	topic := os.Getenv(envKey)
	if topic == "" {
		topic = fallbackTopic
	}
	eventType := fallbackTopic
	if value, ok := payload.(map[string]any); ok {
		if kind, exists := value["type"].(string); exists && kind != "" {
			eventType = kind
		}
	}
	go func() {
		if err := publishEvent(topic, key, eventType, payload); err != nil {
			log.Printf("Kafka 事件发送失败(%s): %v", topic, err)
		}
	}()
}

func cachedRelations(db *sql.DB, cache *redisClient, userID int, table, keyPrefix string) ([]int, error) {
	key := keyPrefix + strconv.Itoa(userID)
	values, err := cache.setMembers(key)
	if err == nil && len(values) > 0 {
		items := make([]int, 0, len(values))
		for _, value := range values {
			if id, err := strconv.Atoi(value); err == nil {
				items = append(items, id)
			}
		}
		return items, nil
	}
	items, err := userGameRelations(db, table, userID)
	if err != nil {
		return nil, err
	}
	for _, id := range items {
		cache.setAdd(key, strconv.Itoa(id))
	}
	return items, nil
}

// ============================================================
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
	if err := rebuildSearchDocuments(db); err != nil {
		log.Printf("初始化搜索文档失败: %v", err)
	}
	if err := backfillDailyAnalytics(db); err != nil {
		log.Printf("历史日统计回填失败: %v", err)
	}

	// 启动redis
	cache := newRedisClient()

	//api
	apiKey := os.Getenv("ZHIPU_API_KEY")
	if apiKey == "" {
		log.Fatal("请设置 ZHIPU_API_KEY 环境变量")
	}
	agentClient = agent.NewClient(apiKey)

	// 启动kafaka
	if err := ensureKafkaTopics(); err != nil {
		log.Printf("Kafka 主题初始化失败: %v", err)
	}
	startGamePlayConsumer(db, cache)
	startPhaseThreeConsumers(db)
	if games, err := listGames(db, nil); err == nil {
		for _, game := range games {
			cache.zadd("game:hot:rank", game.Plays, strconv.Itoa(game.ID))
			cache.zadd("game:fav:rank", game.Favorites, strconv.Itoa(game.ID))
			cache.hset(fmt.Sprintf("game:stats:%d", game.ID), "plays", game.Plays)
			cache.hset(fmt.Sprintf("game:stats:%d", game.ID), "likes", game.Likes)
			cache.hset(fmt.Sprintf("game:stats:%d", game.ID), "favorites", game.Favorites)
			cache.hset(fmt.Sprintf("game:stats:%d", game.ID), "comments", game.Comments)
		}
	}

	r := gin.Default()
	r.Use(rateLimit(cache))
	r.Use(idempotency(cache))
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
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
		primaryType := strings.TrimSpace(c.Query("primary_type"))
		sortBy := c.DefaultQuery("sort", "plays")
		cacheKey := "games:list:" + query + ":" + primaryType + ":" + sortBy
		var cached []gameRecord
		if query == "" && primaryType == "" && (sortBy == "plays" || sortBy == "likes") && cache.getJSON(cacheKey, &cached) {
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
			matchedIDs, err := searchGameIDs(db, query)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "搜索索引读取失败"})
				return
			}
			filtered := items[:0]
			for _, item := range items {
				if matchedIDs[item.ID] {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		if primaryType != "" && primaryType != "全部" {
			filtered := items[:0]
			for _, item := range items {
				if item.PrimaryType == primaryType {
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
		if query == "" && primaryType == "" && (sortBy == "plays" || sortBy == "likes") {
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
	r.GET("/api/games/favorites/rank", func(c *gin.Context) {
		ids := cache.zrange("game:fav:rank", 20)
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
			sort.Slice(items, func(i, j int) bool {
				return items[i].Favorites > items[j].Favorites
			})
			if len(items) > 20 {
				items = items[:20]
			}
			for _, item := range items {
				cache.zadd("game:fav:rank", item.Favorites, strconv.Itoa(item.ID))
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
		queued := true
		if err := publishGamePlay(id); err != nil {
			log.Printf("Kafka 不可用，回退为同步记录游玩量: %v", err)
			queued = false
			if err := recordGamePlayFallback(db, id); err != nil {
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
		cache.hset(fmt.Sprintf("game:stats:%d", id), "plays", item.Plays)
		status := http.StatusAccepted
		if !queued {
			status = http.StatusOK
		}
		c.JSON(status, gin.H{"plays": item.Plays, "queued": queued})
	})
	r.GET("/api/games/:id/comments", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "游戏 ID 无效"})
			return
		}
		cacheKey := fmt.Sprintf("game:comments:%d", id)
		var comments []replyRecord
		if !cache.getJSON(cacheKey, &comments) {
			comments, err = gameComments(db, id)
			if err == nil {
				cache.setJSON(cacheKey, comments, 2*time.Minute)
			}
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取评论失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": comments})
	})
	r.GET("/api/posts", func(c *gin.Context) {
		var posts []postRecord
		if !cache.getJSON("forum:latest", &posts) {
			var err error
			posts, err = listPosts(db)
			if err == nil {
				cache.setJSON("forum:latest", posts, 2*time.Minute)
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "读取论坛失败"})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"items": posts})
	})
	r.GET("/api/posts/:id", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "帖子 ID 无效"})
			return
		}
		cacheKey := fmt.Sprintf("post:detail:%d", id)
		var post postRecord
		if !cache.getJSON(cacheKey, &post) {
			post, err = findPost(db, id)
			if err == nil {
				cache.setJSON(cacheKey, post, 2*time.Minute)
			}
		}
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

	r.POST("/api/auth/register", authRateLimit(cache), func(c *gin.Context) {
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
		emitEvent("KAFKA_NOTIFICATION_TOPIC", "notification", strconv.Itoa(u.ID), map[string]any{"type": "welcome", "user_id": u.ID, "content": "欢迎加入 GameHub，开始探索你的下一场冒险。"})
		c.JSON(http.StatusCreated, gin.H{"user": u, "token": tokenFor(u, 2*time.Hour), "refresh_token": tokenFor(u, 7*24*time.Hour)})
	})
	r.POST("/api/auth/login", authRateLimit(cache), func(c *gin.Context) {
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

	secured := r.Group("/api", authMiddleware(cache))
	agent.RegisterRoutes(secured, db, func(c *gin.Context) (int, error) {
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		return u.ID, err
	}, agentClient)
	secured.GET("/me", func(c *gin.Context) {
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "用户不存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": u})
	})
	secured.GET("/me/relations", func(c *gin.Context) {
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		likes, err := cachedRelations(db, cache, u.ID, "game_likes", "user:like:")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取点赞状态失败"})
			return
		}
		favorites, err := cachedRelations(db, cache, u.ID, "game_favorites", "user:fav:")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取收藏状态失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"liked": likes, "favorites": favorites})
	})
	secured.POST("/auth/logout", func(c *gin.Context) {
		rawToken, _ := c.Get("token")
		expiresAt, _ := c.Get("token_exp")
		ttl := time.Until(time.Unix(expiresAt.(int64), 0))
		if ttl > 0 {
			tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(rawToken.(string))))
			_, _ = cache.command("SET", "auth:blacklist:"+tokenHash, "1", "EX", strconv.Itoa(int(ttl.Seconds())))
		}
		c.Status(http.StatusNoContent)
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
		days := 7
		if value, err := strconv.Atoi(c.DefaultQuery("days", "7")); err == nil && value >= 1 && value <= 90 {
			days = value
		}
		var gameID *int
		if value := c.Query("game_id"); value != "" {
			id, err := strconv.Atoi(value)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": "游戏 ID 无效"})
				return
			}
			gameID = &id
		}
		trend, err := gameDailyAnalytics(db, u.ID, gameID, days)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取趋势数据失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items, "trend": trend})
	})
	secured.GET("/notifications", func(c *gin.Context) {
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		items, err := userNotifications(db, u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取通知失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	secured.POST("/notifications/:id/read", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "通知 ID 无效"})
			return
		}
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		if err := markNotificationRead(db, u.ID, id); err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "通知不存在"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新通知失败"})
			return
		}
		c.Status(http.StatusNoContent)
	})
	secured.POST("/notifications/read-all", func(c *gin.Context) {
		username, _ := c.Get("username")
		u, _, err := findUser(db, username.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		if err := markAllNotificationsRead(db, u.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新通知失败"})
			return
		}
		c.Status(http.StatusNoContent)
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
		cache.hset(fmt.Sprintf("game:stats:%d", id), "likes", itemCounter(db, id, "likes"))
		if liked {
			cache.setAdd("user:like:"+strconv.Itoa(u.ID), strconv.Itoa(id))
		} else {
			cache.setRemove("user:like:"+strconv.Itoa(u.ID), strconv.Itoa(id))
		}
		emitEvent("KAFKA_INTERACTION_TOPIC", "interaction.event", strconv.Itoa(id), map[string]any{"type": "like", "game_id": id, "user_id": u.ID, "liked": liked})
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
		favorites := itemCounter(db, id, "favorites")
		cache.hset(fmt.Sprintf("game:stats:%d", id), "favorites", favorites)
		cache.zadd("game:fav:rank", favorites, strconv.Itoa(id))
		if favorite {
			cache.setAdd("user:fav:"+strconv.Itoa(u.ID), strconv.Itoa(id))
		} else {
			cache.setRemove("user:fav:"+strconv.Itoa(u.ID), strconv.Itoa(id))
		}
		emitEvent("KAFKA_INTERACTION_TOPIC", "interaction.event", strconv.Itoa(id), map[string]any{"type": "favorite", "game_id": id, "user_id": u.ID, "favorite": favorite})
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
		cache.del(fmt.Sprintf("game:comments:%d", id))
		cache.del("games:list:::plays")
		cache.del("games:list:::likes")
		cache.hset(fmt.Sprintf("game:stats:%d", id), "comments", itemCounter(db, id, "comments"))
		emitEvent("KAFKA_INTERACTION_TOPIC", "interaction.event", strconv.Itoa(id), map[string]any{"type": "comment", "game_id": id, "user_id": u.ID})
		if game, err := findGame(db, id); err == nil && game.AuthorID != u.ID {
			emitEvent("KAFKA_NOTIFICATION_TOPIC", "notification", strconv.Itoa(game.AuthorID), map[string]any{"type": "game_comment", "user_id": game.AuthorID, "content": u.Username + " 评论了你的游戏《" + game.Title + "》。", "target_type": "game", "target_id": id})
		}
		c.JSON(http.StatusCreated, gin.H{"comment": comment})
	})
	admin := r.Group("/api/admin", authMiddleware(cache), adminMiddleware(db))
	admin.GET("/kafka/dlq/:topic", func(c *gin.Context) {
		items, err := readDeadLetters(c.Param("topic"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "读取死信消息失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	admin.POST("/kafka/dlq/:topic/:eventID/replay", func(c *gin.Context) {
		err := replayDeadLetter(c.Param("topic"), c.Param("eventID"))
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "死信消息不存在"})
			return
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "重放死信消息失败"})
			return
		}
		c.Status(http.StatusNoContent)
	})
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
		cache.del(fmt.Sprintf("game:comments:%d", gameID))
		cache.del("games:list:::plays")
		cache.del("games:list:::likes")
		cache.hset(fmt.Sprintf("game:stats:%d", gameID), "comments", itemCounter(db, int(gameID), "comments"))
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
		cache.del("forum:latest")
		cache.del(fmt.Sprintf("post:detail:%d", id))
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
		cache.del("forum:latest")
		cache.del(fmt.Sprintf("post:detail:%d", postID))
		c.Status(http.StatusNoContent)
	})
	secured.POST("/games", func(c *gin.Context) {
		var input gameRecord
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请填写完整的游戏信息"})
			return
		}
		input.PrimaryType = strings.TrimSpace(input.PrimaryType)
		if !primaryGameTypes[input.PrimaryType] {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请选择有效的主类型"})
			return
		}
		tags, err := normalizeGameTags(input.Tags)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		input.Tags = tags
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
		emitEvent("KAFKA_SEARCH_TOPIC", "search.sync", strconv.Itoa(input.ID), map[string]any{"type": "game.created", "game_id": input.ID})
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
		cache.del("forum:latest")
		emitEvent("KAFKA_SEARCH_TOPIC", "search.sync", strconv.FormatInt(post.ID, 10), map[string]any{"type": "post.created", "post_id": post.ID})
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
		post, _, err := createPostReply(db, id, input.Text, u.Username, u.ID)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "帖子不存在"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "回复失败"})
			return
		}
		cache.del("forum:latest")
		cache.del(fmt.Sprintf("post:detail:%d", id))
		if ownerID, err := postAuthorID(db, id); err == nil && ownerID != 0 && ownerID != u.ID {
			emitEvent("KAFKA_NOTIFICATION_TOPIC", "notification", strconv.Itoa(ownerID), map[string]any{"type": "post_reply", "user_id": ownerID, "content": u.Username + " 回复了你的帖子。", "target_type": "post", "target_id": id})
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
