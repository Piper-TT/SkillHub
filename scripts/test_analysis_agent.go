package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"skillhub/internal/models"
	"skillhub/internal/repository"
	"skillhub/internal/service"
	"skillhub/internal/utils"
)

func main() {
	// 初始化数据库
	db, err := repository.InitDB("skills.db")
	if err != nil {
		fmt.Printf("数据库初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 创建仓库和服务
	apiKeyRepo := repository.NewAPIKeyRepository(db)
	analysisRepo := repository.NewAnalysisRepository(db)

	// 获取用户配置的 API Key
	userID := "test-user"
	providers := []string{"glm", "deepseek", "openai", "anthropic"}

	var apiKey *models.UserAPIKey
	var provider string

	for _, p := range providers {
		key, err := apiKeyRepo.FindByUserAndProvider(context.Background(), userID, p)
		if err == nil && key != nil {
			apiKey = key
			provider = p
			break
		}
	}

	if apiKey == nil {
		fmt.Println("❌ 未找到配置的 API Key，请先在 AgentHub 页面配置 API Key")
		fmt.Println("   支持的 Provider: glm, deepseek, openai, anthropic")
		os.Exit(1)
	}

	fmt.Printf("✅ 找到 API Key: Provider=%s, Model=%s\n", provider, apiKey.BaseModel)

	// 解密 API Key
	decryptedKey, err := utils.DecryptAPIKey(apiKey.EncryptedKey)
	if err != nil {
		fmt.Printf("❌ 解密 API Key 失败: %v\n", err)
		os.Exit(1)
	}

	// 测试文件路径
	testFile := "uploads/cd/71/cd718112aeffd25a52aac43c39f7ad5a8616ea92fca41749e0e3ceb619630ff3.exe"
	testFileName := "test_malware.exe"

	// 检查文件是否存在
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		fmt.Printf("❌ 测试文件不存在: %s\n", testFile)
		os.Exit(1)
	}

	fmt.Printf("✅ 测试文件: %s\n", testFile)

	// 创建 LLM Agent
	fmt.Println("\n🚀 初始化 LLM Agent...")
	agent := service.NewAnalysisAgent(&service.AnalysisAgentConfig{
		APIKey:   decryptedKey,
		Provider: provider,
		Model:    apiKey.BaseModel,
		MCPURL:   "http://127.0.0.1:8745/mcp",
	})

	// 先启动 MCP 服务器
	fmt.Println("🔄 启动 MCP 服务器...")
	mcpClient := service.NewMCPClient("http://127.0.0.1:8745/mcp")

	if err := mcpClient.StartServer("idalib-mcp.exe", testFile); err != nil {
		fmt.Printf("❌ 启动 MCP 服务器失败: %v\n", err)
		os.Exit(1)
	}
	defer mcpClient.StopServer()

	fmt.Println("✅ MCP 服务器启动成功")
	fmt.Println("🤖 开始 LLM Agent 智能分析...")
	fmt.Println("=" + strings.Repeat("=", 60))

	startTime := time.Now()

	// 执行分析
	result, err := agent.Analyze(context.Background(), testFile, testFileName)
	if err != nil {
		fmt.Printf("❌ 分析失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ 分析完成! 耗时: %v\n", result.Duration)
	fmt.Printf("📊 威胁等级: %s\n", result.ThreatLevel)
	fmt.Printf("🔧 工具调用次数: %d\n", len(result.ToolCalls))

	// 打印工具调用记录
	if len(result.ToolCalls) > 0 {
		fmt.Println("\n📋 工具调用记录:")
		for i, call := range result.ToolCalls {
			status := "✅"
			if call.Error != "" {
				status = "❌"
			}
			fmt.Printf("  %d. %s %s\n", i+1, status, call.Name)
		}
	}

	// 生成报告
	report := agent.GenerateReport(testFileName, result)

	// 保存报告
	reportPath := "test_analysis_report.md"
	if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
		fmt.Printf("❌ 保存报告失败: %v\n", err)
	} else {
		fmt.Printf("\n📄 报告已保存到: %s\n", reportPath)
	}

	// 打印报告摘要
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📄 分析报告预览 (前 1000 字符):")
	fmt.Println(strings.Repeat("-", 60))
	if len(result.RawReport) > 1000 {
		fmt.Println(result.RawReport[:1000] + "...")
	} else {
		fmt.Println(result.RawReport)
	}
}
