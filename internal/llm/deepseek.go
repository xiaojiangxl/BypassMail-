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

// DeepSeek API 请求体结构
type DeepseekRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	// 可根据需要添加其他参数，如 temperature, top_p 等
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DeepSeek API 响应体结构
type DeepseekResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	// 可根据需要添加 error 字段来处理 API 返回的错误信息
}

type DeepseekProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewDeepseekProvider(cfg config.DeepseekConfig) *DeepseekProvider {
	return &DeepseekProvider{
		apiKey: cfg.APIKey,
		model:  cfg.Model,
		client: &http.Client{},
	}
}

// GenerateVariations 实现了 LLMProvider 接口
func (p *DeepseekProvider) GenerateVariations(ctx context.Context, basePrompt string, count int) ([]string, error) {
	// 构建与 Gemini 类似的结构化指令，要求 AI 输出 JSON 格式
	structuredPrompt := fmt.Sprintf(
		`基于以下核心思想，为我生成 %d 份措辞不同但主题思想完全相同的专业邮件正文。
核心思想: "%s"

请严格按照以下要求操作：
1. 每一份邮件正文都必须是独立的、完整的。
2. 每一份邮件的语气、句式或侧重点应有细微差别，但核心信息和意图保持不变。
3. 不要添加任何额外的解释或文本，只返回一个格式正确的 JSON 数组，其中每个元素都是一份邮件正文的字符串。

例如: ["邮件正文1", "邮件正文2", ...]`,
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

	// 设置请求头，关键是 Authorization
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

	// AI 返回的文本本身应该是一个 JSON 字符串，需要再次解析
	var emailVariations []string
	rawJSONText := deepseekResp.Choices[0].Message.Content

	// 有时候 AI 会在 JSON 前后加上 ```json ... ```, 需要清理掉
	rawJSONText = bytes.TrimPrefix([]byte(rawJSONText), []byte("```json\n"))
	rawJSONText = bytes.TrimSuffix(rawJSONText, []byte("\n```"))

	if err := json.Unmarshal(rawJSONText, &emailVariations); err != nil {
		return nil, fmt.Errorf("无法解析 AI 生成的 JSON 内容: %w\n原始文本: %s", err, string(rawJSONText))
	}

	return emailVariations, nil
}
