package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const jwtSecret = "gamehub-phase-one-local-secret"

type user struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"-"`
	Role     string `json:"role"`
}

type authStore struct {
	sync.RWMutex
	nextID int
	users  map[string]user
}

func newStore() *authStore {
	return &authStore{nextID: 1, users: make(map[string]user)}
}

func (s *authStore) register(username, email, password string) (user, error) {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.users[username]; ok {
		return user{}, errors.New("用户名已存在")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return user{}, errors.New("密码处理失败")
	}
	u := user{ID: s.nextID, Username: username, Email: email, Password: string(hashed), Role: "user"}
	s.nextID++
	s.users[username] = u
	return u, nil
}

func (s *authStore) authenticate(username, password string) (user, bool) {
	s.RLock()
	defer s.RUnlock()
	u, ok := s.users[username]
	return u, ok && bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) == nil
}

func (s *authStore) find(username string) (user, bool) {
	s.RLock()
	defer s.RUnlock()
	u, ok := s.users[username]
	return u, ok
}

func tokenFor(u user, lifetime time.Duration) string {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{"sub": u.ID, "username": u.Username, "role": u.Role, "exp": time.Now().Add(lifetime).Unix()})
	enc := base64.RawURLEncoding
	input := enc.EncodeToString(header) + "." + enc.EncodeToString(payload)
	h := hmac.New(sha256.New, []byte(jwtSecret))
	_, _ = h.Write([]byte(input))
	return input + "." + enc.EncodeToString(h.Sum(nil))
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.SplitN(strings.TrimPrefix(header, "Bearer "), ".", 3)
		if header == "" || len(parts) != 3 {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "请先登录"})
			c.Abort()
			return
		}
		enc := base64.RawURLEncoding
		mac := hmac.New(sha256.New, []byte(jwtSecret))
		_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
		sig, err := enc.DecodeString(parts[2])
		if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "登录已失效，请重新登录"})
			c.Abort()
			return
		}
		var payload struct {
			Username string `json:"username"`
			Exp      int64  `json:"exp"`
		}
		data, err := enc.DecodeString(parts[1])
		if err != nil || json.Unmarshal(data, &payload) != nil || payload.Username == "" || payload.Exp < time.Now().Unix() {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "登录已失效，请重新登录"})
			c.Abort()
			return
		}
		c.Set("username", payload.Username)
		c.Next()
	}
}

func main() {
	store := newStore()
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
	r.GET("/api/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
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
		u, err := store.register(input.Username, input.Email, input.Password)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"message": err.Error()})
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
		u, ok := store.authenticate(input.Username, input.Password)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户名或密码错误"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": u, "token": tokenFor(u, 2*time.Hour), "refresh_token": tokenFor(u, 7*24*time.Hour)})
	})
	r.POST("/api/auth/refresh", func(c *gin.Context) {
		var input struct{ RefreshToken string `json:"refresh_token" binding:"required"` }
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "缺少 refresh_token"})
			return
		}
		parts := strings.SplitN(input.RefreshToken, ".", 3)
		if len(parts) != 3 {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "refresh_token 无效"})
			return
		}
		enc := base64.RawURLEncoding
		mac := hmac.New(sha256.New, []byte(jwtSecret))
		_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
		sig, err := enc.DecodeString(parts[2])
		var payload struct {
			Username string `json:"username"`
			Exp      int64  `json:"exp"`
		}
		dataErr := error(nil)
		if err == nil {
			data, decodeErr := enc.DecodeString(parts[1])
			if decodeErr != nil {
				dataErr = decodeErr
			} else {
				dataErr = json.Unmarshal(data, &payload)
			}
		}
		if err != nil || dataErr != nil || !hmac.Equal(sig, mac.Sum(nil)) || payload.Exp < time.Now().Unix() {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "refresh_token 已失效"})
			return
		}
		u, ok := store.find(payload.Username)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": tokenFor(u, 2*time.Hour)})
	})
	secured := r.Group("/api", authMiddleware())
	secured.GET("/me", func(c *gin.Context) {
		username, _ := c.Get("username")
		u, ok := store.find(username.(string))
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"message": "用户不存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": u})
	})
	_ = r.Run(":8080")
}
