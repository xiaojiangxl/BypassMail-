package llm

import "context"

// LLMProvider 是所有大语言模型提供商的通用接口
type LLMProvider interface {
	GenerateVariations(ctx context.Context, basePrompt string, count int) ([]string, error)
}