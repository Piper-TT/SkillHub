# SkillHub

SkillHub 是一个基于 Go 的 Skill 管理与分发平台，提供 Web 界面和 REST API，支持技能包上传、查询、下载、统计与基础管理能力。

## 功能特性

- Skill 列表分页查询、TOP50 排行、分类统计
- Skill 包上传（带文件与参数校验）
- Skill 包下载（自动累计下载量）
- Skill 信息更新与删除
- SQLite 持久化存储
- Web 首页展示与交互

## 技术栈

- Go 1.21
- Gin
- GORM + SQLite（`github.com/glebarez/sqlite`）
- Viper（配置管理）
- Zap（日志）

## 目录结构

```text
.
├── cmd/server/             # 服务入口、页面模板与静态资源
│   ├── main.go
│   ├── templates/index.html
│   └── static/
├── internal/
│   ├── config/             # 配置加载
│   ├── handlers/           # HTTP 处理器
│   ├── middleware/         # 中间件
│   ├── models/             # 数据模型
│   ├── repository/         # 数据访问层
│   ├── service/            # 业务逻辑层
│   └── utils/              # 工具与校验
├── uploads/                # 上传文件目录
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

- `GET /health`：健康检查
- `GET /top50`：TOP50 列表
- `GET /skills`：分页查询技能
- `GET /categories`：分类统计
- `GET /skills/:id`：技能详情
- `GET /stats`：全局统计
- `POST /skills/upload`：上传技能包
- `GET /skills/:id/download`：下载技能包
- `PUT /skills/:id`：更新技能
- `DELETE /skills/:id`：删除技能
- `POST /init`：初始化上传目录示例文件（开发用途）

## 上传规则

- 必填字段：`name`、`category`、`file`
- 允许扩展名：`.zip`、`.tar.gz`、`.tgz`
- 默认大小限制：100MB
- 进行文件名合法性、路径遍历与内容类型校验

## 构建

```bash
build.bat
```

或手动构建：

```bash
go build -o bin/server.exe ./cmd/server/main.go
```

运行：

```bash
./bin/server.exe
```
