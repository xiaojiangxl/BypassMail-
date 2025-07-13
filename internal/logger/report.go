package logger

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"time"
)

// LogEntry 记录单次邮件发送的详细信息
type LogEntry struct {
	Timestamp string // 发送时间
	Sender    string // 发送账号
	Recipient string // 收件人
	Subject   string // 邮件标题
	Status    string // 发送状态 ("Success" 或 "Failed")
	Error     string // 如果失败，记录错误信息
	Content   string // 发送的邮件内容(HTML)
}

// reportTemplate 是用于生成HTML报告的模板字符串
const reportTemplate = `
<!DOCTYPE html>
<html lang="zh">
<head>
    <meta charset="UTF-8">
    <title>BypassMail 发送报告</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #333; background-color: #f8f9fa; margin: 0; padding: 20px; }
        .container { max-width: 1200px; margin: 20px auto; background-color: #fff; border-radius: 8px; box-shadow: 0 4px 10px rgba(0,0,0,0.05); }
        .header { background-color: #007bff; color: #ffffff; padding: 20px; text-align: center; border-top-left-radius: 8px; border-top-right-radius: 8px; }
        .header h1 { margin: 0; }
        .header p { margin: 5px 0 0; opacity: 0.9; }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 12px 15px; text-align: left; border-bottom: 1px solid #dee2e6; }
        th { background-color: #f2f2f2; font-weight: 600; }
        tr:nth-child(even) { background-color: #f9f9f9; }
        tr:hover { background-color: #f1f1f1; }
        .status-success { color: #28a745; font-weight: bold; }
        .status-failed { color: #dc3545; font-weight: bold; }
        .details { cursor: pointer; color: #007bff; text-decoration: underline; }
		.modal { display: none; position: fixed; z-index: 1; left: 0; top: 0; width: 100%; height: 100%; overflow: auto; background-color: rgba(0,0,0,0.5); }
        .modal-content { background-color: #fefefe; margin: 5% auto; padding: 20px; border: 1px solid #888; width: 80%; max-width: 800px; border-radius: 8px; box-shadow: 0 5px 15px rgba(0,0,0,0.3); }
        .close { color: #aaa; float: right; font-size: 28px; font-weight: bold; }
        .close:hover, .close:focus { color: black; text-decoration: none; cursor: pointer; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>BypassMail 发送报告</h1>
            <p>生成时间: {{.GenerationDate}}</p>
        </div>
        <table>
            <thead>
                <tr>
                    <th>时间</th>
                    <th>发送者</th>
                    <th>收件人</th>
                    <th>主题</th>
                    <th>状态</th>
                    <th>详情</th>
                </tr>
            </thead>
            <tbody>
                {{range .Logs}}
                <tr>
                    <td>{{.Timestamp}}</td>
                    <td>{{.Sender}}</td>
                    <td>{{.Recipient}}</td>
                    <td>{{.Subject}}</td>
                    <td>
                        {{if eq .Status "Success"}}
                            <span class="status-success">成功</span>
                        {{else}}
                            <span class="status-failed">失败</span>
                        {{end}}
                    </td>
                    <td>
						{{if eq .Status "Failed"}}
							<span class="details" onclick="showModal('modal-{{.Recipient}}')">查看错误</span>
						{{else}}
							<span class="details" onclick="showModal('modal-{{.Recipient}}')">查看内容</span>
						{{end}}
					</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>

	{{range .Logs}}
    <div id="modal-{{.Recipient}}" class="modal">
        <div class="modal-content">
            <span class="close" onclick="closeModal('modal-{{.Recipient}}')">&times;</span>
            <h3>发送详情: {{.Recipient}}</h3>
            <p><strong>时间:</strong> {{.Timestamp}}</p>
            <p><strong>状态:</strong> {{.Status}}</p>
            {{if .Error}}<p><strong>错误信息:</strong><br><pre>{{.Error}}</pre></p>{{end}}
            <p><strong>邮件内容:</strong></p>
            <iframe srcdoc="{{.Content}}" style="width: 100%; height: 400px; border: 1px solid #ccc;"></iframe>
        </div>
    </div>
    {{end}}

    <script>
        function showModal(id) { document.getElementById(id).style.display = "block"; }
        function closeModal(id) { document.getElementById(id).style.display = "none"; }
        window.onclick = function(event) {
            if (event.target.className === 'modal') {
                event.target.style.display = "none";
            }
        }
    </script>
</body>
</html>
`

// GenerateHTMLReport 根据日志条目生成HTML报告文件
func GenerateHTMLReport(logEntries []LogEntry) (string, error) {
	t, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return "", fmt.Errorf("无法解析HTML报告模板: %w", err)
	}

	// 创建一个带时间戳的文件名
	fileName := fmt.Sprintf("BypassMail-Report-%s.html", time.Now().Format("20060102-150405"))
	file, err := os.Create(fileName)
	if err != nil {
		return "", fmt.Errorf("无法创建报告文件 '%s': %w", fileName, err)
	}
	defer file.Close()

	// 准备模板所需的数据
	data := struct {
		GenerationDate string
		Logs           []LogEntry
	}{
		GenerationDate: time.Now().Format("2006-01-02 15:04:05"),
		Logs:           logEntries,
	}

	// 将数据渲染到模板并写入文件
	if err = t.Execute(file, data); err != nil {
		return "", fmt.Errorf("无法渲染HTML报告: %w", err)
	}

	log.Printf("✅ HTML 报告已成功生成: %s", fileName)
	return fileName, nil
}
