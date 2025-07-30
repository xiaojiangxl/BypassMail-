package logger

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"time"
)

// LogEntry 结构体和 reportTemplate 常量保持不变...
// LogEntry records a single email sending detail
type LogEntry struct {
	Timestamp string // Sending time
	Sender    string // Sender account
	Recipient string // Recipient
	Subject   string // Email subject
	Status    string // Sending status ("Success" or "Failed")
	Error     string // Error message if failed
	Content   string // Sent email content (HTML)
}

// reportTemplate is the template string for generating the HTML report
// ✨【关键改动】模板已更新，使用索引作为唯一ID
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
                {{range $i, $log := .Logs}}
                <tr>
                    <td>{{$log.Timestamp}}</td>
                    <td>{{$log.Sender}}</td>
                    <td>{{$log.Recipient}}</td>
                    <td>{{$log.Subject}}</td>
                    <td>
                        {{if eq $log.Status "成功"}}
                            <span class="status-success">成功</span>
                        {{else}}
                            <span class="status-failed">失败</span>
                        {{end}}
                    </td>
                    <td>
						{{if eq $log.Status "Failed"}}
							<span class="details" onclick="showModal('modal-{{$i}}')">查看错误</span>
						{{else}}
							<span class="details" onclick="showModal('modal-{{$i}}')">查看内容</span>
						{{end}}
					</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>

	{{range $i, $log := .Logs}}
    <div id="modal-{{$i}}" class="modal">
        <div class="modal-content">
            <span class="close" onclick="closeModal('modal-{{$i}}')">&times;</span>
            <h3>发送详情: {{$log.Recipient}}</h3>
            <p><strong>时间:</strong> {{$log.Timestamp}}</p>
            <p><strong>状态:</strong> {{$log.Status}}</p>
            {{if $log.Error}}<p><strong>错误信息:</strong><br><pre>{{$log.Error}}</pre></p>{{end}}
            <p><strong>邮件内容:</strong></p>
            <iframe srcdoc="{{$log.Content}}" style="width: 100%; height: 400px; border: 1px solid #ccc;"></iframe>
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

// WriteHTMLReport 根据给定的日志条目，生成或覆盖HTML报告文件
// 现在它会在日志超过阈值时创建新的分块文件
func WriteHTMLReport(baseFileName string, logEntries []LogEntry, reportChunkSize int) error {
	totalLogs := len(logEntries)
	if totalLogs == 0 {
		return nil
	}

	numReports := (totalLogs + reportChunkSize - 1) / reportChunkSize

	t, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("无法解析HTML报告模板: %w", err)
	}

	for i := 0; i < numReports; i++ {
		start := i * reportChunkSize
		end := start + reportChunkSize
		if end > totalLogs {
			end = totalLogs
		}
		chunkLogs := logEntries[start:end]

		// 为每个分块文件生成一个唯一的文件名
		var chunkFileName string
		// 如果只有一个报告，则不添加 part 后缀
		if numReports > 1 {
			chunkFileName = fmt.Sprintf("%s-part-%d.html", baseFileName, i+1)
		} else {
			// 移除 .html 后缀（如果存在）再添加，以避免重复
			base := baseFileName
			if ext := ".html"; len(base) > len(ext) && base[len(base)-len(ext):] == ext {
				base = base[:len(base)-len(ext)]
			}
			chunkFileName = fmt.Sprintf("%s.html", base)
		}

		file, err := os.Create(chunkFileName)
		if err != nil {
			return fmt.Errorf("无法创建或覆盖报告文件 '%s': %w", chunkFileName, err)
		}
		defer file.Close()

		data := struct {
			GenerationDate string
			Logs           []LogEntry
		}{
			GenerationDate: time.Now().Format("2006-01-02 15:04:05"),
			Logs:           chunkLogs,
		}

		if err = t.Execute(file, data); err != nil {
			// 在关闭文件前返回错误
			return fmt.Errorf("无法为 '%s' 渲染HTML报告: %w", chunkFileName, err)
		}
		log.Printf("✅ HTML 报告分块已生成/更新: %s (%d 条记录)", chunkFileName, len(chunkLogs))
	}

	return nil
}
