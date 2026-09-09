package agent

import (
	"context"
	"fmt"
	"io"

	"github.com/yankeguo/zhipu"
)

// ZhipuClient 智谱AI客户端
type ZhipuClient struct {
	client *zhipu.Client
}

// NewZhipuClient 创建客户端
func NewZhipuClient(apiKey string) *ZhipuClient {
	return &ZhipuClient{
		client: zhipu.NewClient(zhipu.WithAPIKey(apiKey)),
	}
}

// StreamChat 流式对话
func (z *ZhipuClient) StreamChat(ctx context.Context, question string) (<-chan string, error) {
	// 1. 构建请求
	req := &zhipu.ChatCompletionRequest{
		Model: "glm-4-flash",
		Messages: []zhipu.ChatCompletionMessage{
			{
				Role:    "user",
				Content: question,
			},
		},
		Stream: true,
	}

	// 2. 发起流式请求
	stream, err := z.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用智谱AI失败: %w", err)
	}

	// 3. 创建channel，用于传递回答片段
	chunkChan := make(chan string)

	go func() {
		defer close(chunkChan)
		defer stream.Close()

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				chunkChan <- fmt.Sprintf("AI出错了: %v", err)
				break
			}
			// 提取内容
			if len(resp.Choices) > 0 {
				content := resp.Choices[0].Delta.Content
				if content != "" {
					chunkChan <- content
				}
			}
		}
	}()

	return chunkChan, nil
}