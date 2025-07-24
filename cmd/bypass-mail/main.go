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
	version = "dev" // 預設值為 'dev'，可以透過 ldflags 在編譯時覆寫
)

const (
	// 定义批处理大小
	batchSize = 50
)

// RecipientData 用于存储从 CSV 或其他来源读取的每一行个人化数据
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

// testAccounts 函数用于测试发件箱账号的连通性
func testAccounts(cfg *config.Config, strategyName string) {
	strategy, ok := cfg.App.SendingStrategies[strategyName]
	if !ok {
		log.Fatalf("❌ 错误：找不到名为 '%s' 的发送策略。", strategyName)
	}

	log.Printf("🧪 开始测试策略 '%s' 中的 %d 个发件账户...", strategyName, len(strategy.Accounts))
	var wg sync.WaitGroup
	results := make(chan string, len(strategy.Accounts))

	for _, accountName := range strategy.Accounts {
		wg.Add(1)
		go func(accName string) {
			defer wg.Done()
			smtpCfg, ok := cfg.Email.SMTPAccounts[accName]
			if !ok {
				results <- fmt.Sprintf("  - [ %-20s ] ❌ 配置未找到", accName)
				return
			}
			sender := email.NewSender(smtpCfg)
			// 在测试模式下，我们传递一个空的附件路径
			if err := sender.Send("", "", "", ""); err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "authentication failed") ||
					strings.Contains(strings.ToLower(err.Error()), "username and password not accepted") ||
					strings.Contains(err.Error(), "connection refused") ||
					strings.Contains(err.Error(), "timeout") {
					results <- fmt.Sprintf("  - [ %-20s ] ❌ 失败: %v", smtpCfg.Username, err)
				} else {
					results <- fmt.Sprintf("  - [ %-20s ] ✔️ 成功", smtpCfg.Username)
				}
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
	log.Println("✅ 账号测试完成。")
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// --- 1. 命令行参数定义与文档 ---
	showVersion := flag.Bool("version", false, "显示工具的版本号并退出")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "BypassMail: AI 驱动的个性化邮件批量发送工具。\n\n")
		fmt.Fprintf(os.Stderr, "使用方法:\n  bypass-mail [flags]\n\n")
		fmt.Fprintf(os.Stderr, "示例 (批量发送):\n")
		fmt.Fprintf(os.Stderr, "  bypass-mail -subject=\"季度更新\" -recipients-file=\"path/to/list.csv\" -prompt-name=\"weekly_report\" -strategy=\"round_robin_gmail\"\n\n")
		fmt.Fprintf(os.Stderr, "示例 (测试账号):\n")
		fmt.Fprintf(os.Stderr, "  bypass-mail -test-accounts -strategy=\"default\"\n\n")
		fmt.Fprintf(os.Stderr, "可用参数:\n")
		flag.PrintDefaults()
	}

	subject := flag.String("subject", "", "邮件主题 (必需, 可被 CSV 中的 subject 列覆盖)")
	prompt := flag.String("prompt", "", "自定义邮件核心思想 (与 -prompt-name 二选一)")
	promptName := flag.String("prompt-name", "", "使用 ai.yaml 中预设的提示词名称 (与 -prompt 二选一)")
	instructionNames := flag.String("instructions", "format_json_array", "要组合的结构化指令名称, 逗号分隔 (来自 ai.yaml)")

	recipientsStr := flag.String("recipients", "", "收件人列表, 逗号分隔 (例如: a@b.com,c@d.com)")
	recipientsFile := flag.String("recipients-file", "", "从文本或 CSV 文件读取收件人及个人化数据")

	templateName := flag.String("template", "default", "邮件模板名称 (来自 config.yaml)")
	defaultTitle := flag.String("title", "", "默认邮件内页标题 (若 CSV 未提供)")
	defaultName := flag.String("name", "", "默认收件人称呼 (若 CSV 未提供)")
	defaultURL := flag.String("url", "", "默认附加链接 (若 CSV 未提供)")
	defaultFile := flag.String("file", "", "默认附加文件路径 (若 CSV 未提供)")
	defaultImg := flag.String("img", "", "默认邮件头图路径 (本地文件, 若 CSV 未提供)")

	strategyName := flag.String("strategy", "default", "指定使用的发件策略 (来自 config.yaml)")
	configPath := flag.String("config", "configs/config.yaml", "主策略配置文件路径")
	aiConfigPath := flag.String("ai-config", "configs/ai.yaml", "AI 配置文件路径")
	emailConfigPath := flag.String("email-config", "configs/email.yaml", "Email 配置文件路径")
	testAccountsFlag := flag.Bool("test-accounts", false, "仅测试发件策略中的账户是否可用，不发送邮件")

	flag.Parse()

	if *showVersion {
		fmt.Printf("BypassMail version: %s\n", version)
		os.Exit(0)
	}

	// --- 2. 检查并生成初始配置 ---
	created, err := config.GenerateInitialConfigs(*configPath, *aiConfigPath, *emailConfigPath)
	if err != nil {
		log.Fatalf("❌ 初始化配置失败: %v", err)
	}
	if created {
		log.Println("✅ 默认配置文件已生成。请根据您的需求修改 'configs' 目录下的 .yaml 文件，特别是 API Keys 和 SMTP 账户信息，然后重新运行程序。")
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
		log.Fatalf("❌ 错误：找不到名为 '%s' 的发送策略。", *strategyName)
	}
	log.Printf("✅ 使用发件策略: '%s' (策略: %s, 包含 %d 个账号)", *strategyName, strategy.Policy, len(strategy.Accounts))
	if strategy.MaxDelay > 0 {
		log.Printf("✅ 发送延时已启用: %d - %d 秒之间。", strategy.MinDelay, strategy.MaxDelay)
	}

	// --- 5. 加载收件人 ---
	allRecipientsData := loadRecipients(*recipientsFile, *recipientsStr)
	if len(allRecipientsData) == 0 {
		log.Fatal("❌ 错误: 必须提供至少一个收件人。使用 -recipients 或 -recipients-file 指定。")
	}
	log.Printf("✅ 成功加载 %d 位收件人的数据。", len(allRecipientsData))

	// --- 6. 初始化 AI ---
	provider, err := llm.NewProvider(cfg.AI)
	if err != nil {
		log.Fatalf("❌ 初始化AI提供商失败: %v", err)
	}

	// --- 7. 按批次处理邮件 ---
	templatePath, ok := cfg.App.Templates[*templateName]
	if !ok {
		log.Fatalf("❌ 错误：找不到名为 '%s' 的模板。", *templateName)
	}

	totalRecipients := len(allRecipientsData)
	logChan := make(chan logger.LogEntry, totalRecipients)
	var wg sync.WaitGroup

	for i := 0; i < totalRecipients; i += batchSize {
		end := i + batchSize
		if end > totalRecipients {
			end = totalRecipients
		}
		batchRecipients := allRecipientsData[i:end]
		batchNumber := (i / batchSize) + 1
		totalBatches := (totalRecipients + batchSize - 1) / batchSize

		log.Printf("--- 正在处理第 %d / %d 批次 (共 %d 位收件人) ---", batchNumber, totalBatches, len(batchRecipients))

		// --- 7.1 为当前批次构建提示词 ---
		finalPrompts := buildFinalPrompts(batchRecipients, *prompt, *promptName, *instructionNames, cfg.AI)

		// --- 7.2 为当前批次生成内容 ---
		count := len(batchRecipients)
		log.Printf("🤖 正在调用 %s 为 %d 位收件人生成定制化邮件文案...", cfg.AI.ActiveProvider, count)
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		combinedPromptForGeneration := strings.Join(finalPrompts, "\n---\n")
		variations, err := provider.GenerateVariations(ctx, combinedPromptForGeneration, count)
		if err != nil {
			log.Fatalf("❌ 批次 %d 的 AI 生成内容失败: %v", batchNumber, err)
		}
		if len(variations) < count {
			log.Printf("⚠️ 警告: AI 生成的文案数量 (%d) 少于当前批次的收件人数量 (%d)，部分收件人将收到重复内容。", len(variations), count)
			if len(variations) > 0 {
				for j := len(variations); j < count; j++ {
					variations = append(variations, variations[j%len(variations)])
				}
			} else {
				log.Fatalf("❌ AI 未能为批次 %d 生成任何内容，无法继续发送。", batchNumber)
			}
		} else {
			log.Printf("✅ AI 已成功为批次 %d 生成 %d 份不同文案。", batchNumber, len(variations))
		}

		// --- 7.3 并发发送当前批次的邮件 ---
		for j, data := range batchRecipients {
			wg.Add(1)
			go func(recipientIndex int, recipient RecipientData, variationContent string) {
				defer wg.Done()

				if strategy.MaxDelay > 0 {
					delay := rand.Intn(strategy.MaxDelay-strategy.MinDelay+1) + strategy.MinDelay
					log.Printf("  ...等待 %d 秒后发送给 %s...", delay, recipient.Email)
					time.Sleep(time.Duration(delay) * time.Second)
				}

				logEntry := logger.LogEntry{
					Timestamp: time.Now().Format("2006-01-02 15:04:05"),
					Recipient: recipient.Email,
				}

				// 使用全局索引 i + recipientIndex 来决定发件账户
				accountName := selectAccount(strategy, i+recipientIndex)
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

				// **图片处理新逻辑**
				var embeddedImgSrc string
				imgPath := coalesce(recipient.Img, *defaultImg)
				if imgPath != "" {
					var err error
					embeddedImgSrc, err = email.EmbedImageAsBase64(imgPath)
					if err != nil {
						log.Printf("⚠️ 警告: 无法处理图片 '%s'，将忽略此图片: %v", imgPath, err)
					} else {
						log.Printf("  🖼️ 已成功将图片 '%s' 嵌入邮件。", imgPath)
					}
				}

				templateData := &email.TemplateData{
					Content:   variationContent,
					Title:     coalesce(recipient.Title, *defaultTitle, *subject),
					Name:      coalesce(recipient.Name, *defaultName),
					URL:       coalesce(recipient.URL, *defaultURL),
					File:      coalesce(recipient.File, *defaultFile),
					Img:       embeddedImgSrc, // 使用处理后的 Base64 字符串
					Date:      recipient.Date,
					Sender:    smtpCfg.Username,
					Recipient: recipient.Email,
				}
				finalSubject := coalesce(recipient.Title, *subject)
				logEntry.Subject = finalSubject

				attachmentPath := coalesce(recipient.File, *defaultFile)

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
				if err := sender.Send(finalSubject, htmlBody, addr, attachmentPath); err != nil {
					log.Printf("  ❌ 发送给 %s 失败: %v", addr, err)
					logEntry.Status = "Failed"
					logEntry.Error = err.Error()
				} else {
					log.Printf("  ✔️ 成功发送给 %s", addr)
					logEntry.Status = "Success"
				}
				logChan <- logEntry
			}(j, data, variations[j])
		}
		// 等待当前批次的所有邮件发送完成
		wg.Wait()
		log.Printf("--- 第 %d / %d 批次处理完成 ---", batchNumber, totalBatches)
	}

	close(logChan)

	// --- 8. 生成报告 ---
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
