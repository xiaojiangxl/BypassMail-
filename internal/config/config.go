package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// --- AI 相关配置结构体 ---
type AIConfig struct {
	ActiveProvider         string            `yaml:"active_provider"`
	Providers              ProviderConfigs   `yaml:"providers"`
	Prompts                map[string]string `yaml:"prompts"`
	StructuredInstructions map[string]string `yaml:"structured_instructions"`
	GenerationTemplate     string            `yaml:"generation_template"`
}

type ProviderConfigs struct {
	Gemini   GeminiConfig   `yaml:"gemini"`
	Doubao   DoubaoConfig   `yaml:"doubao"`
	Deepseek DeepseekConfig `yaml:"deepseek"`
}
type GeminiConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}
type DoubaoConfig struct {
	APIKey    string `yaml:"api_key"`
	SecretKey string `yaml:"secret_key"`
}
type DeepseekConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

// --- 邮件相关配置结构体 ---
type EmailConfig struct {
	SMTPAccounts map[string]SMTPConfig `yaml:"smtp_accounts"`
}

type SMTPConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	FromAlias string `yaml:"from_alias"`
}

// --- 主策略配置结构体 ---
type AppConfig struct {
	SendingStrategies map[string]SendingStrategy `yaml:"sending_strategies"`
	Templates         map[string]string          `yaml:"templates"`
}

type SendingStrategy struct {
	Policy   string   `yaml:"policy"`
	Accounts []string `yaml:"accounts"`
}

// --- 总配置加载 ---
type Config struct {
	App   *AppConfig
	AI    *AIConfig
	Email *EmailConfig
}

// loadFile 是一个辅助函数，用于读取和解析单个 YAML 文件
func loadFile(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, out)
}

// Load now loads from multiple files and aggregates them
func Load(appPath, aiPath, emailPath string) (*Config, error) {
	var appCfg AppConfig
	if err := loadFile(appPath, &appCfg); err != nil {
		return nil, err
	}

	var aiCfg AIConfig
	if err := loadFile(aiPath, &aiCfg); err != nil {
		return nil, err
	}

	var emailCfg EmailConfig
	if err := loadFile(emailPath, &emailCfg); err != nil {
		return nil, err
	}

	return &Config{
		App:   &appCfg,
		AI:    &aiCfg,
		Email: &emailCfg,
	}, nil
}
