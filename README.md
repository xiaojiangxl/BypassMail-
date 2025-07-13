# BypassMail-

```
bypass-mail/
├── cmd/
│   └── bypass-mail/
│       └── main.go           # 程序主入口
├── internal/
│   ├── config/
│   │   └── config.go         # 配置加载与验证
│   ├── email/
│   │   ├── sender.go         # 邮件发送逻辑 (使用 gomail)
│   │   └── template.go       # HTML 模板处理
│   └── llm/
│       ├── provider.go       # 定义统一的 AI 提供商接口
│       ├── gemini.go         # Gemini 实现
│       ├── doubao.go         # 豆包 (Lark) 实现
│       ├── deepseek.go       # DeepSeek 实现
│       └── factory.go        # 根据配置创建对应的 AI 实例
├── configs/
│   └── config.yaml           # 新的配置文件 (使用 YAML 格式更清晰)
├── templates/
│   └── email_template.html   # 邮件的 HTML 模板
├── go.mod                    # Go 模块文件
├── go.sum
└── README.md
```

## 运行

批量发送

```
go run ./cmd/bypass-mail/ \
    -subject="项目季度回顾与展望" \
    -recipients-file="recipients.txt" \
    -prompt-name="weekly_report" \
    -template="formal" \
    -from="marketing" \
    -title="Q3 季度重要更新" \
    -name="尊敬的合作伙伴" \
    -url="https://your-company.com/q3-report"
```

单独发送

```
go run ./cmd/bypass-mail/ \
    -subject="一个临时的紧急通知" \
    -recipients="boss@company.com" \
    -prompt="今晚服务器需要紧急维护，预计从晚上10点到11点服务不可用，请周知。" \
    -from="default"
```