package email

import (
	"crypto/tls"
	"emailer-ai/internal/config" // 假设模块名是 project
	"fmt"

	"gopkg.in/gomail.v2"
)

type Sender struct {
	dialer *gomail.Dialer
	from   string
}

func NewSender(cfg config.SMTPConfig) *Sender {
	// 设置发件人地址，包含别名
	fromAddress := fmt.Sprintf("%s <%s>", cfg.FromAlias, cfg.Username)

	dialer := gomail.NewDialer(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	// 许多 SMTP 服务器需要这个设置
	dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	return &Sender{
		dialer: dialer,
		from:   fromAddress,
	}
}

func (s *Sender) Send(subject, htmlBody string, to string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", s.from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)

	return s.dialer.DialAndSend(m)
}
