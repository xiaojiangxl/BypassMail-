package main

import (
	"bufio"
	"context"
	"flag"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"emailer-ai/internal/config"
	"emailer-ai/internal/email"
	"emailer-ai/internal/llm"
)

func main() {
	// 1. --- 更新命令行参数 ---
	// 邮件基本信息
	subject := flag.String("subject", "", "邮件主题 (必需)")

	// 提示词相关参数 (二选一)
	prompt := flag.String("prompt", "", "自定义邮件核心内容或想法")
	promptName := flag.String("prompt-name", "", "使用 config.yaml 中预设的提示词名称")

	// 收件人相关参数 (二选一)
	recipientsStr := flag.String("recipients", "", "收件人列表, 逗号分隔")
	recipientsFile := flag.String("recipients-file", "", "从文本文件按行读取收件人列表")

	// 邮件模板增强参数
	templateName := flag.String("template", "default", "要使用的邮件模板名称 (例如: default, formal, casual)")
	title := flag.String("title", "", "邮件的动态标题 (用于模板中的 {{.Title}})")
	name := flag.String("name", "", "收件人称呼 (用于模板中的 {{.Name}})")
	url := flag.String("url", "", "附加链接 (用于模板中的 {{.URL}})")
	file := flag.String("file", "", "附加文件链接 (用于模板中的 {{.File}})")
	img := flag.String("img", "", "邮件头图链接 (用于模板中的 {{.Img}})")

	// 发件人与配置
	fromGroup := flag.String("from", "default", "指定使用的发件邮箱分组")
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")

	flag.Parse()

	// 2. --- 加载和验证配置 ---
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}
	log.Println("✅ 配置加载成功, 当前AI提供商:", cfg.ActiveProvider)

	// 验证发件邮箱组是否存在
	smtpCfg, ok := cfg.SMTPGroups[*fromGroup]
	if !ok {
		log.Fatalf("❌ 错误：找不到名为 '%s' 的发件邮箱分组。请检查 config.yaml 中的 `smtp_groups` 配置。", *fromGroup)
	}

	// 3. --- 处理收件人列表 ---
	recipients := loadRecipients(*recipientsStr, *recipientsFile)
	if len(recipients) == 0 {
		log.Fatal("❌ 错误: 必须通过 -recipients 或 -recipients-file 提供至少一个收件人。")
		flag.Usage()
		return
	}
	log.Printf("✅ 成功加载 %d 位收件人。", len(recipients))

	// 4. --- 处理邮件生成提示词 ---
	finalPrompt := *prompt
	if finalPrompt == "" {
		if *promptName == "" {
			log.Fatal("❌ 错误: 必须通过 -prompt 或 -prompt-name 提供一个邮件生成提示词。")
			flag.Usage()
			return
		}
		finalPrompt, ok = cfg.Prompts[*promptName]
		if !ok {
			log.Fatalf("❌ 错误: 在 config.yaml 中找不到名为 '%s' 的提示词。", *promptName)
		}
		log.Printf("✅ 使用预设提示词: '%s'", *promptName)
	} else {
		log.Println("✅ 使用自定义提示词。")
	}

	// 5. --- 初始化 AI 提供商并生成邮件变体 ---
	// 邮件变体数量现在等于收件人数量
	count := len(recipients)

	provider, err := llm.NewProvider(cfg)
	if err != nil {
		log.Fatalf("❌ 初始化AI提供商失败: %v", err)
	}

	log.Printf("🤖 正在调用 %s 生成 %d 份不同的邮件文案...", cfg.ActiveProvider, count)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second) // 增加超时时间
	defer cancel()

	variations, err := provider.GenerateVariations(ctx, finalPrompt, count)
	if err != nil {
		log.Fatalf("❌ AI 生成内容失败: %v", err)
	}
	if len(variations) < count {
		log.Printf("⚠️ 警告: AI 生成的文案数量 (%d) 少于收件人数量 (%d)，部分收件人将收到重复内容。", len(variations), count)
		// 如果AI返回的变体少于预期，进行填充以避免索引越界
		for i := len(variations); i < count; i++ {
			variations = append(variations, variations[i%len(variations)])
		}
	} else {
		log.Printf("✅ AI 已成功生成 %d 份不同文案。", len(variations))
	}

	// 6. --- 验证模板并准备并发发送 ---
	templatePath, ok := cfg.Templates[*templateName]
	if !ok {
		log.Fatalf("❌ 错误：找不到名为 '%s' 的模板。请检查 config.yaml 中的配置。", *templateName)
	}
	log.Printf("✅ 将使用 '%s' 邮件模板 (%s)", *templateName, templatePath)

	sender := email.NewSender(smtpCfg)
	log.Printf("🚀 准备从 '%s' 分组邮箱向 %d 位收件人并发发送邮件...", *fromGroup, len(recipients))

	var wg sync.WaitGroup
	for i, recipient := range recipients {
		wg.Add(1)
		go func(recipientIndex int, recipientAddress string) {
			defer wg.Done()

			addr := strings.TrimSpace(recipientAddress)
			// 每个收件人获取一个独立的邮件内容
			content := variations[recipientIndex]

			templateData := &email.TemplateData{
				Content: content,
				Title:   *title,
				URL:     *url,
				Name:    *name,
				File:    *file,
				Img:     *img,
				// Date 字段将在 ParseTemplate 中自动填充
			}

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

// loadRecipients 是一个辅助函数，用于从字符串或文件加载收件人列表
func loadRecipients(recipientsStr, recipientsFile string) []string {
	if recipientsFile != "" {
		file, err := os.Open(recipientsFile)
		if err != nil {
			log.Fatalf("❌ 无法打开收件人文件 '%s': %v", recipientsFile, err)
		}
		defer file.Close()

		var recipients []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			email := strings.TrimSpace(scanner.Text())
			if email != "" {
				recipients = append(recipients, email)
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatalf("❌ 读取收件人文件时出错: %v", err)
		}
		return recipients
	}

	if recipientsStr != "" {
		return strings.Split(recipientsStr, ",")
	}

	return nil
}
