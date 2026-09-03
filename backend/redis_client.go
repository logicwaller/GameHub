package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type redisClient struct {
	addr     string
	password string
	db       int
}

func newRedisClient() *redisClient {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	db, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	return &redisClient{addr: addr, password: os.Getenv("REDIS_PASSWORD"), db: db}
}

func (r *redisClient) command(args ...string) (any, error) {
	conn, err := net.DialTimeout("tcp", r.addr, 800*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if r.password != "" {
		if _, err := r.do(conn, "AUTH", r.password); err != nil {
			return nil, err
		}
	}
	if r.db != 0 {
		if _, err := r.do(conn, "SELECT", strconv.Itoa(r.db)); err != nil {
			return nil, err
		}
	}
	return r.do(conn, args...)
}

func (r *redisClient) do(conn net.Conn, args ...string) (any, error) {
	var request bytes.Buffer
	fmt.Fprintf(&request, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&request, "$%d\r\n%s\r\n", len(arg), arg)
	}
	if _, err := conn.Write(request.Bytes()); err != nil {
		return nil, err
	}
	return readRESP(bufio.NewReader(conn))
}

func readRESP(reader *bufio.Reader) (any, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	if line == "" {
		return nil, errors.New("empty redis response")
	}
	switch line[0] {
	case '+':
		return line[1:], nil
	case '-':
		return nil, errors.New(line[1:])
	case ':':
		return strconv.ParseInt(line[1:], 10, 64)
	case '$':
		n, err := strconv.Atoi(line[1:])
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return nil, nil
		}
		value := make([]byte, n+2)
		if _, err := io.ReadFull(reader, value); err != nil {
			return nil, err
		}
		return string(value[:n]), nil
	case '*':
		n, err := strconv.Atoi(line[1:])
		if err != nil {
			return nil, err
		}
		items := make([]any, n)
		for i := range items {
			items[i], err = readRESP(reader)
			if err != nil {
				return nil, err
			}
		}
		return items, nil
	default:
		return nil, fmt.Errorf("unknown redis response: %s", line)
	}
}

func (r *redisClient) getJSON(key string, target any) bool {
	value, err := r.command("GET", key)
	if err != nil || value == nil {
		return false
	}
	text, ok := value.(string)
	if !ok || json.Unmarshal([]byte(text), target) != nil {
		return false
	}
	return true
}

func (r *redisClient) setJSON(key string, value any, ttl time.Duration) {
	payload, err := json.Marshal(value)
	if err != nil {
		return
	}
	if ttl > 0 {
		_, _ = r.command("SET", key, string(payload), "EX", strconv.Itoa(int(ttl.Seconds())))
		return
	}
	_, _ = r.command("SET", key, string(payload))
}

func (r *redisClient) increment(key string, ttl time.Duration) (int64, error) {
	value, err := r.command("INCR", key)
	if err != nil {
		return 0, err
	}
	count, ok := value.(int64)
	if !ok {
		return 0, fmt.Errorf("invalid redis counter")
	}
	if count == 1 && ttl > 0 {
		_, _ = r.command("EXPIRE", key, strconv.Itoa(int(ttl.Seconds())))
	}
	return count, nil
}

func (r *redisClient) zadd(key string, score int, member string) {
	_, _ = r.command("ZADD", key, strconv.Itoa(score), member)
}

func (r *redisClient) del(key string) {
	_, _ = r.command("DEL", key)
}

func (r *redisClient) zrange(key string, limit int) []string {
	value, err := r.command("ZREVRANGE", key, "0", strconv.Itoa(limit-1))
	if err != nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func rateLimit(r *redisClient) func(*gin.Context) {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}
		ip := c.ClientIP()
		count, err := r.increment("rate:limit:"+ip, time.Minute)
		if err == nil && count > 120 {
			c.JSON(429, map[string]string{"message": "请求过于频繁，请稍后再试"})
			c.Abort()
			return
		}
		c.Next()
	}
}
