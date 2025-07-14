package email

import (
	"crypto/tls"
	"emailer-ai/internal/config"
	"fmt"
	"net/smtp"
	"strings"
)

// Sender 结构体不再依赖 gomail
type Sender struct {
	cfg  config.SMTPConfig
	from string
}

// NewSender 创建一个新的 Sender 实例
func NewSender(cfg config.SMTPConfig) *Sender {
	// From 头部需要包含别名和邮箱地址
	fromAddress := fmt.Sprintf("%s <%s>", cfg.FromAlias, cfg.Username)
	if cfg.FromAlias == "" {
		fromAddress = cfg.Username
	}

	return &Sender{
		cfg:  cfg,
		from: fromAddress,
	}
}

// Send 使用 Go 标准库 net/smtp 手动执行邮件发送
func (s *Sender) Send(subject, htmlBody string, to string) error {
	// 服务器地址
	serverAddr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	// 构建邮件头部和正文
	// 注意：这里的 \r\n 是 SMTP 协议的标准换行符
	var msgBuilder strings.Builder
	msgBuilder.WriteString("From: " + s.from + "\r\n")
	msgBuilder.WriteString("To: " + to + "\r\n")
	msgBuilder.WriteString("Subject: " + subject + "\r\n")
	msgBuilder.WriteString("MIME-version: 1.0;\r\n")
	msgBuilder.WriteString("Content-Type: text/html; charset=\"UTF-8\";\r\n")
	msgBuilder.WriteString("\r\n") // 头部和正文的空行分隔
	msgBuilder.WriteString(htmlBody)

	msg := []byte(msgBuilder.String())

	// 认证信息
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)

	// 手动执行 STARTTLS 流程
	// 1. 建立 TCP 连接
	c, err := smtp.Dial(serverAddr)
	if err != nil {
		return fmt.Errorf("failed to dial SMTP server: %w", err)
	}
	defer c.Close()

	// 2. 发送 HELO/EHLO
	if err = c.Hello("localhost"); err != nil {
		return fmt.Errorf("failed to send HELO/EHLO: %w", err)
	}

	// 3. 检查并启动 STARTTLS
	if ok, _ := c.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName:         s.cfg.Host,
			InsecureSkipVerify: true, // 关键：跳过证书验证
		}
		if err = c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS handshake: %w", err)
		}
	}

	// 4. 在加密连接上进行认证
	if err = c.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// 5. 发送邮件
	if err = smtp.SendMail(serverAddr, auth, s.cfg.Username, []string{to}, msg); err != nil {
		// 如果上面的流程可以，但 SendMail 失败，我们尝试在同一个连接上发送
		if err_send := sendData(c, s.cfg.Username, to, msg); err_send != nil {
			// 如果两种方式都失败，返回原始的 SendMail 错误并附加我们的尝试错误
			return fmt.Errorf("smtp.SendMail failed (%v) and subsequent attempt failed (%v)", err, err_send)
		}
	}

	return nil
}

// sendData 是一个辅助函数，在已建立的连接上发送邮件数据
func sendData(c *smtp.Client, from, to string, msg []byte) error {
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}
