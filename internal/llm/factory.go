package llm

import (
	"emailer-ai/internal/config"
	"fmt"
)

// NewProvider 现在接收 AIConfig
func NewProvider(cfg *config.AIConfig) (LLMProvider, error) {
	switch cfg.ActiveProvider {
	case "gemini":
		// return NewGeminiProvider(cfg.Providers.Gemini), nil // 需要适配
		return nil, fmt.Errorf("Gemini provider not fully updated yet")
	case "doubao":
		// return NewDoubaoProvider(cfg.Providers.Doubao), nil // 需要适配
		return nil, fmt.Errorf("豆包模型的功能尚未实现")
	case "deepseek":
		// 传递 Deepseek 特定配置和通用的生成模板
		return NewDeepseekProvider(cfg.Providers.Deepseek, cfg.GenerationTemplate), nil
	default:
		return nil, fmt.Errorf("未知的 AI 提供商: %s", cfg.ActiveProvider)
	}
}
