package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time" // 引入 time 包

	"emailer-ai/internal/config" // 确保这里的模块名与你的 go.mod 文件一致
)

const (
	deepseekAPIURL = "https://api.deepseek.com/chat/completions"
	maxRetries     = 3 // 定义最大重试次数
)

// ... DeepseekRequest, Message, DeepseekResponse 结构体保持不变 ...
type DeepseekRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DeepseekResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

type DeepseekProvider struct {
	apiKey             string
	model              string
	generationTemplate string
	client             *http.Client
}

// NewDeepseekProvider 接收整个 AI 配置
func NewDeepseekProvider(cfg config.DeepseekConfig, template string) *DeepseekProvider {
	return &DeepseekProvider{
		apiKey:             cfg.APIKey,
		model:              cfg.Model,
		generationTemplate: template,
		client:             &http.Client{},
	}
}

// GenerateVariations 实现了 LLMProvider 接口，并增加了重试逻辑
func (p *DeepseekProvider) GenerateVariations(ctx context.Context, basePrompt string, count int) ([]string, error) {
	structuredPrompt := fmt.Sprintf(
		p.generationTemplate,
		count,
		basePrompt,
	)

	reqBody := DeepseekRequest{
		Model: p.model,
		Messages: []Message{
			{Role: "user", Content: structuredPrompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("无法编码 DeepSeek 请求体: %w", err)
	}

	var lastErr error
	// --- ✨ 新增：重试循环 ---
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			// 在重试前进行短暂等待
			time.Sleep(time.Duration(attempt) * time.Second)
			fmt.Printf("... AI 内容生成失败，正在进行第 %d/%d 次重试 ...\n", attempt, maxRetries)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", deepseekAPIURL, bytes.NewBuffer(jsonData))
		if err != nil {
			lastErr = fmt.Errorf("无法创建 HTTP 请求: %w", err)
			continue // 如果创建请求失败，则进入下一次重试
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+p.apiKey)

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("第 %d 次请求 DeepSeek API 失败: %w", attempt, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("DeepSeek API 返回错误状态 %d: %s", resp.StatusCode, string(bodyBytes))
			continue
		}

		var deepseekResp DeepseekResponse
		bodyBytes, err := io.ReadAll(resp.Body) // 读取响应体以备重用
		if err != nil {
			lastErr = fmt.Errorf("无法读取 DeepSeek API 响应体: %w", err)
			continue
		}
		if err := json.Unmarshal(bodyBytes, &deepseekResp); err != nil {
			lastErr = fmt.Errorf("无法解码 DeepSeek API 响应: %w", err)
			continue
		}

		if len(deepseekResp.Choices) == 0 || deepseekResp.Choices[0].Message.Content == "" {
			lastErr = fmt.Errorf("AI 未能生成有效内容 (第 %d 次尝试)", attempt)
			continue
		}

		rawContent := deepseekResp.Choices[0].Message.Content
		if strings.HasPrefix(rawContent, "```json") {
			rawContent = strings.TrimPrefix(rawContent, "```json")
			rawContent = strings.TrimSuffix(rawContent, "```")
		}
		rawContent = strings.TrimSpace(rawContent)

		startIndex := strings.Index(rawContent, "[")
		endIndex := strings.LastIndex(rawContent, "]")

		if startIndex == -1 || endIndex == -1 || endIndex < startIndex {
			lastErr = fmt.Errorf("在 AI 响应中找不到有效的 JSON 数组 (第 %d 次尝试): %s", attempt, rawContent)
			continue // 如果找不到 JSON 数组，则重试
		}

		jsonStr := rawContent[startIndex : endIndex+1]
		var emailVariations []string
		if err := json.Unmarshal([]byte(jsonStr), &emailVariations); err != nil {
			lastErr = fmt.Errorf("无法解析 AI 生成的 JSON 内容 (第 %d 次尝试): %w\n清理后的文本: %s\n原始文本: %s", attempt, err, jsonStr, rawContent)
			continue // 如果解析失败，则重试
		}

		// 如果成功解析且数组不为空，则返回结果
		if len(emailVariations) > 0 {
			return emailVariations, nil
		}

		lastErr = fmt.Errorf("AI 生成了空的邮件列表 (第 %d 次尝试)", attempt)
		// 如果解析成功但列表为空，也进行重试
	}
	// --- 重试循环结束 ---

	return nil, fmt.Errorf("所有 %d 次尝试均告失败: %w", maxRetries, lastErr)
}
