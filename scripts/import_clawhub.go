package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Skill struct {
	gorm.Model
	Name        string `gorm:"size:255;not null"`
	Icon        string `gorm:"size:500"`
	Category    string `gorm:"size:100;not null"`
	Description string `gorm:"size:1000"`
	Downloads   int
	Rating      int
	Verified    bool
	Accelerated bool
	Safe        bool
	FileName    string
	Slug        string `gorm:"size:255;uniqueIndex"`
}

type ClawHubSearchResult struct {
	Results []struct {
		Slug        string  `json:"slug"`
		DisplayName string  `json:"displayName"`
		Summary     string  `json:"summary"`
		Score       float64 `json:"score"`
	} `json:"results"`
}

var searchQueries = []string{
	// AI 核心
	"agent", "claude", "code", "mcp", "ai", "automation", "llm", "gpt",
	"openai", "anthropic", "chat", "assistant", "bot", "rag", "embedding",
	"prompt", "model", "neural", "machine", "learning", "nlp", "ml",
	"conversation", "dialogue", "reasoning", "inference", "fine-tune",
	"training", "dataset", "fine-tuning", "context", "token", "completion",

	// 开发工具
	"git", "github", "gitlab", "test", "api", "debug", "build", "deploy",
	"mcp", "cli", "terminal", "shell", "command", "script", "package",
	"npm", "pip", "cargo", "go", "rust", "python", "node", "typescript",
	"javascript", "java", "kotlin", "swift", "cpp", "react", "vue", "angular",
	"docker", "kubernetes", "k8s", "container", "server", "client", "sdk",
	"compiler", "interpreter", "runtime", "framework", "library", "module",
	"ide", "editor", "vscode", "vim", "emacs", "debugger", "profiler",

	// 数据管理
	"data", "database", "memory", "file", "json", "yaml", "csv", "xml",
	"sql", "postgres", "mysql", "mongodb", "redis", "sqlite", "storage",
	"cache", "queue", "stream", "pipeline", "etl", "backup", "sync",
	"migration", "schema", "table", "column", "row", "record", "query",
	"index", "transaction", "replication", "sharding", "partition",

	// 浏览器自动化
	"web", "browser", "scrape", "crawl", "http", "rest", "html", "dom",
	"puppeteer", "playwright", "selenium", "chrome", "firefox", "safari",
	"page", "site", "url", "link", "request", "response", "fetch", "download",
	"cookie", "session", "header", "body", "form", "submit", "click",
	"navigate", "scroll", "screenshot", "capture", "render", "ajax",

	// 文档处理
	"document", "markdown", "pdf", "text", "write", "read", "edit", "format",
	"word", "excel", "ppt", "slide", "report", "log", "note", "wiki",
	"documentation", "article", "blog", "content", "page", "book",
	"table", "chart", "graph", "diagram", "figure", "caption", "heading",
	"paragraph", "sentence", "word", "character", "spell", "grammar",

	// 信息处理
	"search", "query", "find", "filter", "sort", "index", "analyze",
	"translate", "summarize", "extract", "transform", "process", "parse",
	"knowledge", "graph", "semantic", "vector", "embed",
	"classify", "categorize", "tag", "label", "cluster", "group",
	"compare", "diff", "merge", "split", "join", "aggregate",

	// 安全工具
	"security", "auth", "encrypt", "validate", "token", "key", "secret",
	"password", "login", "oauth", "jwt", "ssl", "tls", "certificate",
	"firewall", "scan", "vulnerability", "protect", "guard", "shield",
	"permission", "role", "access", "policy", "audit", "compliance",
	"encryption", "decryption", "signature", "verify", "identity",

	// 办公协同
	"email", "slack", "discord", "notification", "message", "meeting",
	"calendar", "schedule", "reminder", "todo", "kanban", "board", "card",
	"team", "collaboration", "share", "invite", "member", "workspace",
	"comment", "feedback", "review", "approve", "reject", "assign",
	"deadline", "priority", "status", "progress", "milestone",

	// 多媒体
	"image", "video", "audio", "media", "picture", "photo", "screenshot",
	"record", "stream", "broadcast", "voice", "speech", "music", "sound",
	"ffmpeg", "convert", "compress", "resize", "crop",
	"thumbnail", "preview", "gif", "animation", "filter", "effect",
	"subtitle", "caption", "transcript", "overlay", "watermark",

	// 工作流
	"workflow", "task", "project", "config", "pipeline", "job", "runner",
	"automation", "trigger", "hook", "event", "schedule", "cron", "timer",
	"batch", "parallel", "async", "queue", "worker",
	"orchestration", "choreography", "step", "stage", "phase", "condition",
	"retry", "timeout", "error", "exception", "fallback", "rollback",

	// 网络通信
	"network", "socket", "websocket", "tcp", "udp", "rpc", "grpc",
	"proxy", "tunnel", "vpn", "dns", "ip", "port", "connection",
	"mqtt", "amqp", "kafka", "rabbitmq", "pubsub", "subscribe",
	"bandwidth", "latency", "throughput", "packet", "protocol",
	"load-balancer", "gateway", "router", "switch", "firewall",

	// 云服务
	"cloud", "aws", "azure", "gcp", "s3", "lambda", "function", "serverless",
	"storage", "bucket", "cdn", "edge", "region", "zone", "instance",
	"ec2", "ecs", "eks", "rds", "dynamodb", "sqs", "sns",
	"iam", "vpc", "subnet", "security-group", "auto-scaling",

	// 监控日志
	"monitor", "log", "metric", "trace", "alert", "dashboard", "status",
	"health", "check", "probe", "watch", "observe", "telemetry", "apm",
	"prometheus", "grafana", "elk", "splunk", "datadog", "newrelic",
	"error-tracking", "performance", "availability", "uptime", "latency",

	// 开发辅助
	"scaffold", "template", "generator", "boilerplate", "starter",
	"mock", "stub", "fake", "fixture", "sample", "example", "demo",
	"linter", "formatter", "prettier", "eslint", "test", "spec",
	"coverage", "benchmark", "profiler", "debugger", "inspector",

	// 其他常用
	"math", "calc", "number", "date", "time", "uuid", "hash", "random",
	"string", "array", "object", "list", "map", "set", "tree",
	"currency", "money", "price", "payment", "invoice", "receipt",
	"location", "geo", "route", "distance", "travel",
	"weather", "forecast", "temperature", "climate",
	"game", "fun", "entertainment", "puzzle", "quiz",

	// 更多通用词
	"tool", "util", "helper", "manager", "handler", "provider", "service",
	"adapter", "wrapper", "client", "driver", "connector", "integration",
	"plugin", "extension", "addon", "module", "component", "feature",
	"input", "output", "source", "target", "origin", "destination",
	"user", "admin", "system", "application", "platform", "environment",
	"version", "release", "update", "upgrade", "install", "uninstall",
	"config", "setting", "option", "preference", "property", "attribute",
	"error", "warning", "info", "debug", "trace", "fatal", "panic",
	"start", "stop", "pause", "resume", "restart", "reset", "init",
	"create", "update", "delete", "read", "write", "copy", "move",

	// 单字母搜索
	"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m",
	"n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
}

var categoryMap = map[string]string{
	// AI智能
	"agent": "AI智能", "claude": "AI智能", "ai": "AI智能", "automation": "AI智能",
	"llm": "AI智能", "gpt": "AI智能", "openai": "AI智能", "anthropic": "AI智能",
	"chat": "AI智能", "assistant": "AI智能", "bot": "AI智能", "rag": "AI智能",
	"embedding": "AI智能", "prompt": "AI智能", "model": "AI智能", "neural": "AI智能",
	"machine": "AI智能", "learning": "AI智能", "nlp": "AI智能", "ml": "AI智能",

	// 开发工具
	"code": "开发工具", "git": "开发工具", "github": "开发工具", "gitlab": "开发工具",
	"test": "开发工具", "api": "开发工具", "debug": "开发工具", "build": "开发工具",
	"deploy": "开发工具", "mcp": "开发工具", "cli": "开发工具", "terminal": "开发工具",
	"shell": "开发工具", "command": "开发工具", "script": "开发工具", "package": "开发工具",
	"npm": "开发工具", "pip": "开发工具", "cargo": "开发工具", "go": "开发工具",
	"rust": "开发工具", "python": "开发工具", "node": "开发工具", "typescript": "开发工具",
	"javascript": "开发工具", "java": "开发工具", "kotlin": "开发工具", "swift": "开发工具",
	"cpp": "开发工具", "react": "开发工具", "vue": "开发工具", "angular": "开发工具",
	"docker": "开发工具", "kubernetes": "开发工具", "k8s": "开发工具", "container": "开发工具",
	"server": "开发工具", "client": "开发工具", "sdk": "开发工具", "config": "开发工具",
	"scaffold": "开发工具", "template": "开发工具", "generator": "开发工具", "boilerplate": "开发工具",
	"starter": "开发工具", "mock": "开发工具", "stub": "开发工具", "fake": "开发工具",
	"fixture": "开发工具", "sample": "开发工具", "example": "开发工具", "demo": "开发工具",
	"linter": "开发工具", "formatter": "开发工具", "prettier": "开发工具", "eslint": "开发工具",
	"spec": "开发工具",

	// 数据管理
	"data": "数据管理", "database": "数据管理", "memory": "数据管理", "file": "数据管理",
	"json": "数据管理", "yaml": "数据管理", "csv": "数据管理", "xml": "数据管理",
	"sql": "数据管理", "postgres": "数据管理", "mysql": "数据管理", "mongodb": "数据管理",
	"redis": "数据管理", "sqlite": "数据管理", "storage": "数据管理", "cache": "数据管理",
	"queue": "数据管理", "stream": "数据管理", "pipeline": "数据管理", "etl": "数据管理",
	"backup": "数据管理", "sync": "数据管理",

	// 浏览器自动化
	"web": "浏览器自动化", "browser": "浏览器自动化", "scrape": "浏览器自动化", "crawl": "浏览器自动化",
	"http": "浏览器自动化", "rest": "浏览器自动化", "html": "浏览器自动化", "dom": "浏览器自动化",
	"puppeteer": "浏览器自动化", "playwright": "浏览器自动化", "selenium": "浏览器自动化",
	"chrome": "浏览器自动化", "firefox": "浏览器自动化", "safari": "浏览器自动化",
	"page": "浏览器自动化", "site": "浏览器自动化", "url": "浏览器自动化", "link": "浏览器自动化",
	"request": "浏览器自动化", "response": "浏览器自动化", "fetch": "浏览器自动化", "download": "浏览器自动化",

	// 文档处理
	"document": "文档处理", "markdown": "文档处理", "pdf": "文档处理", "text": "文档处理",
	"write": "文档处理", "read": "文档处理", "edit": "文档处理", "format": "文档处理",
	"word": "文档处理", "excel": "文档处理", "ppt": "文档处理", "slide": "文档处理",
	"report": "文档处理", "note": "文档处理", "wiki": "文档处理",
	"documentation": "文档处理", "article": "文档处理", "blog": "文档处理",
	"content": "文档处理", "book": "文档处理",

	// 信息处理
	"search": "信息处理", "query": "信息处理", "find": "信息处理", "filter": "信息处理",
	"sort": "信息处理", "index": "信息处理", "analyze": "信息处理", "translate": "信息处理",
	"summarize": "信息处理", "extract": "信息处理", "transform": "信息处理",
	"process": "信息处理", "parse": "信息处理", "knowledge": "信息处理",
	"graph": "信息处理", "semantic": "信息处理", "vector": "信息处理", "embed": "信息处理",

	// 安全工具
	"security": "安全工具", "auth": "安全工具", "encrypt": "安全工具", "validate": "安全工具",
	"token": "安全工具", "key": "安全工具", "secret": "安全工具", "password": "安全工具",
	"login": "安全工具", "oauth": "安全工具", "jwt": "安全工具", "ssl": "安全工具",
	"tls": "安全工具", "certificate": "安全工具", "firewall": "安全工具",
	"scan": "安全工具", "vulnerability": "安全工具", "protect": "安全工具",
	"guard": "安全工具", "shield": "安全工具",

	// 办公协同
	"email": "办公协同", "slack": "办公协同", "discord": "办公协同", "notification": "办公协同",
	"message": "办公协同", "meeting": "办公协同", "calendar": "办公协同",
	"schedule": "办公协同", "reminder": "办公协同", "todo": "办公协同", "kanban": "办公协同",
	"board": "办公协同", "card": "办公协同", "team": "办公协同",
	"collaboration": "办公协同", "share": "办公协同", "invite": "办公协同",
	"member": "办公协同", "workspace": "办公协同", "workflow": "办公协同",
	"task": "办公协同", "project": "办公协同",

	// 多媒体
	"image": "多媒体", "video": "多媒体", "audio": "多媒体", "media": "多媒体",
	"picture": "多媒体", "photo": "多媒体", "screenshot": "多媒体", "record": "多媒体",
	"broadcast": "多媒体", "voice": "多媒体", "speech": "多媒体",
	"music": "多媒体", "sound": "多媒体", "ffmpeg": "多媒体", "convert": "多媒体",
	"compress": "多媒体", "resize": "多媒体", "crop": "多媒体",

	// 其他
	"tool": "其他", "util": "其他", "helper": "其他",
	"job": "其他", "runner": "其他", "trigger": "其他", "hook": "其他",
	"event": "其他", "cron": "其他", "timer": "其他", "batch": "其他",
	"parallel": "其他", "async": "其他", "worker": "其他",

	// 网络通信
	"network": "网络通信", "socket": "网络通信", "websocket": "网络通信",
	"tcp": "网络通信", "udp": "网络通信", "rpc": "网络通信", "grpc": "网络通信",
	"proxy": "网络通信", "tunnel": "网络通信", "vpn": "网络通信",
	"dns": "网络通信", "ip": "网络通信", "port": "网络通信", "connection": "网络通信",
	"mqtt": "网络通信", "amqp": "网络通信", "kafka": "网络通信",
	"rabbitmq": "网络通信", "pubsub": "网络通信", "subscribe": "网络通信",

	// 云服务
	"cloud": "云服务", "aws": "云服务", "azure": "云服务", "gcp": "云服务",
	"s3": "云服务", "lambda": "云服务", "function": "云服务", "serverless": "云服务",
	"bucket": "云服务", "cdn": "云服务", "edge": "云服务",
	"region": "云服务", "zone": "云服务", "instance": "云服务",

	// 监控日志
	"monitor": "监控日志", "log": "监控日志", "metric": "监控日志",
	"trace": "监控日志", "alert": "监控日志", "dashboard": "监控日志", "status": "监控日志",
	"health": "监控日志", "check": "监控日志", "probe": "监控日志",
	"watch": "监控日志", "observe": "监控日志", "telemetry": "监控日志", "apm": "监控日志",

	// 数学工具
	"math": "其他", "calc": "其他", "number": "其他",
	"date": "其他", "time": "其他", "uuid": "其他", "hash": "其他", "random": "其他",
	"string": "其他", "array": "其他", "object": "其他", "list": "其他",
	"map": "其他", "set": "其他", "tree": "其他",

	// 商务
	"currency": "其他", "money": "其他", "price": "其他",
	"payment": "其他", "invoice": "其他", "receipt": "其他",

	// 位置
	"location": "其他", "geo": "其他",
	"route": "其他", "distance": "其他", "travel": "其他",

	// 天气
	"weather": "其他", "forecast": "其他", "temperature": "其他", "climate": "其他",

	// 娱乐
	"game": "其他", "fun": "其他", "entertainment": "其他", "puzzle": "其他", "quiz": "其他",
}

func main() {
	db, err := gorm.Open(sqlite.Open("./skills.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&Skill{})

	client := &http.Client{Timeout: 30 * time.Second}
	skillsMap := make(map[string]Skill)

	fmt.Println("开始从 ClawHub 爬取 Skills 数据...")
	fmt.Printf("共 %d 个搜索关键词\n", len(searchQueries))

	for i, query := range searchQueries {
		fmt.Printf("[%d/%d] 正在搜索: %s ... ", i+1, len(searchQueries), query)

		category := categoryMap[query]
		if category == "" {
			category = "其他"
		}

		// 每个关键词搜索多页，每页 200 条
		totalNew := 0
		for offset := 0; offset < 400; offset += 200 {
			url := fmt.Sprintf("https://clawhub.ai/api/v1/search?q=%s&limit=200&offset=%d", query, offset)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("Accept", "application/json")
			req.Header.Set("User-Agent", "SkillHub/3.0")

			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("请求失败: %v\n", err)
				break
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var result ClawHubSearchResult
			if err := json.Unmarshal(body, &result); err != nil {
				fmt.Printf("解析失败: %v\n", err)
				break
			}

			if len(result.Results) == 0 {
				break // 没有更多数据
			}

			newCount := 0
			for _, item := range result.Results {
				if _, exists := skillsMap[item.Slug]; !exists {
					icon := getIcon(item.DisplayName)
					rating := int(4.0 + (item.Score-3.0)/2*1.0)
					if rating > 5 {
						rating = 5
					}
					if rating < 3 {
						rating = 3
					}
					skillsMap[item.Slug] = Skill{
						Name:        item.DisplayName,
						Icon:        icon,
						Category:    category,
						Description: truncate(item.Summary, 200),
						Downloads:   int(item.Score * 10000),
						Rating:      rating,
						Verified:    item.Score > 3.3,
						Accelerated: true,
						Safe:        true,
						Slug:        item.Slug,
					}
					newCount++
				}
			}
			totalNew += newCount

			// 如果返回结果少于 200，说明没有更多数据
			if len(result.Results) < 200 {
				break
			}

			// 避免请求过快
			time.Sleep(200 * time.Millisecond)
		}

		fmt.Printf("新增 %d 条 (总计: %d)\n", totalNew, len(skillsMap))
	}

	fmt.Printf("\n爬取完成，共获取 %d 个 Skills\n", len(skillsMap))

	// 转换为切片
	skills := make([]Skill, 0, len(skillsMap))
	for _, skill := range skillsMap {
		skills = append(skills, skill)
	}

	// 按下载量排序
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Downloads > skills[j].Downloads
	})

	// 限制评分范围
	for i := range skills {
		if skills[i].Rating > 5 {
			skills[i].Rating = 5
		}
		if skills[i].Rating < 3 {
			skills[i].Rating = 3
		}
	}

	// 清空现有数据
	fmt.Println("清空现有数据...")
	db.Exec("DELETE FROM skills")

	// 批量插入
	fmt.Println("插入新数据...")
	batchSize := 100
	for i := 0; i < len(skills); i += batchSize {
		end := i + batchSize
		if end > len(skills) {
			end = len(skills)
		}
		batch := skills[i:end]
		if err := db.Create(&batch).Error; err != nil {
			fmt.Printf("插入批次 %d-%d 失败: %v\n", i, end, err)
		}
	}

	fmt.Printf("\n✅ 成功导入 %d 个 Skills!\n", len(skills))
}

func getIcon(name string) string {
	icons := []string{"🤖", "⚡", "🚀", "💡", "🔧", "📦", "🎯", "💻", "🔥", "⭐", "🌟", "💎", "🎨", "📊", "🔐", "📱", "🌐", "🔍", "📝", "🛠️"}
	if len(name) == 0 {
		return icons[0]
	}
	hash := int(name[0])
	return icons[hash%len(icons)]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
