# SkillHub

SkillHub 是一个基于 Go 的 Skill 管理与分发平台，提供 Web 界面和 REST API。已集成 **1571+ ClawHub 真实技能数据**，支持代理加速下载，绕过 ClawHub 速率限制。

## 功能特性

- **真实数据**: 1571+ ClawHub 技能，含名称、描述、分类、下载量
- **TOP50 排行**: 精选热门技能展示
- **代理加速下载**: 首次从 ClawHub 代理并缓存，后续直接本地提供
- **速率限制处理**: 自动检测 ClawHub 速率限制，提供替代下载方案
- **分类浏览**: 10+ 技能分类（AI智能、开发工具、浏览器自动化等）
- **技能搜索**: 支持名称和描述搜索
- **技能上传**: 支持本地技能包上传
- **SQLite 持久化**: 轻量级数据存储

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
│   └── templates/index.html
├── internal/
│   ├── config/             # 配置加载
│   ├── handlers/           # HTTP 处理器
│   ├── middleware/         # 中间件
│   ├── models/             # 数据模型
│   ├── repository/         # 数据访问层
│   ├── service/            # 业务逻辑层
│   ├── utils/              # 工具与校验
│   └── data/               # 技能数据 (skills.json)
├── uploads/                # 上传文件 & 缓存目录
├── scripts/                # 工具脚本
├── config.yaml             # 运行配置
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

- Web UI: `http://localhost:8081/`
- 健康检查: `http://localhost:8081/api/health`

## 代理加速下载

### 工作原理

1. **首次下载**: 从 ClawHub 代理下载并缓存到本地 `uploads/` 目录
2. **后续下载**: 直接从本地缓存提供，不受 ClawHub 速率限制

### 使用方式

点击技能卡片的"直接下载"按钮：
- 如果已缓存：立即下载
- 如果未缓存：自动从 ClawHub 代理
- 如果遇到速率限制：显示替代方案（ClawHub 命令行 / 直接链接）

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

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/top50` | TOP50 列表 |
| GET | `/skills` | 分页查询技能 |
| GET | `/skills/:id` | 技能详情 |
| GET | `/skills/:id/download` | 下载技能包（代理+缓存） |
| GET | `/categories` | 分类统计 |
| GET | `/stats` | 全局统计 |
| POST | `/skills/upload` | 上传技能包 |
| PUT | `/skills/:id` | 更新技能 |
| DELETE | `/skills/:id` | 删除技能 |

### 下载 API 响应

**成功 (已缓存)**:
```
HTTP 200 - 返回文件流
```

**成功 (首次代理)**:
```
HTTP 200 - 返回文件流，文件已缓存
```

**速率限制 (503)**:
```json
{
  "code": 503,
  "message": "ClawHub rate limit reached, please use direct link",
  "direct_url": "https://clawhub.ai/api/v1/download?slug=xxx",
  "install_cmd": "npx clawhub@latest install xxx",
  "skill_name": "Skill Name"
}
```

## 构建

```bash
go build -o bin/server.exe ./cmd/server/main.go
```

运行：

```bash
./bin/server.exe
```

## 数据来源

技能数据来自 [ClawHub](https://clawhub.ai)，通过 API 抓取并存储在 `internal/data/skills.json`。

## License

MIT
