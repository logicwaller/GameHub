package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client 是 Agent 的模型客户端。
type Client struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     strings.TrimSpace(apiKey),
		endpoint:   "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		httpClient: &http.Client{},
	}
}

// StreamChat 返回模型生成的文本片段。
func (c *Client) StreamChat(ctx context.Context, question string) (<-chan string, error) {
	if c == nil || c.apiKey == "" {
		return nil, fmt.Errorf("智谱 API Key 未设置")
	}
	body, err := json.Marshal(map[string]any{
		"model": "glm-4-flash",
		"messages": []map[string]string{{
			"role": "user", "content": question,
		}},
		"stream": true,
	})
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API 返回错误 (状态码 %d): %s", resp.StatusCode, string(data))
	}

	chunks := make(chan string)
	go func() {
		defer close(chunks)
		defer resp.Body.Close()
		reader := bufio.NewReader(resp.Body)
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				if readErr != io.EOF && ctx.Err() == nil {
					select {
					case chunks <- fmt.Sprintf("读取流式数据出错: %v", readErr):
					case <-ctx.Done():
					}
				}
				return
			}
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				return
			}
			var event struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(payload), &event) != nil || len(event.Choices) == 0 {
				continue
			}
			content := event.Choices[0].Delta.Content
			if content == "" {
				continue
			}
			select {
			case chunks <- content:
			case <-ctx.Done():
				return
			}
		}
	}()
	return chunks, nil
}
