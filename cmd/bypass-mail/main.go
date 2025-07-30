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
	"emailer-ai/internal/logger"
)

var (
	version = "dev" // 默认值为 'dev'，可以在编译时使用 ldflags 覆盖
)

const (
	// 定义批处理大小
	batchSize = 50
	// 定义报告分块大小
	reportChunkSize = 1000
)

// RecipientData 用于存储从 CSV 或其他来源读取的每一行个性化数据
type RecipientData struct {
	Email        string
	Title        string
	URL          string
	Name         string
	File         string
	Date         string
	Img          string
	CustomPrompt string
}

// testAccounts 函数用于测试发件人账户的连通性
func testAccounts(cfg *config.Config, strategyName string) {
	strategy, ok := cfg.App.SendingStrategies[strategyName]
	if !ok {
		log.Fatalf("❌ 错误：找不到发送策略 '%s'。", strategyName)
	}

	log.Printf("🧪 开始测试策略 '%s' 中的 %d 个发件人账户...", strategyName, len(strategy.Accounts))
	var wg sync.WaitGroup
	results := make(chan string, len(strategy.Accounts))

	for _, accountName := range strategy.Accounts {
		wg.Add(1)
		go func(accName string) {
			defer wg.Done()
			smtpCfg, ok := cfg.Email.SMTPAccounts[accName]
			if !ok {
				results <- fmt.Sprintf("  - [ %-20s ] ❌ 未找到配置", accName)
				return
			}
			sender := email.NewSender(smtpCfg)
			// 在测试模式下，我们传递一个空的收件人地址。
			// sender.Send 函数将处理此问题，并且仅执行连接和身份验证测试。
			if err := sender.Send("", "", "", ""); err != nil {
				results <- fmt.Sprintf("  - [ %-20s ] ❌ 失败: %v", smtpCfg.Username, err)
			} else {
				results <- fmt.Sprintf("  - [ %-20s ] ✔️ 成功", smtpCfg.Username)
			}
		}(accountName)
	}

	wg.Wait()
	close(results)

	for res := range results {
		log.Println(res)
	}
	log.Println("✅ 账户测试完成。")
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// --- 1. 命令行参数定义和文档 ---
	showVersion := flag.Bool("version", false, "显示工具版本并退出")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "BypassMail: AI 驱动的个性化批量邮件发送工具。\n\n")
		fmt.Fprintf(os.Stderr, "用法:\n  bypass-mail [flags]\n\n")
		fmt.Fprintf(os.Stderr, "示例 (批量发送):\n")
		fmt.Fprintf(os.Stderr, "  bypass-mail -subject=\"季度更新\" -recipients-file=\"path/to/list.csv\" -prompt-name=\"weekly_report\" -strategy=\"round_robin_gmail\"\n\n")
		fmt.Fprintf(os.Stderr, "示例 (测试账户):\n")
		fmt.Fprintf(os.Stderr, "  bypass-mail -test-accounts -strategy=\"default\"\n\n")
		fmt.Fprintf(os.Stderr, "可用标志:\n")
		flag.PrintDefaults()
	}

	subject := flag.String("subject", "", "邮件主题 (必需，可被 CSV 中的 'subject' 列覆盖)")
	prompt := flag.String("prompt", "", "自定义邮件核心思想 (选择其一: -prompt 或 -prompt-name)")
	promptName := flag.String("prompt-name", "", "使用 ai.yaml 中的预设提示名称 (选择其一: -prompt 或 -prompt-name)")
	instructionNames := flag.String("instructions", "format_json_array", "要组合的结构化指令的逗号分隔名称 (来自 ai.yaml)")

	recipientsStr := flag.String("recipients", "", "收件人的逗号分隔列表 (例如 a@b.com,c@d.com)")
	recipientsFile := flag.String("recipients-file", "", "从文本或 CSV 文件读取收件人和个性化数据")

	templateName := flag.String("template", "default", "邮件模板名称 (来自 config.yaml)")
	defaultTitle := flag.String("title", "", "默认邮件内页标题 (如果 CSV 中未提供)")
	defaultName := flag.String("name", "", "默认收件人姓名 (如果 CSV 中未提供)")
	defaultURL := flag.String("url", "", "默认附加链接 (如果 CSV 中未提供)")
	defaultFile := flag.String("file", "", "默认附件文件路径 (如果 CSV 中未提供)")
	defaultImg := flag.String("img", "", "默认邮件标题图片路径 (本地文件，如果 CSV 中未提供)")

	strategyName := flag.String("strategy", "default", "指定要使用的发送策略 (来自 config.yaml)")
	configPath := flag.String("config", "configs/config.yaml", "主策略配置文件路径")
	aiConfigPath := flag.String("ai-config", "configs/ai.yaml", "AI 配置文件路径")
	emailConfigPath := flag.String("email-config", "configs/email.yaml", "电子邮件配置文件路径")
	testAccountsFlag := flag.Bool("test-accounts", false, "仅测试发送策略中的账户是否可用，不发送邮件")

	flag.Parse()

	if *showVersion {
		fmt.Printf("BypassMail 版本: %s\n", version)
		os.Exit(0)
	}

	// --- 2. 检查并生成初始配置 ---
	created, err := config.GenerateInitialConfigs(*configPath, *aiConfigPath, *emailConfigPath)
	if err != nil {
		log.Fatalf("❌ 初始化配置失败: %v", err)
	}
	if created {
		log.Println("✅ 已生成默认配置文件。请修改 'configs' 目录中的 .yaml 文件，特别是 API 密钥和 SMTP 账户信息，然后再次运行程序。")
		os.Exit(0)
	}

	// --- 3. 加载配置 ---
	cfg, err := config.Load(*configPath, *aiConfigPath, *emailConfigPath)
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}
	log.Println("✅ 所有配置加载成功")

	if *testAccountsFlag {
		testAccounts(cfg, *strategyName)
		os.Exit(0)
	}

	// --- 4. 验证发送策略 ---
	strategy, ok := cfg.App.SendingStrategies[*strategyName]
	if !ok {
		log.Fatalf("❌ 错误：找不到发送策略 '%s'。", *strategyName)
	}
	log.Printf("✅ 使用发送策略: '%s' (策略: %s, %d 个账户)", *strategyName, strategy.Policy, len(strategy.Accounts))
	if strategy.MaxDelay > 0 {
		log.Printf("✅ 已启用发送延迟：在 %d - %d 秒之间。", strategy.MinDelay, strategy.MaxDelay)
	}

	// --- 5. 加载收件人 ---
	allRecipientsData := loadRecipients(*recipientsFile, *recipientsStr)
	if len(allRecipientsData) == 0 {
		log.Fatal("❌ 错误：必须至少提供一个收件人。使用 -recipients 或 -recipients-file。")
	}
	log.Printf("✅ 成功为 %d 位收件人加载数据。", len(allRecipientsData))

	// --- 6. 初始化 AI ---
	provider, err := llm.NewProvider(cfg.AI)
	if err != nil {
		log.Fatalf("❌ 初始化 AI 提供程序失败: %v", err)
	}

	// --- 7. 批量处理电子邮件 ---
	templatePath, ok := cfg.App.Templates[*templateName]
	if !ok {
		log.Fatalf("❌ 错误：找不到模板 '%s'。", *templateName)
	}

	totalRecipients := len(allRecipientsData)
	logChan := make(chan logger.LogEntry, totalRecipients) // 为所有可能的日志设置缓冲区
	var wg sync.WaitGroup

	var allLogEntries []logger.LogEntry
	totalBatches := (totalRecipients + batchSize - 1) / batchSize

	for i := 0; i < totalRecipients; i += batchSize {
		end := i + batchSize
		if end > totalRecipients {
			end = totalRecipients
		}
		batchRecipients := allRecipientsData[i:end]
		batchNumber := (i / batchSize) + 1

		log.Printf("--- 正在处理批次 %d / %d (%d 个收件人) ---", batchNumber, totalBatches, len(batchRecipients))

		// --- 7.1 为当前批次构建提示 ---
		finalPrompts := buildFinalPrompts(batchRecipients, *prompt, *promptName, *instructionNames, cfg.AI)

		// --- 7.2 为当前批次生成内容 ---
		count := len(batchRecipients)
		log.Printf("🤖 正在调用 %s 为 %d 位收件人生成自定义内容...", cfg.AI.ActiveProvider, count)
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)

		combinedPromptForGeneration := strings.Join(finalPrompts, "\n---\n")
		variations, err := provider.GenerateVariations(ctx, combinedPromptForGeneration, count)
		cancel() // 此处不需要 defer，使用后立即取消

		if err != nil {
			log.Fatalf("❌ 第 %d 批的 AI 内容生成失败: %v", batchNumber, err)
		}
		if len(variations) < count {
			log.Printf("⚠️ 警告：AI 生成了 %d 个变体，少于此批次中的 %d 个收件人。某些内容将被重复使用。", len(variations), count)
			if len(variations) > 0 {
				for j := len(variations); j < count; j++ {
					variations = append(variations, variations[j%len(variations)])
				}
			} else {
				log.Fatalf("❌ AI 未能为批次 %d 生成任何内容。无法继续。", batchNumber)
			}
		} else {
			log.Printf("✅ AI 已成功为批次 %d 生成 %d 个变体。", len(variations), batchNumber)
		}

		// --- 7.3 并发发送当前批次的电子邮件 ---
		for j, data := range batchRecipients {
			wg.Add(1)
			go func(recipientIndex int, recipient RecipientData, variationContent string) {
				defer wg.Done()

				if strategy.MaxDelay > 0 {
					delay := rand.Intn(strategy.MaxDelay-strategy.MinDelay+1) + strategy.MinDelay
					log.Printf("  ...正在等待 %d 秒，然后再发送给 %s...", delay, recipient.Email)
					time.Sleep(time.Duration(delay) * time.Second)
				}

				logEntry := logger.LogEntry{
					Timestamp: time.Now().Format("2006-01-02 15:04:05"),
					Recipient: recipient.Email,
				}

				// 使用全局索引 i + recipientIndex 来确定发送帐户
				accountName := selectAccount(strategy, i+recipientIndex)
				smtpCfg, ok := cfg.Email.SMTPAccounts[accountName]
				if !ok {
					errMsg := fmt.Sprintf("在策略 '%s' 中定义的账户 '%s' 在配置中找不到。", accountName, *strategyName)
					log.Printf("❌ 错误: %s", errMsg)
					logEntry.Status = "失败"
					logEntry.Error = errMsg
					logChan <- logEntry
					return
				}
				sender := email.NewSender(smtpCfg)
				logEntry.Sender = smtpCfg.Username

				addr := strings.TrimSpace(recipient.Email)

				// **图像处理逻辑**
				var embeddedImgSrc string
				imgPath := coalesce(recipient.Img, *defaultImg)
				if imgPath != "" {
					var err error
					embeddedImgSrc, err = email.EmbedImageAsBase64(imgPath)
					if err != nil {
						log.Printf("⚠️ 警告：无法处理图像 '%s'，将跳过该图像: %v", imgPath, err)
					} else {
						log.Printf("  🖼️ 成功将图像 '%s' 嵌入到电子邮件中。", imgPath)
					}
				}

				templateData := &email.TemplateData{
					Content:   variationContent,
					Title:     coalesce(recipient.Title, *defaultTitle, *subject),
					Name:      coalesce(recipient.Name, *defaultName),
					URL:       coalesce(recipient.URL, *defaultURL),
					File:      coalesce(recipient.File, *defaultFile),
					Img:       embeddedImgSrc, // 使用处理过的 Base64 字符串
					Date:      recipient.Date,
					Sender:    smtpCfg.Username,
					Recipient: recipient.Email,
				}
				finalSubject := coalesce(recipient.Title, *subject)
				logEntry.Subject = finalSubject

				attachmentPath := coalesce(recipient.File, *defaultFile)

				htmlBody, err := email.ParseTemplate(templatePath, templateData)
				if err != nil {
					log.Printf("❌ 为 %s 解析电子邮件模板失败: %v", addr, err)
					logEntry.Status = "失败"
					logEntry.Error = fmt.Sprintf("解析模板失败: %v", err)
					logChan <- logEntry
					return
				}
				logEntry.Content = htmlBody

				log.Printf("  -> [使用 %s] 正在发送至 %s...", smtpCfg.Username, addr)
				if err := sender.Send(finalSubject, htmlBody, addr, attachmentPath); err != nil {
					log.Printf("  ❌ 发送至 %s 失败: %v", addr, err)
					logEntry.Status = "失败"
					logEntry.Error = err.Error()
				} else {
					log.Printf("  ✔️ 成功发送至 %s", addr)
					logEntry.Status = "成功"
				}
				logChan <- logEntry
			}(j, data, variations[j])
		}
		// 等待当前批次中的所有电子邮件都已发送
		wg.Wait()
		log.Printf("--- 批次 %d / %d 已处理 ---", batchNumber, totalBatches)
	}

	close(logChan)

	// 从通道收集所有日志
	for entry := range logChan {
		allLogEntries = append(allLogEntries, entry)
	}

	// --- 8. 生成最终报告 ---
	if len(allLogEntries) > 0 {
		log.Println("--- 正在生成最终的 HTML 报告 ---")
		baseReportName := fmt.Sprintf("BypassMail-Report-%s", time.Now().Format("20060102-150405"))
		if err := logger.WriteHTMLReport(baseReportName, allLogEntries, reportChunkSize); err != nil {
			log.Fatalf("❌ 生成最终 HTML 报告失败: %v", err)
		}
	}

	log.Println("🎉 所有邮件任务均已处理完毕！")
}

// loadRecipients 首先处理 CSV，然后是 TXT，最后是命令行字符串
func loadRecipients(filePath, recipientsStr string) []RecipientData {
	if filePath != "" {
		if strings.HasSuffix(strings.ToLower(filePath), ".csv") {
			return loadRecipientsFromCSV(filePath)
		}
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
		log.Printf("⚠️ 警告：无法打开文本文件 '%s'，正在跳过: %v", filePath, err)
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
		log.Printf("⚠️ 警告：读取文件 '%s' 时出错: %v", filePath, err)
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
		log.Fatal("❌ CSV 文件必须至少有一个标题行和一个数据行。")
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
			log.Printf("⚠️ 警告：CSV 中的第 %d 行缺少电子邮件，正在跳过。", i+2)
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

// buildFinalPrompts 为每个收件人构建最终的提示
func buildFinalPrompts(recipients []RecipientData, basePrompt, promptName, instructionsStr string, aiCfg *config.AIConfig) []string {
	var finalPrompts []string

	finalBasePrompt := basePrompt
	if finalBasePrompt == "" && promptName != "" {
		if p, ok := aiCfg.Prompts[promptName]; ok {
			finalBasePrompt = p
		} else {
			log.Fatalf("❌ 未找到预设提示 '%s'。", promptName)
		}
	}
	if finalBasePrompt == "" && len(recipients) > 0 && recipients[0].CustomPrompt == "" {
		log.Fatal("❌ 如果并非所有收件人在 CSV 中都有 CustomPrompt，则必须通过 -prompt 或 -prompt-name 提供基本提示。")
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
				log.Printf("⚠️ 警告：未找到结构化指令 '%s'。", trimmedName)
			}
		}
	}

	baseInstructions := instructionBuilder.String()
	for _, r := range recipients {
		var prompt strings.Builder
		prompt.WriteString(baseInstructions)

		// 如果 CSV 中有 CustomPrompt，则使用它，否则使用基本提示
		currentCoreIdea := coalesce(r.CustomPrompt, finalBasePrompt)
		prompt.WriteString("核心思想: \"" + currentCoreIdea + "\"\n")

		finalPrompts = append(finalPrompts, prompt.String())
	}
	return finalPrompts
}

// selectAccount 根据策略选择发件人帐户名
func selectAccount(strategy config.SendingStrategy, index int) string {
	numAccounts := len(strategy.Accounts)
	if numAccounts == 0 {
		log.Fatal("❌ 策略中未配置发件人帐户。")
	}

	switch strategy.Policy {
	case "round-robin":
		return strategy.Accounts[index%numAccounts]
	case "random":
		return strategy.Accounts[rand.Intn(numAccounts)]
	default:
		// 如果策略未知或未指定，则默认为循环
		return strategy.Accounts[index%numAccounts]
	}
}

// coalesce 返回字符串列表中第一个非空字符串
func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
