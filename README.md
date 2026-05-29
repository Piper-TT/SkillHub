# SkillHub

基于 Go 的 Skill 与 MCP Server 管理与分发平台，集成 **12000+ ClawHub 技能** 和 **3400+ MCP 服务器**，支持本地上传和团队共享。

## 页面

| 页面 | 路径 | 说明 |
|------|------|------|
| Portal | `/` | 统一入口 |
| SkillHub | `/skills` | 技能浏览与管理 |
| ServerHub | `/server` | MCP 服务器浏览与管理 |
| AgentHub | `/agent` | 智能体聊天平台 |
| Analysis | `/analysis` | 恶意文件分析 |
| Crash Dump | `/crash-dump` | 崩溃转储分析 |
| Kernel | `/kernel` | 内核适配服务（反向代理） |
| TI | `/ti` | 威胁情报查询（反向代理） |
| TinyClaw | `/tinyclaw` | TinyClaw 安装引导 |

Portal、SkillHub、ServerHub、AgentHub 为独立页面（完整导航栏）。Analysis、Crash Dump、Kernel、TI、Chat 为 AgentHub 子页面。TinyClaw 使用 Go 模板渲染。

## 功能

- **技能管理** — 12000+ ClawHub 技能数据，支持本地上传、分类浏览、搜索
- **服务器管理** — 3400+ MCP 服务器，配置一键复制
- **智能体聊天** — 多 LLM Provider（Anthropic/OpenAI/DeepSeek/GLM），SSE 流式响应，支持创建/编辑/删除自定义智能体
- **漏洞补丁查询** — LLM Tool Call 驱动，通过 REST API 查询漏洞数据库
- **恶意文件分析** — IDA-Pro-MCP + LLM Agent 自动化二进制分析，PDF 报告导出
- **崩溃转储分析** — Windows/Linux crash dump 分析
- **TinyClaw** — 轻量级主机管控代理安装引导，支持 x86_64/aarch64/mips64el/loongarch64

## 快速开始

```bash
go mod tidy
go run ./cmd/server/main.go          # 开发模式
go build -a -o bin/server.exe ./cmd/server/main.go  # 构建（模板变更需 -a）
```

默认端口 `18089`，通过 `config.yaml` 配置。

## 架构

分层架构：**Handler → Service → Repository → Database**

```
cmd/server/main.go           # 入口：配置、DI、Gin 路由
internal/
├── config/                  # Viper 配置加载
├── handlers/                # HTTP 处理器（Gin）
├── service/                 # 业务逻辑
├── repository/              # 数据访问（GORM）
├── models/                  # 数据模型与 DTO
├── middleware/               # 日志、安全、恢复、大小限制
└── utils/                   # 校验、响应、加密、PDF
cmd/server/templates/        # 前端页面（纯 HTML/CSS/JS，go:embed 嵌入）
scripts/                     # 数据导入与工具脚本
```

### 核心组件

- **LLM Service** — 统一 OpenAI 兼容客户端，支持 5 个 Provider
- **ToolExecutor 接口** — LLM Tool Call 抽象，实现：VulnAPIClient（漏洞查询）、MCPToolAdapter（恶意文件分析）
- **LLM Tool Call 循环** — 多轮工具执行：LLM 返回 tool_call → 执行 → 结果回传 → 重复直到最终回答
- **VulnAPIClient** — 漏洞查询 REST API 客户端，替代本地 SQLite 直查
- **API Key 加密** — AES-GCM 加密存储

## 配置

`config.yaml`：

```yaml
server:
  port: 18089
  upload_dir: ./uploads
  max_upload_size: 104857600   # 100MB

analysis:
  idalib_path: "path/to/idalib-mcp.exe"

kernel:
  service_url: "http://localhost:8081"

ti:
  service_url: "http://localhost:8080"

vuln:
  api_base: "http://localhost:8903/api/v1"  # 漏洞查询服务
  token: "your-token"

tinyclaw:
  tinyclaw_linux_url: "curl -fsSL http://10.50.6.49/edr/tinyclaw/install.sh | sudo bash"
  tinyclaw_linux_wget_url: "wget -qO- http://10.50.6.49/edr/tinyclaw/install.sh | sudo bash"
```

## API 路由

所有接口在 `/api` 下。

| 路由组 | 路径 | 说明 |
|--------|------|------|
| Skills | `/api/skills/*` | 技能 CRUD + 上传下载 |
| Servers | `/api/server/*` | 服务器 CRUD + 上传下载 |
| Agents | `/api/agent/*` | 智能体 CRUD、聊天（SSE）、会话 |
| API Key | `/api/user/apikey/*` | 加密 API Key 管理 |
| Analysis | `/api/analysis/*` | 恶意文件分析 |
| Crash Dump | `/api/crash-dump/*` | 崩溃转储分析 |
| Vuln | `/api/vuln/*` | 漏洞数据库状态 + 导入 |
| IDA | `/api/ida/servers/*` | IDA 服务器注册 + 心跳 |
| Kernel | `/api/kernel/*` | 反向代理 |
| TI | `/api/ti/*` | 反向代理 |

## 技术栈

Go 1.23 · Gin · GORM + SQLite (pure-Go) · Viper · gofpdf · go-openai

## License

MIT
