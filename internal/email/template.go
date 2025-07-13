package email

import (
	"bytes"
	"html/template"
	"time"
)

// TemplateData 包含更多自定义字段
type TemplateData struct {
	// 核心邮件内容，由 AI 生成
	Content string
	// 其他可自定义的模板字段
	Title string
	URL   string
	Name  string
	File  string
	Date  string // 通常在发送时动态生成
	Img   string // 图片链接
	// 新增字段
	Sender    string // 发件人账号
	Recipient string // 收件人地址
}

// ParseTemplate 函数保持不变
func ParseTemplate(templatePath string, data interface{}) (string, error) {
	// 为了动态填充日期，我们在这里处理一下
	// 如果 data 是 *TemplateData 类型，并且 Date 字段为空，则填充当前日期
	if td, ok := data.(*TemplateData); ok {
		if td.Date == "" {
			td.Date = time.Now().Format("2025-01-02")
		}
	}

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	if err = t.Execute(buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
