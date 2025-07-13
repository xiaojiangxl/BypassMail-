package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const geminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash-latest:generateContent?key="

// Gemini API 请求体结构
type GeminiRequest struct {
	Contents         []Content        `json:"contents"`
	GenerationConfig GenerationConfig `json:"generationConfig"`
}

type Content struct {
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type GenerationConfig struct {
	ResponseMIMEType string `json:"response_mime_type"`
}

// Gemini API 响应体结构（简化版）
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func GenerateEmailVariations(apiKey, basePrompt string, count int) ([]string, error) {
	// 构建一个更精确的指令，要求 AI 输出 JSON 格式
	structuredPrompt := fmt.Sprintf(
		`基于以下核心思想，为我生成 %d 份措辞不同但主题思想完全相同的专业邮件正文。
核心思想: "%s"

请严格按照以下要求操作：
1. 每一份邮件正文都必须是独立的、完整的。
2. 每一份邮件的语气、句式或侧重点应有细微差别，但核心信息和意图保持不变。
3. 不要添加任何额外的解释或文本，只返回一个 JSON 数组，其中每个元素都是一份邮件正文的字符串。

例如: ["邮件正文1", "邮件正文2", ...]`,
		count,
		basePrompt,
	)

	reqBody := GeminiRequest{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: structuredPrompt},
				},
			},
		},
		GenerationConfig: GenerationConfig{
			ResponseMIMEType: "application/json",
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("无法编码请求体: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", geminiAPIURL+apiKey, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("无法创建 HTTP 请求: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Gemini API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API 返回错误状态 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("无法解码 Gemini API 响应: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("AI 未能生成有效内容")
	}

	// AI 返回的文本本身是一个 JSON 字符串，需要再次解析
	var emailVariations []string
	rawJSONText := geminiResp.Candidates[0].Content.Parts[0].Text
	if err := json.Unmarshal([]byte(rawJSONText), &emailVariations); err != nil {
		return nil, fmt.Errorf("无法解析 AI 生成的 JSON 内容: %w\n原始文本: %s", err, rawJSONText)
	}

	return emailVariations, nil
}
