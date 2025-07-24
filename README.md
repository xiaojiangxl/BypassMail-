# BypassMail: 高级邮件规避与渗透测试工具

**BypassMail** 是一款专为网络安全专业人士、红队渗透测试人员和安全研究员设计的邮件发送工具。它利用先进的人工智能技术和多样化的发送策略，旨在绕过主流的邮件安全网关（ESG）、垃圾邮件过滤器和各种反钓鱼检测机制。

## 核心规避技术 (Core Evasion Techniques)

传统的邮件群发工具因其固定的内容和发送模式极易被安全系统拦截。BypassMail 采用以下多层混合技术，显著提高邮件送达率和隐蔽性：

#### 1. **动态内容生成 (Dynamic Content Generation)**
- **AI 驱动的内容变体**: BypassMail 的核心优势在于它集成了多种大型语言模型（LLMs），如 DeepSeek, Gemini 等。它不依赖固定的邮件模板，而是根据您提供的核心思想（Prompt），为每一个收件人动态生成措辞、语气和结构都不同的邮件正文。这使得每一封邮件在内容上都是独一无二的，从而有效绕过基于内容签名和重复模式的垃圾邮件过滤器。

#### 2. **发件人身份混淆 (Sender Identity Obfuscation)**
- **多账户轮换与随机化**: 您可以在 `configs/email.yaml` 中配置多个发件邮箱账户。BypassMail 支持多种发送策略，如“轮询”（round-robin）和“随机”（random）。程序会根据策略自动切换发件人，将邮件流量分散到不同的身份上，避免单一发件人因发送频率过高而被列入黑名单或触发速率限制。
- **发件人别名**: 每个账户都可以设置一个 `from_alias`（发件人别名），使得邮件在收件箱中显示的名称更具迷惑性。

#### 3. **模拟人类行为 (Human Behavior Simulation)**
- **随机化发送延迟**: 为了对抗基于行为分析的检测引擎，BypassMail 可以在两次邮件发送之间插入一个随机的等待时间。您可以在 `configs/config.yaml` 中为每个策略设置 `min_delay` 和 `max_delay`。这种机制打破了机器自动化脚本固有的固定发送频率，使其行为模式更接近于人类。

#### 4. **深度个性化 (Deep Personalization)**
- **可编程的邮件模板**: 除了 AI 生成的正文，您还可以通过 CSV 文件为每位收件人注入高度个性化的字段，例如 `Name`, `Title`, `URL`, `File`, `Img` 等。一封带有真实姓名和相关链接的邮件，比通用邮件更容易通过启发式扫描。
- **定制化 Prompt**: CSV 文件中甚至可以包含 `CustomPrompt` 列，允许您为特定的、高价值的目标动态改变 AI 生成内容的核心方向，实现“千人千面”的精准打击。

#### 5. **结构化规避 (Structural Evasion)**
- **多模板支持**: 您可以创建多个结构完全不同的 HTML 模板（例如 `formal_template.html`, `casual_template.html`），并在运行时通过 `-template` 参数指定使用哪一个。定期更换邮件的 HTML 结构和 CSS 样式，可以绕过基于结构指纹的过滤器。

## 适用场景

* **钓鱼演练与攻击模拟**: 在企业内部进行高度仿真的钓鱼攻击演练，评估员工的安全意识和现有安全产品的防护能力。
* **红队渗透测试**: 作为社会工程学攻击的一环，向目标发送嵌入链接或附件的邮件，以获取初始访问权限。
* **邮件安全产品评估**: 测试和验证您的邮件安全网关（ESG）或终端防护（EDR）产品在面对高级规避技术时的真实表现。
* **垃圾邮件过滤机制研究**: 深入探究不同邮件服务商（如 Gmail, Office 365）的垃圾邮件过滤算法和检测逻辑。

## 快速上手
### 命令行参数

以下是 `bypass-mail` 支持的所有命令行参数，定义于 `cmd/bypass-mail/main.go` 中：

| 参数 | 说明 | 默认值 |
| --- | --- | --- |
| `-version` | 显示工具的版本号并退出。 | `false` |
| `-subject` | 邮件主题 (可被 CSV 中的 `subject` 列覆盖)。 | `""` |
| `-prompt` | 自定义邮件核心思想 (与 `-prompt-name` 二选一)。 | `""` |
| `-prompt-name` | 使用 `ai.yaml` 中预设的提示词名称 (与 `-prompt` 二选一)。 | `""` |
| `-instructions` | 要组合的结构化指令名称, 逗号分隔 (来自 `ai.yaml`)。 | `format_json_array` |
| `-recipients` | 收件人列表, 逗号分隔 (例如: `a@b.com,c@d.com`)。 | `""` |
| `-recipients-file` | 从文本或 CSV 文件读取收件人及个人化数据。 | `""` |
| `-template` | 邮件模板名称 (来自 `config.yaml`)。 | `default` |
| `-title` | 默认邮件内页标题 (若 CSV 未提供)。 | `""` |
| `-name` | 默认收件人称呼 (若 CSV 未提供)。 | `""` |
| `-url` | 默认附加链接 (若 CSV 未提供)。 | `""` |
| `-file` | 默认附加文件路径 (若 CSV 未提供)。 | `""` |
| `-img` | 默认邮件头图路径 (本地文件, 若 CSV 未提供)。 | `""` |
| `-strategy` | 指定使用的发件策略 (来自 `config.yaml`)。 | `default` |
| `-config` | 主策略配置文件路径。 | `configs/config.yaml` |
| `-ai-config` | AI 配置文件路径。 | `configs/ai.yaml` |
| `-email-config` | Email 配置文件路径。 | `configs/email.yaml` |
| `-test-accounts` | 仅测试发件策略中的账户是否可用，不发送邮件。 | `false` |

### 1. 配置

1.  **`configs/ai.yaml`**: 配置您选择的 AI 模型的提供商和 API Key。
2.  **`configs/email.yaml`**: 配置所有用于发送邮件的 SMTP 账户信息，包括密码和别名。
3.  **`configs/config.yaml`**: 定义发送策略，将不同的 SMTP 账户组合起来，并设置发送延迟。

### 2. 账号存活测试

在进行大规模发送前，先验证您的发件箱凭据是否有效。此模式不会发送任何邮件。
```bash
bypass-mail -test-accounts -strategy="round_robin_gmail"
```

### 3.执行发送任务
#### 示例1：批量发送

此命令将从 `recipients.csv` 读取收件人列表，使用 `weekly_report` 作为 AI 的核心思想，采用 `round_robin_gmail` 策略进行发送。

```bash
bypass-mail \
    -subject="项目季度回顾与展望" \
    -recipients-file="recipients.csv" \
    -prompt-name="weekly_report" \
    -strategy="round_robin_gmail"
```

#### 示例2：单次精准发送
此命令向单个目标发送一封邮件，内容由 `-prompt` 参数临时指定。

```bash
bypass-mail \
    -subject="一个临时的紧急通知" \
    -recipients="boss@company.com" \
    -prompt="今晚服务器需要紧急维护，预计从晚上10点到11点服务不可用，请周知。" \
    -strategy="default"
```

## 免责声明
此工具仅供授权的、合法的安全测试和教育研究目的使用。严禁将此工具用于任何未经授权的、非法的活动。工具的开发者对因使用此工具而导致的任何直接或间接的后果概不负责。您必须对自己的所有行为承担全部责任。