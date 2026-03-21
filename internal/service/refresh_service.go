package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"skillhub/internal/models"
	"skillhub/internal/repository"

	"go.uber.org/zap"
)

// RefreshService 数据刷新服务
type RefreshService struct {
	repo   repository.SkillRepository
	client *http.Client
	log    *zap.SugaredLogger
	mu     sync.Mutex
}

// ClawHubSearchResult ClawHub 搜索结果
type ClawHubSearchResult struct {
	Results []struct {
		Slug        string  `json:"slug"`
		DisplayName string  `json:"displayName"`
		Summary     string  `json:"summary"`
		Score       float64 `json:"score"`
	} `json:"results"`
}

// 搜索关键词
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

// 分类映射
var categoryMap = map[string]string{
	"agent": "AI智能", "claude": "AI智能", "ai": "AI智能", "automation": "AI智能",
	"llm": "AI智能", "gpt": "AI智能", "openai": "AI智能", "anthropic": "AI智能",
	"chat": "AI智能", "assistant": "AI智能", "bot": "AI智能", "rag": "AI智能",
	"embedding": "AI智能", "prompt": "AI智能", "model": "AI智能", "neural": "AI智能",
	"machine": "AI智能", "learning": "AI智能", "nlp": "AI智能", "ml": "AI智能",

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

	"data": "数据管理", "database": "数据管理", "memory": "数据管理", "file": "数据管理",
	"json": "数据管理", "yaml": "数据管理", "csv": "数据管理", "xml": "数据管理",
	"sql": "数据管理", "postgres": "数据管理", "mysql": "数据管理", "mongodb": "数据管理",
	"redis": "数据管理", "sqlite": "数据管理", "storage": "数据管理", "cache": "数据管理",

	"web": "浏览器自动化", "browser": "浏览器自动化", "scrape": "浏览器自动化", "crawl": "浏览器自动化",
	"http": "浏览器自动化", "rest": "浏览器自动化", "html": "浏览器自动化", "dom": "浏览器自动化",
	"puppeteer": "浏览器自动化", "playwright": "浏览器自动化", "selenium": "浏览器自动化",

	"document": "文档处理", "markdown": "文档处理", "pdf": "文档处理", "text": "文档处理",
	"write": "文档处理", "read": "文档处理", "edit": "文档处理", "format": "文档处理",
	"word": "文档处理", "excel": "文档处理", "ppt": "文档处理", "note": "文档处理",

	"search": "信息处理", "query": "信息处理", "find": "信息处理", "filter": "信息处理",
	"translate": "信息处理", "summarize": "信息处理", "extract": "信息处理", "analyze": "信息处理",

	"security": "安全工具", "auth": "安全工具", "encrypt": "安全工具", "token": "安全工具",
	"password": "安全工具", "oauth": "安全工具", "jwt": "安全工具",

	"email": "办公协同", "slack": "办公协同", "discord": "办公协同", "notification": "办公协同",
	"calendar": "办公协同", "meeting": "办公协同", "task": "办公协同", "project": "办公协同",
	"workflow": "办公协同",

	"image": "多媒体", "video": "多媒体", "audio": "多媒体", "media": "多媒体",
	"ffmpeg": "多媒体", "convert": "多媒体",

	"network": "网络通信", "socket": "网络通信", "websocket": "网络通信",
	"proxy": "网络通信",

	"cloud": "云服务", "aws": "云服务", "azure": "云服务", "gcp": "云服务",
	"s3": "云服务", "lambda": "云服务", "serverless": "云服务",

	"monitor": "监控日志", "log": "监控日志", "metric": "监控日志", "alert": "监控日志",
}

// NewRefreshService 创建刷新服务
func NewRefreshService(repo repository.SkillRepository) *RefreshService {
	return &RefreshService{
		repo: repo,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: zap.NewNop().Sugar(),
	}
}

// SetLogger 设置日志
func (s *RefreshService) SetLogger(log *zap.SugaredLogger) {
	s.log = log
}

// StartBackgroundRefresh 启动后台定时刷新
func (s *RefreshService) StartBackgroundRefresh(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		s.log.Infof("数据刷新服务已启动，间隔: %v", interval)

		for {
			select {
			case <-ctx.Done():
				s.log.Info("数据刷新服务已停止")
				return
			case <-ticker.C:
				s.log.Info("开始定时刷新数据...")
				if err := s.RefreshData(ctx); err != nil {
					s.log.Errorf("刷新数据失败: %v", err)
				} else {
					s.log.Info("数据刷新完成")
				}
			}
		}
	}()
}

// RefreshData 刷新数据
func (s *RefreshService) RefreshData(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	skillsMap := make(map[string]models.Skill)

	s.log.Infof("开始从 ClawHub 爬取数据，共 %d 个关键词", len(searchQueries))

	for i, query := range searchQueries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		category := categoryMap[query]
		if category == "" {
			category = "其他"
		}

		// 每个关键词搜索多页
		for offset := 0; offset < 400; offset += 200 {
			url := fmt.Sprintf("https://clawhub.ai/api/v1/search?q=%s&limit=200&offset=%d", query, offset)
			req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
			req.Header.Set("Accept", "application/json")
			req.Header.Set("User-Agent", "SkillHub/4.0")

			resp, err := s.client.Do(req)
			if err != nil {
				break
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var result ClawHubSearchResult
			if err := json.Unmarshal(body, &result); err != nil {
				break
			}

			if len(result.Results) == 0 {
				break
			}

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
					skillsMap[item.Slug] = models.Skill{
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
				}
			}

			if len(result.Results) < 200 {
				break
			}

			time.Sleep(150 * time.Millisecond)
		}

		if (i+1)%50 == 0 {
			s.log.Infof("进度: %d/%d, 已获取: %d", i+1, len(searchQueries), len(skillsMap))
		}
	}

	s.log.Infof("爬取完成，共获取 %d 个 Skills", len(skillsMap))

	// 转换为切片并排序
	skills := make([]models.Skill, 0, len(skillsMap))
	for _, skill := range skillsMap {
		skills = append(skills, skill)
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Downloads > skills[j].Downloads
	})

	// 保存到数据库
	if err := s.repo.ReplaceAll(ctx, skills); err != nil {
		return fmt.Errorf("保存数据失败: %w", err)
	}

	s.log.Infof("成功导入 %d 个 Skills", len(skills))
	return nil
}

func getIcon(name string) string {
	icons := []string{"🤖", "⚡", "🚀", "💡", "🔧", "📦", "🎯", "💻", "🔥", "⭐", "🌟", "💎", "🎨", "📊", "🔐", "📱", "🌐", "🔍", "📝", "🛠️"}
	if len(name) == 0 {
		return icons[0]
	}
	hash := int(name[0])
	return icons[hash%len(icons)]
}

func truncate(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen] + "..."
}
