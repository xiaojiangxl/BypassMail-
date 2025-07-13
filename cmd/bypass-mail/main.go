package main

import (
	"context"
	"flag"
	"log"
	"strings"
	"sync"
	"time"

	"emailer-ai/internal/config"
	"emailer-ai/internal/email"
	"emailer-ai/internal/llm"
)

func main() {
	// 1. 更新命令行参数
	subject := flag.String("subject", "", "邮件主题")
	prompt := flag.String("prompt", "", "邮件的核心内容或想法")
	count := flag.Int("count", 3, "生成邮件版本的数量")
	recipientsStr := flag.String("recipients", "", "收件人列表,逗号分隔")
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	// 新增模板选择参数，默认值为 "default"
	templateName := flag.String("template", "default", "要使用的邮件模板名称 (例如: default, formal, casual)")
	flag.Parse()

	if *subject == "" || *prompt == "" || *recipientsStr == "" {
		flag.Usage()
		return
	}

	// 2. 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}
	log.Println("✅ 配置加载成功, 当前AI提供商:", cfg.ActiveProvider)

	// 3. 查找并验证所选模板
	templatePath, ok := cfg.Templates[*templateName]
	if !ok {
		log.Fatalf("❌ 错误：找不到名为 '%s' 的模板。请检查 config.yaml 中的配置。", *templateName)
	}
	log.Printf("✅ 将使用 '%s' 邮件模板 (%s)", *templateName, templatePath)

	// ... (初始化 AI 提供商和调用 AI 生成内容的逻辑不变) ...

	// 4. 初始化 AI 提供商 (此处为了完整性保留，实际代码中这部分不变)
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		log.Fatalf("❌ 初始化AI提供商失败: %v", err)
	}
	log.Printf("🤖 正在调用 %s 生成 %d 份邮件文案...", cfg.ActiveProvider, *count)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	variations, err := provider.GenerateVariations(ctx, *prompt, *count)
	if err != nil {
		log.Fatalf("❌ AI 生成内容失败: %v", err)
	}
	log.Printf("✅ AI 已成功生成 %d 份文案。", len(variations))

	// 5. 初始化邮件发送器和收件人列表
	sender := email.NewSender(cfg.SMTP)
	recipients := strings.Split(*recipientsStr, ",")
	log.Printf("🚀 准备向 %d 位收件人并发发送邮件...", len(recipients))

	// 6. 并发发送邮件，使用选择的模板路径
	var wg sync.WaitGroup
	for i, recipient := range recipients {
		wg.Add(1)
		go func(recipientIndex int, recipientAddress string) {
			defer wg.Done()

			addr := strings.TrimSpace(recipientAddress)
			content := variations[recipientIndex%len(variations)]

			// 使用从配置中获取的模板路径
			templateData := email.TemplateData{Content: content}
			htmlBody, err := email.ParseTemplate(templatePath, templateData)
			if err != nil {
				log.Printf("❌ 为 %s 解析邮件模板失败: %v", addr, err)
				return
			}

			log.Printf("  -> 正在发送给 %s...", addr)
			if err := sender.Send(*subject, htmlBody, addr); err != nil {
				log.Printf("  ❌ 发送给 %s 失败: %v", addr, err)
			} else {
				log.Printf("  ✔️ 成功发送给 %s", addr)
			}
		}(i, recipient)
	}

	wg.Wait()
	log.Println("🎉 所有邮件已发送完毕!")
}
