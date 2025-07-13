package llm

import (
	"emailer-ai/internal/config" //
	"fmt"
)

func NewProvider(cfg *config.Config) (LLMProvider, error) {
	switch cfg.ActiveProvider {
	case "gemini":
		providerCfg := cfg.Providers.Gemini
		return NewGeminiProvider(providerCfg.APIKey, providerCfg.Model), nil
	case "doubao":
		providerCfg := cfg.Providers.Doubao
		return NewDoubaoProvider(providerCfg.APIKey, providerCfg.SecretKey), nil
	case "deepseek":
		// ... DeepSeek 的实现
		return nil, fmt.Errorf("DeepSeek provider not implemented yet")
	default:
		return nil, fmt.Errorf("未知的 AI 提供商: %s", cfg.ActiveProvider)
	}
}
