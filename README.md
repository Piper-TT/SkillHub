# SkillHub

SkillHub 是一个基于 Go 的 Skill 与 MCP 服务器管理与分发平台，提供 Web 界面和 REST API。已集成 **12000+ ClawHub 技能** 和 **3400+ MCP 服务器**，支持本地上传和团队共享。

## 功能特性

- **ClawHub 技能**: 12000+ 技能数据，含名称、描述、分类
- **MCP 服务器**: 3400+ MCP 服务器，支持配置一键复制
- **TOP50 排行**: 精选热门技能/服务器展示
- **本地上传**: 支持上传本地技能包和 MCP 服务器，自定义分类
- **团队共享**: 部署到内网服务器，团队共享技能资源
- **安装提示**: 一键复制安装提示，发送给 AI 助手安装技能
- **分类浏览**: 10+ 技能分类（AI智能、开发工具、浏览器自动化等）
- **技能搜索**: 支持名称和描述搜索
- **SQLite 持久化**: 轻量级数据存储
- **Portal 入口**: 统一入口页面，24 小时定时刷新

## 技术栈

- Go 1.21+
- Gin (HTTP 框架)
- GORM + SQLite (`github.com/glebarez/sqlite`)
- Viper (配置管理)
- Zap (日志)

## 目录结构

```text
.
├── cmd/server/             # 服务入口、页面模板与静态资源
│   ├── main.go
│   └── templates/
│       ├── portal.html     # Portal 入口页面
│       ├── index.html      # SkillHub 页面
│       └── mcp.html        # MCPHub 页面
├── internal/
│   ├── config/             # 配置加载
│   ├── handlers/           # HTTP 处理器
│   ├── middleware/         # 中间件
│   ├── models/             # 数据模型
│   ├── repository/         # 数据访问层
│   ├── service/            # 业务逻辑层
│   ├── utils/              # 工具与校验
│   └── data/               # 技能数据
├── uploads/                # 上传文件目录
├── scripts/                # 工具脚本
│   ├── import_clawhub.go       # ClawHub 数据爬取脚本
│   ├── import_mcp.go           # MCP 服务器爬取脚本 (mcp.so API)
│   └── parse_mcp_readme.go     # MCP 服务器解析脚本 (GitHub README)
├── config.yaml             # 运行配置
├── skills.db               # SQLite 数据库
└── go.mod
```

## 快速开始

### 1. 环境要求

- Go >= 1.21

### 2. 安装依赖

```bash
go mod tidy
```

### 3. 启动服务

```bash
go run ./cmd/server/main.go
```

启动后默认访问：

- Portal 入口: `http://localhost:8081/`
- SkillHub: `http://localhost:8081/skills`
- MCPHub: `http://localhost:8081/mcp`
- 健康检查: `http://localhost:8081/api/health`

## 使用说明

### 上传技能

1. 访问 SkillHub 页面 `/skills` 上传区域
2. 填写技能名称、选择或输入自定义分类
3. 上传 .zip 技能包
4. 提交保存

### 上传 MCP 服务器

1. 访问 MCPHub 页面 `/mcp` 上传区域
2. 填写服务器名称、GitHub 地址、描述等
3. 上传 .zip 或 .tar.gz 服务器文件
4. 提交保存

### 安装技能

**本地上传的技能**:
- 点击技能详情，使用"本地下载"按钮直接下载

**ClawHub 技能**:
- 点击技能详情，复制安装提示
- 将提示发送给 AI 助手（Claude、ChatGPT、Cursor 等）自动安装

### 配置 MCP 服务器

1. 浏览 MCPHub 页面查找需要的服务器
2. 点击服务器卡片查看详情
3. 复制 GitHub 地址或下载本地文件
4. 按照服务器文档进行配置

## 配置说明

配置文件：`config.yaml`

```yaml
server:
  port: 8081
  upload_dir: ./uploads
  max_upload_size: 104857600

database:
  type: sqlite
  path: ./skills.db

security:
  allowed_extensions:
    - .zip
    - .tar.gz
    - .tgz
  max_filename_length: 255

logging:
  level: info
  format: console
```

## API 概览

基础路径：`/api`

### 技能 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/top50` | TOP50 列表 |
| GET | `/skills` | 分页查询技能 |
| GET | `/skills/:id` | 技能详情 |
| GET | `/skills/:id/download` | 下载技能包 |
| GET | `/categories` | 分类统计 |
| GET | `/stats` | 全局统计 |
| POST | `/skills/upload` | 上传技能包 |
| PUT | `/skills/:id` | 更新技能 |
| DELETE | `/skills/:id` | 删除技能 |

### MCP 服务器 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/mcp` | 分页查询 MCP 服务器 |
| GET | `/mcp/:id` | MCP 服务器详情 |
| GET | `/mcp/categories` | MCP 分类统计 |
| GET | `/mcp/stats` | MCP 统计 |
| GET | `/mcp/:id/download` | 下载 MCP 服务器文件 |
| POST | `/mcp/upload` | 上传 MCP 服务器文件 |

## 构建

```bash
# 构建（模板嵌入，修改模板后需 -a 强制重建）
go build -a -o bin/server.exe ./cmd/server/main.go
```

运行：

```bash
./bin/server.exe
```

## 数据来源

- **技能数据**: 来自 [ClawHub](https://clawhub.ai)，通过 `scripts/import_clawhub.go` 脚本从 API 爬取，已扩展关键词覆盖 12000+ 技能
- **MCP 服务器**: 来自 [mcp.so](https://mcp.so) API 和 [awesome-mcp-servers](https://github.com/punkpeye/awesome-mcp-servers)，通过 `scripts/import_mcp.go` 和 `scripts/parse_mcp_readme.go` 爬取

### 爬取数据

```bash
# 爬取 ClawHub 技能
go run ./scripts/import_clawhub.go

# 爬取 MCP 服务器 (mcp.so API)
go run ./scripts/import_mcp.go

# 解析 MCP 服务器 (GitHub README)
go run ./scripts/parse_mcp_readme.go
```

## License

MIT
