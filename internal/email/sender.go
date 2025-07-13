package email

import (
	"crypto/tls"
	"emailer-ai/internal/config"
	"fmt"

	"gopkg.in/gomail.v2"
)

// Sender 结构体保持不变
type Sender struct {
	dialer *gomail.Dialer
	from   string
}

// NewSender 现在接收一个具体的 SMTPConfig，而不是整个配置
func NewSender(cfg config.SMTPConfig) *Sender {
	// 设置发件人地址，包含别名
	fromAddress := fmt.Sprintf("%s <%s>", cfg.FromAlias, cfg.Username)

	dialer := gomail.NewDialer(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	return &Sender{
		dialer: dialer,
		from:   fromAddress,
	}
}

// Send 方法保持不变
func (s *Sender) Send(subject, htmlBody string, to string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", s.from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)

	return s.dialer.DialAndSend(m)
}
