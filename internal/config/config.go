package config

import (
	"fmt"
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
	// 新增字段
	MinDelay int `yaml:"min_delay"`
	MaxDelay int `yaml:"max_delay"`
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

// GenerateInitialConfigs 检查配置文件是否存在，如果不存在则创建
func GenerateInitialConfigs(appPath, aiPath, emailPath string) (bool, error) {
	configDir := "configs"
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return false, fmt.Errorf("无法创建配置目录 '%s': %w", configDir, err)
		}
	}

	created := false // 标记是否有文件被创建

	// 辅助函数，用于检查并创建文件
	createFile := func(path string, content []byte) error {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("🔧 检测到配置文件 '%s' 不存在，正在生成默认配置...\n", path)
			if err := os.WriteFile(path, content, 0644); err != nil {
				return fmt.Errorf("无法写入默认配置文件 '%s': %w", path, err)
			}
			created = true
		}
		return nil
	}

	// ai.yaml 的默认内容
	defaultAIContent := []byte(`# configs/ai.yaml
# 所有与 AI 模型和提示词相关的配置

active_provider: "deepseek" # 可选: gemini, doubao, deepseek

providers:
  gemini:
    api_key: "YOUR_GEMINI_API_KEY"
    model: "gemini-1.5-flash-latest"
  doubao:
    api_key: "YOUR_DOUBAO_API_KEY"
    secret_key: "YOUR_DOUBAO_SECRET_KEY"
  deepseek:
    api_key: "YOUR_DEEPSEEK_API_KEY"
    model: "deepseek-chat"

# 预设的邮件生成基础提示词
prompts:
  weekly_report: "总结本周项目的主要进展、挑战及下周计划。"
  marketing_campaign: "介绍我们的新产品特性，并提供一个限时优惠码。"

# 结构化指令，用于组合和精细化控制 AI 生成
structured_instructions:
  tone_formal: "请使用非常正式和专业的商务书面语。"
  tone_casual: "请使用轻松、友好、非正式的口吻，可以适当使用 emoji。"
  format_json_array: "严格以 JSON 数组格式返回结果，数组的每个元素都是一份邮件正文的字符串。不要添加任何额外的解释或文本。"
  add_call_to_action: "在邮件末尾，加入明确的号召性用语（Call to Action），鼓励用户点击链接或回复邮件。"

# 将 DeepSeek 的生成模板移到此处
generation_template: >-
  基于以下核心思想，为我生成 %d 份措辞不同但主题思想完全相同的专业邮件正文。
  核心思想: "%s"

  请严格按照以下要求操作：
  1. 每一份邮件正文都必须是独立的、完整的。
  2. 每一份邮件的语气、句式或侧重点应有细微差别，但核心信息和意图保持不变。
  3. 不要添加任何额外的解释或文本，只返回一个格式正确的 JSON 数组，其中每个元素都是一份邮件正文的字符串。

  例如: ["邮件正文1", "邮件正文2", ...]
`)

	// email.yaml 的默认内容
	defaultEmailContent := []byte(`# configs/email.yaml
# 负责所有 SMTP 发件账户的配置
# 注意：密码字段推荐使用应用专用密码（App Password），而不是您的主登录密码。

smtp_accounts:
  gmail_example:
    host: "smtp.gmail.com"
    port: 587
    username: "your-email@gmail.com"
    password: "YOUR_GMAIL_APP_PASSWORD" # 在此填入 Gmail 应用专用密码
    from_alias: "你的名字或团队" # 邮件中显示的发件人名称
  office365_example:
    host: "smtp.office365.com"
    port: 587
    username: "your-email@your-domain.com"
    password: "YOUR_OFFICE365_PASSWORD" # 在此填入 Office 365 账户密码
    from_alias: "你的公司"
`)

	// config.yaml 的默认内容
	defaultAppContent := []byte(`# configs/config.yaml
# 负责核心策略、模板和默认值的配置

# 邮件发送策略
sending_strategies:
  # 默认策略，使用名为 'gmail_example' 的账户，以轮询方式
  default:
    policy: "round-robin" # 策略类型: round-robin (轮询), random (随机)
    accounts:
      - "gmail_example"   # 对应 email.yaml 中定义的账户名
    min_delay: 5          # 最小发送延迟（秒）
    max_delay: 15         # 最大发送延迟（秒）
  
  # 随机使用所有账户的策略示例
  random_all:
    policy: "random"
    accounts:
      - "gmail_example"
      - "office365_example"
    min_delay: 10
    max_delay: 30

# 邮件模板配置 (路径相对于程序运行的根目录)
templates:
  default: "templates/default_template.html"
  formal: "templates/formal_template.html"
  casual: "templates/casual_template.html"
`)

	if err := createFile(aiPath, defaultAIContent); err != nil {
		return false, err
	}
	if err := createFile(emailPath, defaultEmailContent); err != nil {
		return false, err
	}
	if err := createFile(appPath, defaultAppContent); err != nil {
		return false, err
	}

	return created, nil
}
