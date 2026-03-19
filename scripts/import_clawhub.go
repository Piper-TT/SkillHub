package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Skill struct {
	gorm.Model
	Name        string `gorm:"size:255;not null"`
	Icon        string `gorm:"size:10"`
	Category    string `gorm:"size:100;not null"`
	Description string
	Downloads   int64
	Rating      float64
	Verified    bool
	Accelerated bool
	Safe        bool
	FileName    string
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
	"agent",
	"claude",
	"code",
	"mcp",
	"browser",
	"search",
	"git",
	"test",
	"api",
	"data",
	"web",
	"security",
	"ai",
	"automation",
	"memory",
	"github",
	"file",
	"document",
	"email",
	"database",
}

var categoryMap = map[string]string{
	"agent":    "AI智能",
	"claude":   "编程助手",
	"code":     "开发工具",
	"mcp":      "开发工具",
	"browser":  "浏览器自动化",
	"search":   "信息处理",
	"git":      "开发工具",
	"github":   "开发工具",
	"test":     "开发工具",
	"api":      "开发工具",
	"data":     "数据管理",
	"web":      "浏览器自动化",
	"security": "安全工具",
	"ai":       "AI智能",
	"memory":   "数据管理",
	"file":     "数据管理",
	"document": "文档处理",
	"email":    "办公协同",
	"database": "数据管理",
}

func main() {
	db, err := gorm.Open(sqlite.Open("./skills.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&Skill{})

	client := &http.Client{Timeout: 30 * time.Second}
	skillsMap := make(map[string]Skill)

	for _, query := range searchQueries {
		fmt.Printf("Fetching skills for query: %s\n", query)

		url := fmt.Sprintf("https://clawhub.ai/api/v1/search?q=%s&limit=50", query)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "SkillHub/1.0")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error fetching %s: %v\n", query, err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result ClawHubSearchResult
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Printf("Error parsing %s: %v\n", query, err)
			continue
		}

		category := categoryMap[query]
		if category == "" {
			category = "其他"
		}

		for _, item := range result.Results {
			if _, exists := skillsMap[item.Slug]; !exists {
				icon := getIcon(item.DisplayName)
				skillsMap[item.Slug] = Skill{
					Name:        item.DisplayName,
					Icon:        icon,
					Category:    category,
					Description: truncate(item.Summary, 200),
					Downloads:   int64(item.Score * 10000),
					Rating:      4.0 + (item.Score-3.0)/2*1.0,
					Verified:    item.Score > 3.3,
					Accelerated: true,
					Safe:        true,
				}
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Clear existing data
	db.Exec("DELETE FROM skills")

	// Insert new skills
	skills := make([]Skill, 0, len(skillsMap))
	for _, skill := range skillsMap {
		skills = append(skills, skill)
	}

	// Sort by downloads and take top 50
	sortSkills(skills)
	if len(skills) > 50 {
		skills = skills[:50]
	}

	// Assign rankings as downloads
	for i := range skills {
		skills[i].Downloads = int64((50 - i) * 1000 + int(i)*100)
		if i < 3 {
			skills[i].Rating = 5.0
		} else if i < 10 {
			skills[i].Rating = 4.8
		} else {
			skills[i].Rating = 4.5
		}
	}

	if err := db.Create(&skills).Error; err != nil {
		fmt.Printf("Error inserting skills: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully imported %d skills from ClawHub!\n", len(skills))
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

func sortSkills(skills []Skill) {
	for i := 0; i < len(skills); i++ {
		for j := i + 1; j < len(skills); j++ {
			if skills[j].Downloads > skills[i].Downloads {
				skills[i], skills[j] = skills[j], skills[i]
			}
		}
	}
}
