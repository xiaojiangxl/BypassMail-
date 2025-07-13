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

> go run ./cmd/bypass-mail/ -subject="项目更新" -prompt="本周项目进展顺利，下周计划..." -recipients="a@a.com,b@b.com"