package llm

import (
	"emailer-ai/internal/config" // 确保模块名正确
	"fmt"
)

func NewProvider(cfg *config.Config) (LLMProvider, error) {
	switch cfg.ActiveProvider {
	case "gemini":
		// ... Gemini 的实现不变
		return NewGeminiProvider(cfg.Providers.Gemini), nil
	case "doubao":
		// ... 豆包的实现不变
		return NewDoubaoProvider(cfg.Providers.Doubao), nil
	case "deepseek":
		// 更新此部分以传递正确的配置
		return NewDeepseekProvider(cfg.Providers.Deepseek), nil
	default:
		return nil, fmt.Errorf("未知的 AI 提供商: %s", cfg.ActiveProvider)
	}
}
