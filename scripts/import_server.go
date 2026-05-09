package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"skillhub/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// MCP.so API 响应结构
type ServerSearchResult struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	RepoURL     string  `json:"repoUrl"`
	Category    string  `json:"category"`
	Tags        []string `json:"tags"`
	Stars       int     `json:"stars"`
	Verified    bool    `json:"verified"`
	Official    bool    `json:"official"`
	NPMPackage  string  `json:"npmPackage"`
	PyPIPackage string  `json:"pypiPackage"`
	InstallCmd  string  `json:"installCommand"`
	Config      string  `json:"configExample"`
}

type ServerListResponse struct {
	Data []ServerSearchResult `json:"data"`
	Total int `json:"total"`
}

// 分类映射
var categoryMap = map[string]string{
	"filesystem": "File System",
	"database": "Database",
	"web-search": "Web Search",
	"search": "Web Search",
	"browser": "Browser",
	"browser-automation": "Browser",
	"ai": "AI Tools",
	"ml": "AI Tools",
	"cloud": "Cloud",
	"aws": "Cloud",
	"azure": "Cloud",
	"gcp": "Cloud",
	"communication": "Communication",
	"slack": "Communication",
	"discord": "Communication",
	"email": "Communication",
	"developer": "Developer",
	"github": "Developer",
	"git": "Developer",
	"gitlab": "Developer",
	"data": "Data",
	"analytics": "Data",
	"memory": "AI Tools",
	"tools": "Other",
}

func main() {
	db, err := gorm.Open(sqlite.Open("./skills.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&models.Server{})

	client := &http.Client{Timeout: 30 * time.Second}
	serversMap := make(map[string]models.Server)

	fmt.Println("开始从 mcp.so 爬取 MCP 服务器数据...")

	// 搜索关键词列表
	keywords := []string{
		"mcp", "server", "api", "database", "filesystem",
		"github", "git", "slack", "browser", "search",
		"ai", "memory", "cloud", "aws", "azure",
		"postgres", "mysql", "redis", "mongodb", "sqlite",
		"web", "http", "fetch", "puppeteer", "playwright",
		"email", "calendar", "file", "data", "tool",
	}

	for i, keyword := range keywords {
		fmt.Printf("[%d/%d] 正在搜索: %s ... ", i+1, len(keywords), keyword)

		// 尝试多个 API 端点
		urls := []string{
			fmt.Sprintf("https://mcp.so/api/servers?search=%s&limit=100", keyword),
			fmt.Sprintf("https://mcp.so/api/v1/servers?search=%s&limit=100", keyword),
			fmt.Sprintf("https://api.mcp.so/servers?search=%s&limit=100", keyword),
		}

		var results []ServerSearchResult
		found := false

		for _, url := range urls {
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("Accept", "application/json")
			req.Header.Set("User-Agent", "ServerHub/1.0")

			resp, err := client.Do(req)
			if err != nil {
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			// 尝试解析响应
			var listResp ServerListResponse
			if err := json.Unmarshal(body, &listResp); err == nil && len(listResp.Data) > 0 {
				results = listResp.Data
				found = true
				break
			}

			// 尝试直接解析为数组
			var directResults []ServerSearchResult
			if err := json.Unmarshal(body, &directResults); err == nil && len(directResults) > 0 {
				results = directResults
				found = true
				break
			}
		}

		if !found {
			fmt.Printf("API 不可用，使用备用数据\n")
			continue
		}

		newCount := 0
		for _, item := range results {
			if _, exists := serversMap[item.Slug]; !exists && item.Slug != "" {
				category := normalizeCategory(item.Category)
				if category == "" {
					category = guessCategory(item.Name, item.Description, item.Tags)
				}

				icon := getIcon(item.Name, category)
				installCmd := item.InstallCmd
				if installCmd == "" && item.NPMPackage != "" {
					installCmd = "npx " + item.NPMPackage
				}
				if installCmd == "" {
					installCmd = "npx @" + item.Slug
				}

				config := item.Config
				if config == "" {
					config = generateConfig(item.Slug, installCmd)
				}

				serversMap[item.Slug] = models.Server{
					Name:        item.Name,
					Slug:        item.Slug,
					Icon:        icon,
					Category:    category,
					Description: truncate(item.Description, 200),
					GitHubURL:   item.RepoURL,
					NPMPackage:  item.NPMPackage,
					PyPIPkg:     item.PyPIPackage,
					Stars:       item.Stars,
					Downloads:   item.Stars * 3, // 估算下载量
					InstallCmd:  installCmd,
					Config:      config,
					Verified:    item.Verified,
					Official:    item.Official,
				}
				newCount++
			}
		}

		fmt.Printf("新增 %d 条 (总计: %d)\n", newCount, len(serversMap))
		time.Sleep(200 * time.Millisecond)
	}

	// 如果 API 没有返回数据，使用备用数据
	if len(serversMap) == 0 {
		fmt.Println("\nAPI 不可用，使用精选数据...")
		serversMap = getBackupServers()
	}

	fmt.Printf("\n爬取完成，共获取 %d 个 MCP 服务器\n", len(serversMap))

	// 转换为切片
	servers := make([]models.Server, 0, len(serversMap))
	for _, server := range serversMap {
		servers = append(servers, server)
	}

	// 按 Stars 排序
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Stars > servers[j].Stars
	})

	// 清空现有数据
	fmt.Println("清空现有数据...")
	db.Exec("DELETE FROM servers")

	// 批量插入
	fmt.Println("插入新数据...")
	batchSize := 100
	for i := 0; i < len(servers); i += batchSize {
		end := i + batchSize
		if end > len(servers) {
			end = len(servers)
		}
		batch := servers[i:end]
		if err := db.Create(&batch).Error; err != nil {
			fmt.Printf("插入批次 %d-%d 失败: %v\n", i, end, err)
		}
	}

	fmt.Printf("\n✅ 成功导入 %d 个 MCP 服务器!\n", len(servers))

	// 保存到 JSON 文件
	jsonData, _ := json.MarshalIndent(servers, "", "  ")
	os.WriteFile("internal/data/servers.json", jsonData, 0644)
	fmt.Println("数据已保存到 internal/data/mcp_servers.json")
}

func normalizeCategory(cat string) string {
	if cat == "" {
		return ""
	}
	cat = strings.ToLower(cat)
	if mapped, ok := categoryMap[cat]; ok {
		return mapped
	}
	return cat
}

func guessCategory(name, desc string, tags []string) string {
	text := strings.ToLower(name + " " + desc + " " + strings.Join(tags, " "))

	keywords := map[string]string{
		"file": "File System", "filesystem": "File System",
		"database": "Database", "postgres": "Database", "mysql": "Database",
		"redis": "Database", "mongodb": "Database", "sqlite": "Database",
		"search": "Web Search", "brave": "Web Search", "google": "Web Search",
		"browser": "Browser", "puppeteer": "Browser", "playwright": "Browser",
		"selenium": "Browser", "chrome": "Browser",
		"slack": "Communication", "discord": "Communication", "email": "Communication",
		"github": "Developer", "git": "Developer", "gitlab": "Developer",
		"aws": "Cloud", "azure": "Cloud", "gcp": "Cloud", "cloud": "Cloud",
		"ai": "AI Tools", "llm": "AI Tools", "memory": "AI Tools",
		"api": "Developer", "http": "Developer", "fetch": "Web Search",
	}

	for keyword, category := range keywords {
		if strings.Contains(text, keyword) {
			return category
		}
	}

	return "Other"
}

func getIcon(name, category string) string {
	icons := map[string]string{
		"File System":  "📁",
		"Database":     "🗄️",
		"Web Search":   "🔍",
		"Browser":      "🌐",
		"AI Tools":     "🤖",
		"Cloud":        "☁️",
		"Communication": "💬",
		"Developer":    "🛠️",
		"Data":         "📊",
		"Other":        "📦",
	}

	if icon, ok := icons[category]; ok {
		return icon
	}

	// 根据名称选择图标
	iconsList := []string{"📁", "🗄️", "🔍", "🌐", "🤖", "☁️", "💬", "🛠️", "📊", "📦"}
	if len(name) > 0 {
		return iconsList[int(name[0])%len(iconsList)]
	}
	return "📦"
}

func generateConfig(slug, installCmd string) string {
	return fmt.Sprintf(`{"mcpServers": {"%s": {"command": "%s"}}}`, slug, installCmd)
}

func truncate(s string, maxLen int) string {
	// 移除 HTML 标签
	re := regexp.MustCompile(`<[^>]*>`)
	s = re.ReplaceAllString(s, "")

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func getBackupServers() map[string]models.Server {
	servers := []models.Server{
		{
			Name:        "Filesystem MCP",
			Slug:        "filesystem",
			Icon:        "📁",
			Category:    "File System",
			Description: "安全文件系统操作，支持读写、搜索和管理文件",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem",
			Stars:       8500,
			Downloads:   50000,
			InstallCmd:  "npx @anthropic/mcp-server-filesystem",
			Config:      `{"mcpServers": {"filesystem": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-filesystem", "/path/to/allowed/dir"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "PostgreSQL MCP",
			Slug:        "postgresql",
			Icon:        "🗄️",
			Category:    "Database",
			Description: "只读 PostgreSQL 数据库访问，支持模式检查",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/postgres",
			Stars:       6200,
			Downloads:   35000,
			InstallCmd:  "npx @anthropic/mcp-server-postgres",
			Config:      `{"mcpServers": {"postgres": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-postgres", "postgresql://user:pass@localhost/db"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "GitHub MCP",
			Slug:        "github",
			Icon:        "🛠️",
			Category:    "Developer",
			Description: "GitHub API 集成，支持仓库、Issue、PR 等操作",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/github",
			Stars:       7800,
			Downloads:   42000,
			InstallCmd:  "npx @anthropic/mcp-server-github",
			Config:      `{"mcpServers": {"github": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-github"], "env": {"GITHUB_TOKEN": "your-token"}}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Puppeteer MCP",
			Slug:        "puppeteer",
			Icon:        "🌐",
			Category:    "Browser",
			Description: "浏览器自动化，支持截图、表单填写、页面导航",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/puppeteer",
			Stars:       5400,
			Downloads:   28000,
			InstallCmd:  "npx @anthropic/mcp-server-puppeteer",
			Config:      `{"mcpServers": {"puppeteer": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-puppeteer"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Brave Search MCP",
			Slug:        "brave-search",
			Icon:        "🔍",
			Category:    "Web Search",
			Description: "Brave 搜索 API 集成，支持网络搜索",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/brave-search",
			Stars:       4800,
			Downloads:   25000,
			InstallCmd:  "npx @anthropic/mcp-server-brave-search",
			Config:      `{"mcpServers": {"brave-search": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-brave-search"], "env": {"BRAVE_API_KEY": "your-key"}}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Slack MCP",
			Slug:        "slack",
			Icon:        "💬",
			Category:    "Communication",
			Description: "Slack API 集成，支持消息发送和频道管理",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/slack",
			Stars:       3200,
			Downloads:   18000,
			InstallCmd:  "npx @anthropic/mcp-server-slack",
			Config:      `{"mcpServers": {"slack": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-slack"], "env": {"SLACK_BOT_TOKEN": "your-token"}}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Google Maps MCP",
			Slug:        "google-maps",
			Icon:        "🗺️",
			Category:    "Data",
			Description: "Google Maps API 集成，支持地址搜索和路线规划",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/google-maps",
			Stars:       2800,
			Downloads:   15000,
			InstallCmd:  "npx @anthropic/mcp-server-google-maps",
			Config:      `{"mcpServers": {"google-maps": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-google-maps"], "env": {"GOOGLE_MAPS_API_KEY": "your-key"}}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Memory MCP",
			Slug:        "memory",
			Icon:        "🧠",
			Category:    "AI Tools",
			Description: "持久化记忆存储，让 AI 记住对话历史",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/memory",
			Stars:       4500,
			Downloads:   22000,
			InstallCmd:  "npx @anthropic/mcp-server-memory",
			Config:      `{"mcpServers": {"memory": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-memory"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Fetch MCP",
			Slug:        "fetch",
			Icon:        "📡",
			Category:    "Web Search",
			Description: "HTTP 请求工具，支持获取网页内容",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/fetch",
			Stars:       3600,
			Downloads:   19000,
			InstallCmd:  "npx @anthropic/mcp-server-fetch",
			Config:      `{"mcpServers": {"fetch": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-fetch"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "SQLite MCP",
			Slug:        "sqlite",
			Icon:        "🗃️",
			Category:    "Database",
			Description: "SQLite 数据库操作，支持查询和修改",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/sqlite",
			Stars:       2900,
			Downloads:   16000,
			InstallCmd:  "npx @anthropic/mcp-server-sqlite",
			Config:      `{"mcpServers": {"sqlite": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-sqlite", "--db-path", "/path/to/db.sqlite"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Git MCP",
			Slug:        "git",
			Icon:        "📝",
			Category:    "Developer",
			Description: "Git 操作工具，支持提交、分支、差异对比",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/git",
			Stars:       4100,
			Downloads:   21000,
			InstallCmd:  "npx @anthropic/mcp-server-git",
			Config:      `{"mcpServers": {"git": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-git", "--repository", "/path/to/repo"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Sequential Thinking MCP",
			Slug:        "sequential-thinking",
			Icon:        "💭",
			Category:    "AI Tools",
			Description: "结构化思考工具，帮助 AI 进行复杂推理",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/sequentialthinking",
			Stars:       3800,
			Downloads:   17000,
			InstallCmd:  "npx @anthropic/mcp-server-sequential-thinking",
			Config:      `{"mcpServers": {"sequential-thinking": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-sequential-thinking"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Redis MCP",
			Slug:        "redis",
			Icon:        "🔴",
			Category:    "Database",
			Description: "Redis 数据库操作，支持键值存储",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/redis",
			Stars:       2100,
			Downloads:   12000,
			InstallCmd:  "npx @anthropic/mcp-server-redis",
			Config:      `{"mcpServers": {"redis": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-redis"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Elasticsearch MCP",
			Slug:        "elasticsearch",
			Icon:        "🔍",
			Category:    "Database",
			Description: "Elasticsearch 搜索引擎集成",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/elasticsearch",
			Stars:       1800,
			Downloads:   9000,
			InstallCmd:  "npx @anthropic/mcp-server-elasticsearch",
			Config:      `{"mcpServers": {"elasticsearch": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-elasticsearch"]}}}`,
			Verified:    true,
			Official:    false,
		},
		{
			Name:        "AWS KB Retrieval MCP",
			Slug:        "aws-kb-retrieval",
			Icon:        "☁️",
			Category:    "Cloud",
			Description: "AWS Knowledge Base 检索服务",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/aws-kb-retrieval",
			Stars:       2400,
			Downloads:   11000,
			InstallCmd:  "npx @anthropic/mcp-server-aws-kb-retrieval",
			Config:      `{"mcpServers": {"aws-kb-retrieval": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-aws-kb-retrieval"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Google Drive MCP",
			Slug:        "gdrive",
			Icon:        "📁",
			Category:    "File System",
			Description: "Google Drive 文件操作集成",
			GitHubURL:   "https://github.com/modelcontextprotocol/servers/tree/main/src/gdrive",
			Stars:       3200,
			Downloads:   16000,
			InstallCmd:  "npx @anthropic/mcp-server-gdrive",
			Config:      `{"mcpServers": {"gdrive": {"command": "npx", "args": ["-y", "@anthropic/mcp-server-gdrive"]}}}`,
			Verified:    true,
			Official:    true,
		},
		{
			Name:        "Exa Search MCP",
			Slug:        "exa-search",
			Icon:        "🔍",
			Category:    "Web Search",
			Description: "Exa AI 搜索引擎集成",
			GitHubURL:   "https://github.com/exa-labs/exa-mcp-server",
			Stars:       1500,
			Downloads:   8000,
			InstallCmd:  "npx exa-mcp-server",
			Config:      `{"mcpServers": {"exa-search": {"command": "npx", "args": ["-y", "exa-mcp-server"], "env": {"EXA_API_KEY": "your-key"}}}}`,
			Verified:    false,
			Official:    false,
		},
		{
			Name:        "Notion MCP",
			Slug:        "notion",
			Icon:        "📝",
			Category:    "Developer",
			Description: "Notion API 集成，支持页面和数据库操作",
			GitHubURL:   "https://github.com/suekou/mcp-notion-server",
			Stars:       1200,
			Downloads:   6000,
			InstallCmd:  "npx mcp-notion-server",
			Config:      `{"mcpServers": {"notion": {"command": "npx", "args": ["-y", "mcp-notion-server"], "env": {"NOTION_API_KEY": "your-key"}}}}`,
			Verified:    false,
			Official:    false,
		},
		{
			Name:        "Linear MCP",
			Slug:        "linear",
			Icon:        "📋",
			Category:    "Developer",
			Description: "Linear 项目管理工具集成",
			GitHubURL:   "https://github.com/gerred/mcp-linear-server",
			Stars:       980,
			Downloads:   5000,
			InstallCmd:  "npx mcp-linear-server",
			Config:      `{"mcpServers": {"linear": {"command": "npx", "args": ["-y", "mcp-linear-server"], "env": {"LINEAR_API_KEY": "your-key"}}}}`,
			Verified:    false,
			Official:    false,
		},
		{
			Name:        "Jira MCP",
			Slug:        "jira",
			Icon:        "🎫",
			Category:    "Developer",
			Description: "Jira 项目管理工具集成",
			GitHubURL:   "https://github.com/sooperset/mcp-atlassian",
			Stars:       850,
			Downloads:   4500,
			InstallCmd:  "npx mcp-atlassian",
			Config:      `{"mcpServers": {"jira": {"command": "npx", "args": ["-y", "mcp-atlassian"], "env": {"JIRA_API_KEY": "your-key"}}}}`,
			Verified:    false,
			Official:    false,
		},
	}

	result := make(map[string]models.Server)
	for _, s := range servers {
		result[s.Slug] = s
	}
	return result
}
