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
	version = "dev" // Default value is 'dev', can be overwritten at compile time with ldflags
)

const (
	// Define batch processing size
	batchSize = 50
)

// RecipientData is used to store each line of personalized data read from CSV or other sources
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

// testAccounts function is used to test the connectivity of sender accounts
func testAccounts(cfg *config.Config, strategyName string) {
	strategy, ok := cfg.App.SendingStrategies[strategyName]
	if !ok {
		log.Fatalf("❌ Error: Sending strategy '%s' not found.", strategyName)
	}

	log.Printf("🧪 Starting test for %d sender accounts in strategy '%s'...", len(strategy.Accounts), strategyName)
	var wg sync.WaitGroup
	results := make(chan string, len(strategy.Accounts))

	for _, accountName := range strategy.Accounts {
		wg.Add(1)
		go func(accName string) {
			defer wg.Done()
			smtpCfg, ok := cfg.Email.SMTPAccounts[accName]
			if !ok {
				results <- fmt.Sprintf("  - [ %-20s ] ❌ Configuration not found", accName)
				return
			}
			sender := email.NewSender(smtpCfg)
			// In test mode, we pass an empty recipient address.
			// The sender.Send function will handle this and only perform connection and authentication tests.
			if err := sender.Send("", "", "", ""); err != nil {
				results <- fmt.Sprintf("  - [ %-20s ] ❌ Failed: %v", smtpCfg.Username, err)
			} else {
				results <- fmt.Sprintf("  - [ %-20s ] ✔️ Success", smtpCfg.Username)
			}
		}(accountName)
	}

	wg.Wait()
	close(results)

	for res := range results {
		log.Println(res)
	}
	log.Println("✅ Account test completed.")
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// --- 1. Command-line argument definition and documentation ---
	showVersion := flag.Bool("version", false, "Show the tool version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "BypassMail: AI-driven personalized bulk email sending tool.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  bypass-mail [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Example (Bulk Send):\n")
		fmt.Fprintf(os.Stderr, "  bypass-mail -subject=\"Quarterly Update\" -recipients-file=\"path/to/list.csv\" -prompt-name=\"weekly_report\" -strategy=\"round_robin_gmail\"\n\n")
		fmt.Fprintf(os.Stderr, "Example (Test Accounts):\n")
		fmt.Fprintf(os.Stderr, "  bypass-mail -test-accounts -strategy=\"default\"\n\n")
		fmt.Fprintf(os.Stderr, "Available Flags:\n")
		flag.PrintDefaults()
	}

	subject := flag.String("subject", "", "Email subject (required, can be overridden by 'subject' column in CSV)")
	prompt := flag.String("prompt", "", "Custom core idea for the email (choose one: -prompt or -prompt-name)")
	promptName := flag.String("prompt-name", "", "Use a preset prompt name from ai.yaml (choose one: -prompt or -prompt-name)")
	instructionNames := flag.String("instructions", "format_json_array", "Comma-separated names of structured instructions to combine (from ai.yaml)")

	recipientsStr := flag.String("recipients", "", "Comma-separated list of recipients (e.g., a@b.com,c@d.com)")
	recipientsFile := flag.String("recipients-file", "", "Read recipients and personalized data from a text or CSV file")

	templateName := flag.String("template", "default", "Email template name (from config.yaml)")
	defaultTitle := flag.String("title", "", "Default email inner page title (if not provided in CSV)")
	defaultName := flag.String("name", "", "Default recipient name (if not provided in CSV)")
	defaultURL := flag.String("url", "", "Default additional link (if not provided in CSV)")
	defaultFile := flag.String("file", "", "Default attachment file path (if not provided in CSV)")
	defaultImg := flag.String("img", "", "Default email header image path (local file, if not provided in CSV)")

	strategyName := flag.String("strategy", "default", "Specify the sending strategy to use (from config.yaml)")
	configPath := flag.String("config", "configs/config.yaml", "Main strategy configuration file path")
	aiConfigPath := flag.String("ai-config", "configs/ai.yaml", "AI configuration file path")
	emailConfigPath := flag.String("email-config", "configs/email.yaml", "Email configuration file path")
	testAccountsFlag := flag.Bool("test-accounts", false, "Only test if accounts in the sending strategy are available, without sending emails")

	flag.Parse()

	if *showVersion {
		fmt.Printf("BypassMail version: %s\n", version)
		os.Exit(0)
	}

	// --- 2. Check and generate initial configurations ---
	created, err := config.GenerateInitialConfigs(*configPath, *aiConfigPath, *emailConfigPath)
	if err != nil {
		log.Fatalf("❌ Failed to initialize configurations: %v", err)
	}
	if created {
		log.Println("✅ Default configuration files have been generated. Please modify the .yaml files in the 'configs' directory with your details, especially API Keys and SMTP account information, then run the program again.")
		os.Exit(0)
	}

	// --- 3. Load configurations ---
	cfg, err := config.Load(*configPath, *aiConfigPath, *emailConfigPath)
	if err != nil {
		log.Fatalf("❌ Failed to load configurations: %v", err)
	}
	log.Println("✅ All configurations loaded successfully")

	if *testAccountsFlag {
		testAccounts(cfg, *strategyName)
		os.Exit(0)
	}

	// --- 4. Validate sending strategy ---
	strategy, ok := cfg.App.SendingStrategies[*strategyName]
	if !ok {
		log.Fatalf("❌ Error: Sending strategy '%s' not found.", *strategyName)
	}
	log.Printf("✅ Using sending strategy: '%s' (Policy: %s, %d accounts)", *strategyName, strategy.Policy, len(strategy.Accounts))
	if strategy.MaxDelay > 0 {
		log.Printf("✅ Sending delay enabled: between %d - %d seconds.", strategy.MinDelay, strategy.MaxDelay)
	}

	// --- 5. Load recipients ---
	allRecipientsData := loadRecipients(*recipientsFile, *recipientsStr)
	if len(allRecipientsData) == 0 {
		log.Fatal("❌ Error: At least one recipient must be provided. Use -recipients or -recipients-file.")
	}
	log.Printf("✅ Successfully loaded data for %d recipients.", len(allRecipientsData))

	// --- 6. Initialize AI ---
	provider, err := llm.NewProvider(cfg.AI)
	if err != nil {
		log.Fatalf("❌ Failed to initialize AI provider: %v", err)
	}

	// --- 7. Process emails in batches ---
	templatePath, ok := cfg.App.Templates[*templateName]
	if !ok {
		log.Fatalf("❌ Error: Template '%s' not found.", *templateName)
	}

	totalRecipients := len(allRecipientsData)
	logChan := make(chan logger.LogEntry, totalRecipients) // Buffer for all possible logs
	var wg sync.WaitGroup

	// ✨ Initialize report file name and the master log list outside the loop
	reportFileName := fmt.Sprintf("BypassMail-Report-%s.html", time.Now().Format("20060102-150405"))
	var allLogEntries []logger.LogEntry

	// ✨ **FIXED**: Correct calculation of totalBatches
	totalBatches := (totalRecipients + batchSize - 1) / batchSize

	for i := 0; i < totalRecipients; i += batchSize {
		end := i + batchSize
		if end > totalRecipients {
			end = totalRecipients
		}
		batchRecipients := allRecipientsData[i:end]
		batchNumber := (i / batchSize) + 1

		log.Printf("--- Processing batch %d / %d (%d recipients) ---", batchNumber, totalBatches, len(batchRecipients))

		// --- 7.1 Build prompts for the current batch ---
		finalPrompts := buildFinalPrompts(batchRecipients, *prompt, *promptName, *instructionNames, cfg.AI)

		// --- 7.2 Generate content for the current batch ---
		count := len(batchRecipients)
		log.Printf("🤖 Calling %s to generate custom content for %d recipients...", cfg.AI.ActiveProvider, count)
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)

		combinedPromptForGeneration := strings.Join(finalPrompts, "\n---\n")
		variations, err := provider.GenerateVariations(ctx, combinedPromptForGeneration, count)
		cancel() // Defer is not needed here, cancel right after use

		if err != nil {
			log.Fatalf("❌ AI content generation failed for batch %d: %v", batchNumber, err)
		}
		if len(variations) < count {
			log.Printf("⚠️ Warning: AI generated %d variations, which is less than the %d recipients in this batch. Some content will be reused.", len(variations), count)
			if len(variations) > 0 {
				for j := len(variations); j < count; j++ {
					variations = append(variations, variations[j%len(variations)])
				}
			} else {
				log.Fatalf("❌ AI failed to generate any content for batch %d. Cannot continue.", batchNumber)
			}
		} else {
			log.Printf("✅ AI successfully generated %d variations for batch %d.", len(variations), batchNumber)
		}

		// --- 7.3 Concurrently send emails for the current batch ---
		for j, data := range batchRecipients {
			wg.Add(1)
			go func(recipientIndex int, recipient RecipientData, variationContent string) {
				defer wg.Done()

				if strategy.MaxDelay > 0 {
					delay := rand.Intn(strategy.MaxDelay-strategy.MinDelay+1) + strategy.MinDelay
					log.Printf("  ...waiting %d seconds before sending to %s...", delay, recipient.Email)
					time.Sleep(time.Duration(delay) * time.Second)
				}

				logEntry := logger.LogEntry{
					Timestamp: time.Now().Format("2006-01-02 15:04:05"),
					Recipient: recipient.Email,
				}

				// Use global index i + recipientIndex to determine the sending account
				accountName := selectAccount(strategy, i+recipientIndex)
				smtpCfg, ok := cfg.Email.SMTPAccounts[accountName]
				if !ok {
					errMsg := fmt.Sprintf("Account '%s' defined in strategy '%s' not found in configurations.", accountName, *strategyName)
					log.Printf("❌ Error: %s", errMsg)
					logEntry.Status = "Failed"
					logEntry.Error = errMsg
					logChan <- logEntry
					return
				}
				sender := email.NewSender(smtpCfg)
				logEntry.Sender = smtpCfg.Username

				addr := strings.TrimSpace(recipient.Email)

				// **Image processing logic**
				var embeddedImgSrc string
				imgPath := coalesce(recipient.Img, *defaultImg)
				if imgPath != "" {
					var err error
					embeddedImgSrc, err = email.EmbedImageAsBase64(imgPath)
					if err != nil {
						log.Printf("⚠️ Warning: Could not process image '%s', it will be skipped: %v", imgPath, err)
					} else {
						log.Printf("  🖼️ Successfully embedded image '%s' into email.", imgPath)
					}
				}

				templateData := &email.TemplateData{
					Content:   variationContent,
					Title:     coalesce(recipient.Title, *defaultTitle, *subject),
					Name:      coalesce(recipient.Name, *defaultName),
					URL:       coalesce(recipient.URL, *defaultURL),
					File:      coalesce(recipient.File, *defaultFile),
					Img:       embeddedImgSrc, // Use the processed Base64 string
					Date:      recipient.Date,
					Sender:    smtpCfg.Username,
					Recipient: recipient.Email,
				}
				finalSubject := coalesce(recipient.Title, *subject)
				logEntry.Subject = finalSubject

				attachmentPath := coalesce(recipient.File, *defaultFile)

				htmlBody, err := email.ParseTemplate(templatePath, templateData)
				if err != nil {
					log.Printf("❌ Failed to parse email template for %s: %v", addr, err)
					logEntry.Status = "Failed"
					logEntry.Error = fmt.Sprintf("Failed to parse template: %v", err)
					logChan <- logEntry
					return
				}
				logEntry.Content = htmlBody

				log.Printf("  -> [Using %s] Sending to %s...", smtpCfg.Username, addr)
				if err := sender.Send(finalSubject, htmlBody, addr, attachmentPath); err != nil {
					log.Printf("  ❌ Failed to send to %s: %v", addr, err)
					logEntry.Status = "Failed"
					logEntry.Error = err.Error()
				} else {
					log.Printf("  ✔️ Successfully sent to %s", addr)
					logEntry.Status = "Success"
				}
				logChan <- logEntry
			}(j, data, variations[j])
		}
		// Wait for all emails in the current batch to be sent
		wg.Wait()

		// ✨ **FIXED**: Collect logs from the channel and update the report
		batchLogCount := len(batchRecipients)
		for k := 0; k < batchLogCount; k++ {
			entry := <-logChan
			allLogEntries = append(allLogEntries, entry)
		}

		if err := logger.WriteHTMLReport(reportFileName, allLogEntries); err != nil {
			log.Printf("❌ Failed to update HTML report: %v", err)
		}

		log.Printf("--- Batch %d / %d processed ---", batchNumber, totalBatches)
	}

	close(logChan)

	// --- 8. Generate Final Report ---
	// The report is already generated/updated, this is just a final message.
	log.Println("🎉 All email tasks have been processed!")
}

// loadRecipients first handles CSV, then TXT, and finally the command-line string
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
		log.Printf("⚠️ Warning: Cannot open text file '%s', skipping: %v", filePath, err)
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
		log.Printf("⚠️ Warning: Error reading file '%s': %v", filePath, err)
	}
	return data
}

func loadRecipientsFromCSV(filePath string) []RecipientData {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("❌ Cannot open CSV file '%s': %v", filePath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("❌ Failed to parse CSV file: %v", err)
	}

	if len(records) < 2 {
		log.Fatal("❌ CSV file must have at least a header row and one data row.")
	}

	header := records[0]
	headerMap := make(map[string]int)
	for i, h := range header {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	if _, ok := headerMap["email"]; !ok {
		log.Fatal("❌ CSV file must contain a column named 'email'.")
	}

	var data []RecipientData
	for i, row := range records[1:] {
		recipient := RecipientData{}
		if idx, ok := headerMap["email"]; ok {
			recipient.Email = row[idx]
		}
		if recipient.Email == "" {
			log.Printf("⚠️ Warning: Row %d in CSV is missing an email, skipping.", i+2)
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

// buildFinalPrompts builds the final prompt for each recipient
func buildFinalPrompts(recipients []RecipientData, basePrompt, promptName, instructionsStr string, aiCfg *config.AIConfig) []string {
	var finalPrompts []string

	finalBasePrompt := basePrompt
	if finalBasePrompt == "" && promptName != "" {
		if p, ok := aiCfg.Prompts[promptName]; ok {
			finalBasePrompt = p
		} else {
			log.Fatalf("❌ Preset prompt '%s' not found.", promptName)
		}
	}
	if finalBasePrompt == "" && len(recipients) > 0 && recipients[0].CustomPrompt == "" {
		log.Fatal("❌ A base prompt must be provided via -prompt or -prompt-name if not all recipients have a CustomPrompt in the CSV.")
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
				log.Printf("⚠️ Warning: Structured instruction '%s' not found.", trimmedName)
			}
		}
	}

	baseInstructions := instructionBuilder.String()
	for _, r := range recipients {
		var prompt strings.Builder
		prompt.WriteString(baseInstructions)

		// Use CustomPrompt from CSV if available, otherwise use the base prompt
		currentCoreIdea := coalesce(r.CustomPrompt, finalBasePrompt)
		prompt.WriteString("Core idea: \"" + currentCoreIdea + "\"\n")

		finalPrompts = append(finalPrompts, prompt.String())
	}
	return finalPrompts
}

// selectAccount selects a sender account name based on the strategy
func selectAccount(strategy config.SendingStrategy, index int) string {
	numAccounts := len(strategy.Accounts)
	if numAccounts == 0 {
		log.Fatal("❌ No sender accounts configured in the strategy.")
	}

	switch strategy.Policy {
	case "round-robin":
		return strategy.Accounts[index%numAccounts]
	case "random":
		return strategy.Accounts[rand.Intn(numAccounts)]
	default:
		// Default to round-robin if policy is unknown or not specified
		return strategy.Accounts[index%numAccounts]
	}
}

// coalesce returns the first non-empty string from a list of strings
func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
