package main

import (
	"flag"
	"log"
	"strings"
)

func main() {
	// 1. 定义和解析命令行参数
	subject := flag.String("subject", "", "邮件主题 (必需)")
	prompt := flag.String("prompt", "", "邮件的核心内容或想法 (必需)")
	count := flag.Int("count", 3, "希望 AI 生成的邮件版本数量")
	recipientsStr := flag.String("recipients", "", "收件人邮箱列表，用逗号分隔 (必需)")
	configPath := flag.String("config", "config.json", "配置文件路径")
	flag.Parse()

	if *subject == "" || *prompt == "" || *recipientsStr == "" {
		log.Println("错误: 'subject', 'prompt', 和 'recipients' 是必需参数。")
		flag.Usage()
		return
	}

	// 2. 加载配置
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}
	log.Println("✅ 配置文件加载成功。")

	// 3. 调用 AI 生成邮件内容
	log.Printf("🤖 正在调用 AI 为您生成 %d 份邮件文案，请稍候...", *count)
	emailVariations, err := GenerateEmailVariations(cfg.AIConfig.APIKey, *prompt, *count)
	if err != nil {
		log.Fatalf("AI 生成邮件内容失败: %v", err)
	}

	if len(emailVariations) < *count {
		log.Printf("⚠️ AI 成功生成了 %d 份文案，少于您请求的 %d 份。", len(emailVariations), *count)
	} else {
		log.Printf("✅ AI 已成功生成 %d 份独特的邮件文案。", len(emailVariations))
	}

	if len(emailVariations) == 0 {
		log.Fatal("AI 未返回任何可用文案，程序终止。")
	}

	// 4. 解析收件人并准备发送
	recipients := strings.Split(*recipientsStr, ",")
	for i, r := range recipients {
		recipients[i] = strings.TrimSpace(r) // 清理每个地址前后的空格
	}

	sender := NewEmailSender(cfg.SMTPConfig)

	log.Printf("🚀 准备向 %d 位收件人发送邮件...", len(recipients))

	// 5. 遍历收件人并发送邮件
	for i, recipient := range recipients {
		// 使用模运算（%）来循环使用邮件文案
		// recipient 1 -> body 1, recipient 2 -> body 2, ..., recipient N -> body N, recipient N+1 -> body 1
		emailBody := emailVariations[i%len(emailVariations)]

		log.Printf("  -> 正在发送第 %d 封邮件给 %s...", i+1, recipient)

		// 发送邮件时，'To' 参数是一个只包含当前收件人的切片
		err := sender.Send(*subject, emailBody, []string{recipient})
		if err != nil {
			log.Printf("  ❌ 发送给 %s 失败: %v", recipient, err)
		} else {
			log.Printf("  ✔️ 成功发送给 %s。", recipient)
		}
	}

	log.Println("🎉 所有邮件发送任务已完成。")
}
