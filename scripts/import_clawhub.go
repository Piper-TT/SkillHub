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
	// 核心
	"agent", "claude", "code", "mcp", "ai", "automation",
	// 开发
	"git", "github", "test", "api", "debug", "build", "deploy",
	// 数据
	"data", "database", "memory", "file", "json", "yaml", "csv",
	// Web
	"web", "browser", "scrape", "crawl", "http", "rest",
	// 文档
	"document", "markdown", "pdf", "text", "write",
	// 搜索
	"search", "query", "find", "filter",
	// 安全
	"security", "auth", "encrypt", "validate",
	// 通信
	"email", "slack", "discord", "notification", "message",
	// 工具
	"tool", "util", "helper", "convert", "parse",
	// 其他
	"image", "video", "audio", "translate", "schedule",
	"workflow", "task", "project", "config", "cli",
}

var categoryMap = map[string]string{
	"agent":      "AI智能",
	"claude":     "AI智能",
	"ai":         "AI智能",
	"automation": "AI智能",
	"code":       "开发工具",
	"git":        "开发工具",
	"github":     "开发工具",
	"test":       "开发工具",
	"api":        "开发工具",
	"debug":      "开发工具",
	"build":      "开发工具",
	"deploy":     "开发工具",
	"mcp":        "开发工具",
	"cli":        "开发工具",
	"data":       "数据管理",
	"database":   "数据管理",
	"memory":     "数据管理",
	"file":       "数据管理",
	"json":       "数据管理",
	"yaml":       "数据管理",
	"csv":        "数据管理",
	"web":        "浏览器自动化",
	"browser":    "浏览器自动化",
	"scrape":     "浏览器自动化",
	"crawl":      "浏览器自动化",
	"http":       "浏览器自动化",
	"rest":       "浏览器自动化",
	"document":   "文档处理",
	"markdown":   "文档处理",
	"pdf":        "文档处理",
	"text":       "文档处理",
	"write":      "文档处理",
	"search":     "信息处理",
	"query":      "信息处理",
	"find":       "信息处理",
	"filter":     "信息处理",
	"security":   "安全工具",
	"auth":       "安全工具",
	"encrypt":    "安全工具",
	"validate":   "安全工具",
	"email":      "办公协同",
	"slack":      "办公协同",
	"discord":    "办公协同",
	"notification": "办公协同",
	"message":    "办公协同",
	"tool":       "其他",
	"util":       "其他",
	"helper":     "其他",
	"convert":    "其他",
	"parse":      "其他",
	"image":      "多媒体",
	"video":      "多媒体",
	"audio":      "多媒体",
	"translate":  "信息处理",
	"schedule":   "办公协同",
	"workflow":   "办公协同",
	"task":       "办公协同",
	"project":    "办公协同",
	"config":     "开发工具",
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

		// 每个关键词搜索 100 条
		url := fmt.Sprintf("https://clawhub.ai/api/v1/search?q=%s&limit=100", query)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "SkillHub/2.0")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("失败: %v\n", err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result ClawHubSearchResult
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Printf("解析失败: %v\n", err)
			continue
		}

		category := categoryMap[query]
		if category == "" {
			category = "其他"
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

		fmt.Printf("找到 %d 条, 新增 %d 条 (总计: %d)\n", len(result.Results), newCount, len(skillsMap))

		// 避免请求过快
		time.Sleep(300 * time.Millisecond)
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
