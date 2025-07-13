package llm

import (
	"bytes"
	"context"
	"emailer-ai/internal/config" // 确保这里的模块名与你的 go.mod 文件一致
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const deepseekAPIURL = "https://api.deepseek.com/chat/completions"

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
	generationTemplate string // 新增字段
	client             *http.Client
}

// NewDeepseekProvider 接收整个 AI 配置
func NewDeepseekProvider(cfg config.DeepseekConfig, template string) *DeepseekProvider {
	return &DeepseekProvider{
		apiKey:             cfg.APIKey,
		model:              cfg.Model,
		generationTemplate: template, // 从配置中获取模板
		client:             &http.Client{},
	}
}

// GenerateVariations 实现了 LLMProvider 接口
func (p *DeepseekProvider) GenerateVariations(ctx context.Context, basePrompt string, count int) ([]string, error) {
	// 使用从配置中传入的模板来构建 prompt
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

	req, err := http.NewRequestWithContext(ctx, "POST", deepseekAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("无法创建 HTTP 请求: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 DeepSeek API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DeepSeek API 返回错误状态 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var deepseekResp DeepseekResponse
	if err := json.NewDecoder(resp.Body).Decode(&deepseekResp); err != nil {
		return nil, fmt.Errorf("无法解码 DeepSeek API 响应: %w", err)
	}

	if len(deepseekResp.Choices) == 0 || deepseekResp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("AI 未能生成有效内容")
	}

	var emailVariations []string
	rawJSONText := []byte(deepseekResp.Choices[0].Message.Content)

	// 清理可能的 markdown 代码块
	rawJSONText = bytes.TrimPrefix(rawJSONText, []byte("```json\n"))
	rawJSONText = bytes.TrimSuffix(rawJSONText, []byte("\n```"))

	if err := json.Unmarshal(rawJSONText, &emailVariations); err != nil {
		return nil, fmt.Errorf("无法解析 AI 生成的 JSON 内容: %w\n原始文本: %s", err, string(rawJSONText))
	}

	return emailVariations, nil
}
