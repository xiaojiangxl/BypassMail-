package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"emailer-ai/internal/config"
	"emailer-ai/internal/email"
	"emailer-ai/internal/llm"
	"emailer-ai/internal/logger" // 导入新的日志包
)

// RecipientData 用于存储从 CSV 或其他来源读取的每一行个人化数据
type RecipientData struct {
	Email string
	// 这些字段将覆盖命令行参数，为每个收件人提供定制内容
	Title        string
	URL          string
	Name         string
	File         string
	Date         string
	Img          string
	CustomPrompt string
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// --- 1. 命令行参数定义与文档 ---
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "BypassMail: AI 驱动的个性化邮件批量发送工具。\n\n")
		fmt.Fprintf(os.Stderr, "使用方法:\n  go run ./cmd/bypass-mail/ [flags]\n\n")
		fmt.Fprintf(os.Stderr, "示例 (批量发送):\n")
		fmt.Fprintf(os.Stderr, "  go run ./cmd/bypass-mail/ -subject=\"季度更新\" -recipients-file=\"path/to/list.csv\" -prompt-name=\"weekly_report\" -strategy=\"round_robin_gmail\"\n\n")
		fmt.Fprintf(os.Stderr, "示例 (单次发送):\n")
		fmt.Fprintf(os.Stderr, "  go run ./cmd/bypass-mail/ -subject=\"紧急通知\" -recipients=\"boss@example.com\" -prompt=\"服务器将重启\"\n\n")
		fmt.Fprintf(os.Stderr, "可用参数:\n")
		flag.PrintDefaults()
	}

	// 邮件核心内容
	subject := flag.String("subject", "", "邮件主题 (必需, 可被 CSV 中的 subject 列覆盖)")
	prompt := flag.String("prompt", "", "自定义邮件核心思想 (与 -prompt-name 二选一)")
	promptName := flag.String("prompt-name", "", "使用 ai.yaml 中预设的提示词名称 (与 -prompt 二选一)")
	instructionNames := flag.String("instructions", "format_json_array", "要组合的结构化指令名称, 逗号分隔 (来自 ai.yaml)")

	// 收件人信息
	recipientsStr := flag.String("recipients", "", "收件人列表, 逗号分隔 (例如: a@b.com,c@d.com)")
	recipientsFile := flag.String("recipients-file", "", "从文本或 CSV 文件读取收件人及个人化数据")

	// 邮件模板与个人化默认值
	templateName := flag.String("template", "default", "邮件模板名称 (来自 config.yaml)")
	defaultTitle := flag.String("title", "", "默认邮件内页标题 (若 CSV 未提供)")
	defaultName := flag.String("name", "", "默认收件人称呼 (若 CSV 未提供)")
	defaultURL := flag.String("url", "", "默认附加链接 (若 CSV 未提供)")
	defaultFile := flag.String("file", "", "默认附加文件链接 (若 CSV 未提供)")
	defaultImg := flag.String("img", "", "默认邮件头图链接 (若 CSV 未提供)")

	// 发送与配置路径
	strategyName := flag.String("strategy", "default", "指定使用的发件策略 (来自 config.yaml)")
	configPath := flag.String("config", "configs/config.yaml", "主策略配置文件路径")
	aiConfigPath := flag.String("ai-config", "configs/ai.yaml", "AI 配置文件路径")
	emailConfigPath := flag.String("email-config", "configs/email.yaml", "Email 配置文件路径")

	flag.Parse()

	// --- 2. 加载和验证配置 ---
	cfg, err := config.Load(*configPath, *aiConfigPath, *emailConfigPath)
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}
	log.Println("✅ 所有配置加载成功")

	// 验证发送策略
	strategy, ok := cfg.App.SendingStrategies[*strategyName]
	if !ok {
		log.Fatalf("❌ 错误：找不到名为 '%s' 的发送策略。", *strategyName)
	}
	log.Printf("✅ 使用发件策略: '%s' (策略: %s, 包含 %d 个账号)", *strategyName, strategy.Policy, len(strategy.Accounts))

	// --- 3. 加载收件人数据 ---
	recipientsData := loadRecipients(*recipientsFile, *recipientsStr)
	if len(recipientsData) == 0 {
		log.Fatal("❌ 错误: 必须提供至少一个收件人。使用 -recipients 或 -recipients-file 指定。")
	}
	log.Printf("✅ 成功加载 %d 位收件人的数据。", len(recipientsData))

	// --- 4. 为每个收件人构建最终提示词 ---
	finalPrompts := buildFinalPrompts(recipientsData, *prompt, *promptName, *instructionNames, cfg.AI)

	// --- 5. 初始化 AI 并生成邮件变体 ---
	count := len(recipientsData)
	provider, err := llm.NewProvider(cfg.AI)
	if err != nil {
		log.Fatalf("❌ 初始化AI提供商失败: %v", err)
	}

	log.Printf("🤖 正在调用 %s 为 %d 位收件人生成定制化邮件文案...", cfg.AI.ActiveProvider, count)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second) // 增加超时
	defer cancel()

	combinedPromptForGeneration := strings.Join(finalPrompts, "\n---\n")
	variations, err := provider.GenerateVariations(ctx, combinedPromptForGeneration, count)
	if err != nil {
		log.Fatalf("❌ AI 生成内容失败: %v", err)
	}
	if len(variations) < count {
		log.Printf("⚠️ 警告: AI 生成的文案数量 (%d) 少于收件人数量 (%d)，部分收件人将收到重复内容。", len(variations), count)
		// 循环使用已生成的变体来填充不足的部分
		for i := len(variations); i < count; i++ {
			variations = append(variations, variations[i%len(variations)])
		}
	} else {
		log.Printf("✅ AI 已成功生成 %d 份不同文案。", len(variations))
	}

	// --- 6. 验证模板并并发发送 ---
	templatePath, ok := cfg.App.Templates[*templateName]
	if !ok {
		log.Fatalf("❌ 错误：找不到名为 '%s' 的模板。", *templateName)
	}

	var wg sync.WaitGroup
	logChan := make(chan logger.LogEntry, len(recipientsData))

	for i, data := range recipientsData {
		wg.Add(1)
		go func(recipientIndex int, recipient RecipientData) {
			defer wg.Done()

			logEntry := logger.LogEntry{
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
				Recipient: recipient.Email,
			}

			// --- 策略化选择发件人 ---
			accountName := selectAccount(strategy, recipientIndex)
			smtpCfg, ok := cfg.Email.SMTPAccounts[accountName]
			if !ok {
				errMsg := fmt.Sprintf("在策略 '%s' 中定义的账户 '%s' 找不到配置。", *strategyName, accountName)
				log.Printf("❌ 错误: %s", errMsg)
				logEntry.Status = "Failed"
				logEntry.Error = errMsg
				logChan <- logEntry
				return
			}
			sender := email.NewSender(smtpCfg)
			logEntry.Sender = smtpCfg.Username

			addr := strings.TrimSpace(recipient.Email)
			content := variations[recipientIndex]

			// --- 填充个人化模板数据 ---
			templateData := &email.TemplateData{
				Content: content,
				Title:   coalesce(recipient.Title, *defaultTitle, *subject),
				Name:    coalesce(recipient.Name, *defaultName),
				URL:     coalesce(recipient.URL, *defaultURL),
				File:    coalesce(recipient.File, *defaultFile),
				Img:     coalesce(recipient.Img, *defaultImg),
				Date:    recipient.Date,
			}
			finalSubject := coalesce(recipient.Title, *subject)
			logEntry.Subject = finalSubject

			htmlBody, err := email.ParseTemplate(templatePath, templateData)
			if err != nil {
				log.Printf("❌ 为 %s 解析邮件模板失败: %v", addr, err)
				logEntry.Status = "Failed"
				logEntry.Error = fmt.Sprintf("解析模板失败: %v", err)
				logChan <- logEntry
				return
			}
			logEntry.Content = htmlBody

			log.Printf("  -> [使用 %s] 正在发送给 %s...", smtpCfg.Username, addr)
			if err := sender.Send(finalSubject, htmlBody, addr); err != nil {
				log.Printf("  ❌ 发送给 %s 失败: %v", addr, err)
				logEntry.Status = "Failed"
				logEntry.Error = err.Error()
			} else {
				log.Printf("  ✔️ 成功发送给 %s", addr)
				logEntry.Status = "Success"
			}
			logChan <- logEntry
		}(i, data)
	}

	wg.Wait()
	close(logChan) // 所有协程完成，关闭通道

	// --- 7. 收集日志并生成报告 ---
	var logEntries []logger.LogEntry
	for entry := range logChan {
		logEntries = append(logEntries, entry)
	}

	if len(logEntries) > 0 {
		if _, err := logger.GenerateHTMLReport(logEntries); err != nil {
			log.Printf("❌ 生成 HTML 报告失败: %v", err)
		}
	}

	log.Println("🎉 所有邮件任务已处理完毕!")
}

// loadRecipients 优先处理 CSV，然后是 TXT，最后是命令行字符串
func loadRecipients(filePath, recipientsStr string) []RecipientData {
	if filePath != "" {
		if strings.HasSuffix(strings.ToLower(filePath), ".csv") {
			return loadRecipientsFromCSV(filePath)
		}
		// 默认处理为纯文本文件
		return loadRecipientsFromTxt(filePath)
	}
	if recipientsStr != "" {
		var data []RecipientData
		for _, email := range strings.Split(recipientsStr, ",") {
			if em := strings.TrimSpace(email); em != "" {
				data = append(data, RecipientData{Email: em})
			}
		}
		return data
	}
	return nil
}

func loadRecipientsFromTxt(filePath string) []RecipientData {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("⚠️ 警告: 无法打开文本文件 '%s', 将跳过此文件: %v", filePath, err)
		return nil
	}
	defer file.Close()

	var data []RecipientData
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		email := strings.TrimSpace(scanner.Text())
		if email != "" {
			data = append(data, RecipientData{Email: email})
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("⚠️ 警告: 读取文件 '%s' 时发生错误: %v", filePath, err)
	}
	return data
}

func loadRecipientsFromCSV(filePath string) []RecipientData {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("❌ 无法打开 CSV 文件 '%s': %v", filePath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("❌ 解析 CSV 文件失败: %v", err)
	}

	if len(records) < 2 {
		log.Fatal("❌ CSV 文件至少需要一个标题行和一行数据。")
	}

	header := records[0]
	headerMap := make(map[string]int)
	for i, h := range header {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	if _, ok := headerMap["email"]; !ok {
		log.Fatal("❌ CSV 文件必须包含一个名为 'email' 的列。")
	}

	var data []RecipientData
	for i, row := range records[1:] {
		recipient := RecipientData{}
		if idx, ok := headerMap["email"]; ok {
			recipient.Email = row[idx]
		}
		if recipient.Email == "" {
			log.Printf("⚠️ 警告: CSV 第 %d 行缺少 email，已跳过。", i+2)
			continue
		}
		if idx, ok := headerMap["title"]; ok {
			recipient.Title = row[idx]
		}
		if idx, ok := headerMap["name"]; ok {
			recipient.Name = row[idx]
		}
		if idx, ok := headerMap["url"]; ok {
			recipient.URL = row[idx]
		}
		if idx, ok := headerMap["file"]; ok {
			recipient.File = row[idx]
		}
		if idx, ok := headerMap["date"]; ok {
			recipient.Date = row[idx]
		}
		if idx, ok := headerMap["img"]; ok {
			recipient.Img = row[idx]
		}
		if idx, ok := headerMap["customprompt"]; ok {
			recipient.CustomPrompt = row[idx]
		}
		data = append(data, recipient)
	}
	return data
}

// buildFinalPrompts 为每个收件人构建最终的提示词
func buildFinalPrompts(recipients []RecipientData, basePrompt, promptName, instructionsStr string, aiCfg *config.AIConfig) []string {
	var finalPrompts []string

	finalBasePrompt := basePrompt
	if finalBasePrompt == "" && promptName != "" {
		if p, ok := aiCfg.Prompts[promptName]; ok {
			finalBasePrompt = p
		} else {
			log.Fatalf("❌ 找不到名为 '%s' 的预设提示词。", promptName)
		}
	}
	if finalBasePrompt == "" {
		log.Fatal("❌ 必须通过 -prompt 或 -prompt-name 提供一个基础 prompt。")
	}

	var instructionBuilder strings.Builder
	if instructionsStr != "" {
		names := strings.Split(instructionsStr, ",")
		for _, name := range names {
			trimmedName := strings.TrimSpace(name)
			if instr, ok := aiCfg.StructuredInstructions[trimmedName]; ok {
				instructionBuilder.WriteString(instr)
				instructionBuilder.WriteString("\n")
			} else {
				log.Printf("⚠️ 警告: 找不到名为 '%s' 的结构化指令。", trimmedName)
			}
		}
	}

	for _, r := range recipients {
		var prompt strings.Builder
		prompt.WriteString(instructionBuilder.String())
		// 优先使用收件人特定的 custom prompt
		if r.CustomPrompt != "" {
			prompt.WriteString("核心思想: \"" + r.CustomPrompt + "\"\n")
		} else {
			prompt.WriteString("核心思想: \"" + finalBasePrompt + "\"\n")
		}
		finalPrompts = append(finalPrompts, prompt.String())
	}
	return finalPrompts
}

// selectAccount 根据策略选择一个发件箱账户名
func selectAccount(strategy config.SendingStrategy, index int) string {
	numAccounts := len(strategy.Accounts)
	if numAccounts == 0 {
		log.Fatal("❌ 策略中没有配置任何发件账户。")
	}

	switch strategy.Policy {
	case "round-robin":
		return strategy.Accounts[index%numAccounts]
	case "random":
		return strategy.Accounts[rand.Intn(numAccounts)]
	default:
		// 默认或未知策略，使用第一个
		return strategy.Accounts[0]
	}
}

// coalesce 返回第一个非空的字符串
func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
