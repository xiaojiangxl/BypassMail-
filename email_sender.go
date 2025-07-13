package main

import (
	"fmt"
	"net/smtp"
	"strings"
)

type EmailSender struct {
	config SMTPConfig
	auth   smtp.Auth
}

func NewEmailSender(config SMTPConfig) *EmailSender {
	// PlainAuth 会在不加密的连接中以明文发送密码，但由于我们稍后会使用 STARTTLS，这是安全的。
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
	return &EmailSender{
		config: config,
		auth:   auth,
	}
}

func (s *EmailSender) Send(subject, body string, to []string) error {
	// 构造邮件消息体
	// 注意 "From", "To", "Subject" 是邮件头的一部分，需要用 \r\n 分隔
	fromHeader := fmt.Sprintf("From: %s\r\n", s.config.Username)
	toHeader := fmt.Sprintf("To: %s\r\n", strings.Join(to, ","))
	subjectHeader := fmt.Sprintf("Subject: %s\r\n", subject)
	mimeHeader := "MIME-version: 1.0;\nContent-Type: text/plain; charset=\"UTF-8\";\n\n"

	msg := []byte(fromHeader + toHeader + subjectHeader + mimeHeader + body)

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// 发送邮件
	err := smtp.SendMail(addr, s.auth, s.config.Username, to, msg)
	if err != nil {
		return fmt.Errorf("发送邮件到 %v 失败: %w", to, err)
	}

	return nil
}
