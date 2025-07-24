package email

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/smtp"
	"path/filepath"
	"strings"

	"emailer-ai/internal/config"
)

// Sender 结构体
type Sender struct {
	cfg  config.SMTPConfig
	from string
}

// NewSender 创建一个新的 Sender 实例
func NewSender(cfg config.SMTPConfig) *Sender {
	fromAddress := fmt.Sprintf("%s <%s>", cfg.FromAlias, cfg.Username)
	if cfg.FromAlias == "" {
		fromAddress = cfg.Username
	}
	return &Sender{
		cfg:  cfg,
		from: fromAddress,
	}
}

// buildPlainMessage 构建纯文本/HTML邮件
func (s *Sender) buildPlainMessage(subject, htmlBody, to string) []byte {
	var msgBuilder strings.Builder
	msgBuilder.WriteString("From: " + s.from + "\r\n")
	msgBuilder.WriteString("To: " + to + "\r\n")
	msgBuilder.WriteString("Subject: " + subject + "\r\n")
	msgBuilder.WriteString("MIME-version: 1.0;\r\n")
	msgBuilder.WriteString("Content-Type: text/html; charset=\"UTF-8\";\r\n")
	msgBuilder.WriteString("\r\n")
	msgBuilder.WriteString(htmlBody)
	return []byte(msgBuilder.String())
}

// buildMIMEMessage 构建带附件的MIME邮件
func (s *Sender) buildMIMEMessage(subject, htmlBody, to, attachmentPath string) ([]byte, error) {
	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)

	// 设置邮件头
	headers := make(map[string]string)
	headers["From"] = s.from
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "multipart/mixed; boundary=" + writer.Boundary()

	var headerBuilder strings.Builder
	for k, v := range headers {
		headerBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	headerBuilder.WriteString("\r\n") //
	// 写入 multipart 的正文前，先写入 header
	finalBuf := new(bytes.Buffer)
	finalBuf.WriteString(headerBuilder.String())

	// HTML 部分
	htmlPart, err := writer.CreatePart(map[string][]string{
		"Content-Type":              {"text/html; charset=\"UTF-8\""},
		"Content-Transfer-Encoding": {"8bit"},
	})
	if err != nil {
		return nil, err
	}
	_, err = htmlPart.Write([]byte(htmlBody))
	if err != nil {
		return nil, err
	}

	// 附件部分
	fileBytes, err := ioutil.ReadFile(attachmentPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取附件 '%s': %w", attachmentPath, err)
	}

	attachmentPart, err := writer.CreatePart(map[string][]string{
		"Content-Type":              {"application/octet-stream"},
		"Content-Transfer-Encoding": {"base64"},
		"Content-Disposition":       {fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(attachmentPath))},
	})
	if err != nil {
		return nil, err
	}

	b64 := make([]byte, base64.StdEncoding.EncodedLen(len(fileBytes)))
	base64.StdEncoding.Encode(b64, fileBytes)
	_, err = attachmentPart.Write(b64)
	if err != nil {
		return nil, err
	}

	writer.Close()

	// 将 multipart 的内容追加到 header 后面
	finalBuf.Write(buf.Bytes())

	return finalBuf.Bytes(), nil
}

// Send 函数现在支持附件
func (s *Sender) Send(subject, htmlBody string, to string, attachmentPath string) error {
	serverAddr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)

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

	// 如果 'to' 为空，则认为这是一个测试连接的请求，认证成功后直接退出
	if to == "" {
		return c.Quit()
	}

	var msg []byte
	if attachmentPath != "" {
		fmt.Printf("  📎 发现附件，构建MIME邮件: %s\n", attachmentPath)
		msg, err = s.buildMIMEMessage(subject, htmlBody, to, attachmentPath)
		if err != nil {
			return err
		}
	} else {
		msg = s.buildPlainMessage(subject, htmlBody, to)
	}

	// 5. 在同一个连接上发送邮件数据
	return sendData(c, s.cfg.Username, to, msg)
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
