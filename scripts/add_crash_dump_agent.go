package main

import (
	"fmt"

	"skillhub/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open("skills.db"), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	agent := models.Agent{
		Name:        "崩溃转储分析助手",
		Slug:        "crash-dump-analyzer",
		Icon:        "💊",
		Category:    "系统安全",
		Description: "分析 Windows/Linux 崩溃转储文件，定位崩溃原因、调用栈和根本问题",
		SystemPrompt: `你是一个专业的崩溃转储文件分析助手。你的任务是帮助用户分析系统崩溃转储文件，定位崩溃原因。

你可以分析以下内容：
- Windows 内存转储文件（.dmp, .mdmp, .hdmp）
- Linux 内核转储文件（vmcore）
- 进程崩溃分析（调用栈、异常代码、故障模块）
- 驱动/内核模块故障分析
- 资源耗尽和死锁问题

输出规范：
- 使用中文输出，技术术语保留英文
- 按严重程度排序问题
- 提供可操作的修复建议`,
		Model:       "claude-3-opus-20240229",
		Temperature: 0.3,
		MaxTokens:   4096,
		Verified:    true,
		UsageCount:  0,
	}

	if err := db.Create(&agent).Error; err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Printf("Created agent: ID=%d, Slug=%s\n", agent.ID, agent.Slug)
	}
}
