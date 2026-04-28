package service

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"skillhub/internal/models"
)

// AnalysisAgent 恶意文件分析智能体
type AnalysisAgent struct {
	llmService   *LLMToolService
	toolRegistry *ToolRegistry
	mcpAdapter   *MCPToolAdapter
	apiKey       string
	provider     string
	model        string
}

// AnalysisAgentConfig 智能体配置
type AnalysisAgentConfig struct {
	APIKey    string
	Provider  string // "anthropic", "openai", "deepseek", "glm"
	Model     string
	MCPURL    string
	MCPClient *MCPClient // 使用已初始化的 MCP 客户端
}

// NewAnalysisAgent 创建分析智能体
func NewAnalysisAgent(config *AnalysisAgentConfig) *AnalysisAgent {
	// 创建工具注册中心
	toolRegistry := NewToolRegistry()

	// 使用传入的 MCP 客户端（已启动服务器并初始化 session）
	var mcpClient *MCPClient
	if config.MCPClient != nil {
		mcpClient = config.MCPClient
	} else {
		// 向后兼容：如果没有传入 MCPClient，创建新的
		mcpClient = NewMCPClient(config.MCPURL)
	}
	mcpAdapter := NewMCPToolAdapter(mcpClient)

	// 注册 MCP 工具
	for _, toolDef := range mcpAdapter.GetToolDefinitions() {
		executor := NewMCPToolExecutor(mcpAdapter, toolDef.Function.Name, toolDef)
		toolRegistry.Register(executor)
	}

	// 创建 LLM 服务
	llmService := NewLLMToolService(toolRegistry)

	return &AnalysisAgent{
		llmService:   llmService,
		toolRegistry: toolRegistry,
		mcpAdapter:   mcpAdapter,
		apiKey:       config.APIKey,
		provider:     config.Provider,
		model:        config.Model,
	}
}

// AnalysisResult 分析结果
type AnalysisResult struct {
	Summary     string         // 分析摘要
	ThreatLevel string         // 威胁等级: low, medium, high, critical
	Indicators  []string       // 威胁指标
	ToolCalls   []ToolCallInfo // 工具调用记录
	Duration    time.Duration  // 分析耗时
	RawReport   string         // 原始报告内容
}

// Analyze 分析文件
func (a *AnalysisAgent) Analyze(ctx context.Context, filePath, fileName string) (*AnalysisResult, error) {
	startTime := time.Now()

	// 二进制文件已在 MCP 服务器启动时加载，无需再传 file_path
	// 调用 analyze_binary (survey_binary) 进行初步分析
	_, err := a.mcpAdapter.Execute("analyze_binary", map[string]interface{}{})
	if err != nil {
		fmt.Printf("[Agent] Failed to initialize MCP analysis: %v\n", err)
		// 继续尝试，LLM 可能会重试
	}

	// 构建系统提示
	systemPrompt := a.buildSystemPrompt(fileName)

	// 构建用户消息
	userMessage := fmt.Sprintf(`## 分析任务

**文件名**: %s
**文件路径**: %s

## 分析要求

请对该文件进行全面的恶意软件分析，按照以下阶段执行:

### 阶段1: 静态特征收集
1. 调用 analyze_binary 获取全面的初步分析（文件元数据、节区、导入分类、top 字符串和函数）
2. 调用 get_imports 获取完整导入表，识别危险 API
3. 调用 get_strings 提取所有字符串，筛选可疑内容（URL、IP、注册表路径、命令行等）

### 阶段2: 代码分析
4. 调用 get_functions 获取完整函数列表
5. 根据阶段1的发现，选择可疑函数调用 analyze_function 进行综合分析（包含反编译、调用关系、字符串引用）
6. 分析反编译代码，识别恶意逻辑

### 阶段3: 关联分析
7. 调用 get_xrefs 追踪关键 API 的调用来源（参数 addr 传地址或函数名）
8. 将所有发现映射到 MITRE ATT&CK 框架

### 阶段4: 报告生成
9. 输出 JSON 格式的结构化结果（必须严格遵循格式）
10. 输出 Markdown 格式的可读分析报告

## 输出要求

1. **必须首先输出 JSON 格式结果**，包含 threat_level, ttps, iocs 等字段
2. 然后输出详细的 Markdown 分析报告
3. 每个结论必须有工具返回的数据作为证据
4. 使用中文输出，技术术语保留英文

现在开始分析。`, fileName, filePath)

	// 获取可用工具
	tools := a.toolRegistry.GetAllTools()

	// 调用 LLM with Tools
	req := &ChatWithToolsRequest{
		Provider:     a.provider,
		APIKey:       a.apiKey,
		Model:        a.model,
		SystemPrompt: systemPrompt,
		Messages: []models.ChatMessage{
			{Role: "user", Content: userMessage},
		},
		Tools:    tools,
		MaxTurns: 30, // 增加轮数以完成完整分析
	}

	resp, err := a.llmService.ChatWithTools(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLM 分析失败: %w", err)
	}

	// 构建结果
	result := &AnalysisResult{
		RawReport: resp.Content,
		ToolCalls: resp.ToolCalls,
		Duration:  time.Since(startTime),
	}

	// 解析威胁等级和指标
	a.parseAnalysisResult(result, resp.Content)

	fmt.Printf("[Agent] Analysis completed in %v with %d tool calls\n", result.Duration, len(result.ToolCalls))

	return result, nil
}

// buildSystemPrompt 构建系统提示
func (a *AnalysisAgent) buildSystemPrompt(fileName string) string {
	return `# 角色定义

你是一位资深的恶意软件逆向分析专家，拥有 15+ 年的二进制分析和威胁情报经验。你精通 PE/ELF/Mach-O 文件格式、x86/x64/ARM 汇编语言、反调试/反虚拟机技术以及主流恶意软件家族行为。你产出的分析报告被安全团队和 CERT 直接用于威胁响应和情报共享。

# 分析框架

你的分析必须基于 **MITRE ATT&CK 框架**，将观察到的行为映射到具体的战术(Tactic)和技术(Technique)。

## 核心战术映射表

| 战术 | 常见恶意行为 | 对应 ATT&CK ID |
|------|-------------|----------------|
| 执行 | CreateProcess, WinExec, ShellExecute | T1059 |
| 持久化 | 注册表 Run 键、计划任务、服务安装 | T1547, T1053, T1543 |
| 权限提升 | Token 操作、UAC 绕过 | T1134, T1088 |
| 防御规避 | 进程注入、反调试、加壳 | T1055, T1622, T1027 |
| 凭证访问 | LSASS 内存读取、密码转储 | T1003 |
| 发现 | 系统信息收集、网络扫描 | T1082, T1046 |
| 横向移动 | SMB/WMI 执行、远程桌面 | T1021, T1047 |
| 收集 | 截屏、键盘记录、剪贴板 | T1113, T1056 |
| 命令控制 | HTTP/HTTPS 通信、DNS 隧道 | T1071, T1071.004 |
| 数据外泄 | 文件上传、数据压缩 | T1041, T1560 |

# 可用工具详解

**重要**: 二进制文件已在分析引擎中加载，无需传递文件路径参数。

## 1. analyze_binary - 全面的初步分析（必须首先调用）
**无参数**。返回完整的二进制概览：
- 文件元数据（架构、MD5、SHA256、imagebase、大小）
- 节区列表（含熵值和权限）
- 入口点
- 导入函数按类别分组（crypto/network/file_io/process/registry）
- 按 xref 排序的 top 15 有趣字符串和函数
- 调用图摘要

**分析要点**:
- 熵值 > 7.0 的节区 → 可能加壳
- WX 权限节区 → 代码注入特征
- 异常节区名 (.UPX, .vmp0) → 壳标识

## 2. get_functions - 函数列表
**参数**: offset (number, 默认 0), count (number, 默认 200)
**重点关注**:
- 函数名包含 sub_ 且无符号 → 可能是核心恶意代码
- 大函数 (>500 字节) → 可能包含复杂逻辑
- 入口点附近函数 → 程序初始化逻辑

## 3. get_strings - 字符串提取
**无参数**。使用正则搜索所有字符串。
**可疑字符串模式**:
  网络指标: http://, https://, ftp://, IP地址格式
  凭证相关: password, passwd, pwd, secret, key, token, credential
  系统路径: C:\Windows\System32, %APPDATA%, %TEMP%, \\.\PhysicalDrive
  注册表: HKEY_CURRENT_USER\Software\, CurrentVersion\Run, Winlogon
  进程操作: CreateRemoteThread, VirtualAllocEx, WriteProcessMemory
  加密相关: AES, RSA, XOR, Base64, encrypt, decrypt, crypto
  命令执行: cmd.exe, powershell, wscript, cscript, regsvr32
  反分析: debugger, vmware, virtualbox, sandbox, analyze

## 4. get_imports - 导入表分析
**无参数**。返回所有导入的 DLL 和 API。
**危险 API 分类**:

| 类别 | API 函数 | 风险等级 |
|------|----------|----------|
| 进程注入 | CreateRemoteThread, WriteProcessMemory, VirtualAllocEx | 高危 |
| 内存操作 | VirtualAlloc, VirtualProtect, HeapCreate | 中危 |
| 注册表 | RegCreateKeyEx, RegSetValueEx, RegDeleteKey | 中危 |
| 文件操作 | CreateFile, WriteFile, MoveFileEx, DeleteFile | 低危 |
| 网络通信 | InternetOpen, InternetConnect, HttpSendRequest | 中危 |
| 进程操作 | OpenProcess, TerminateProcess, CreateProcess | 中危 |
| 权限操作 | AdjustTokenPrivileges, LookupPrivilegeValue | 高危 |
| 反调试 | IsDebuggerPresent, CheckRemoteDebuggerPresent | 高危 |

## 5. decompile_function - 函数反编译
**参数**: addr (string, 必填) - 函数地址如 "0x401000" 或函数名如 "sub_401000"、"main"
**何时调用**:
- 函数名可疑或来自危险 API 的调用图
- 字符串引用指向该函数
- 入口点函数需要理解程序初始化逻辑

**代码模式识别**:
- IsDebuggerPresent() 调用 → 反调试 (T1622)
- GetTickCount() 时间检测 → 反调试
- CreateToolhelp32Snapshot 枚举进程 → 进程发现 (T1057)
- 异或循环 → 简单字符串解密

## 6. get_xrefs - 交叉引用
**参数**: addr (string, 必填) - 目标地址如 "0x401000" 或函数名
**用途**: 追踪敏感 API 的调用来源，定位恶意代码位置

## 7. analyze_function - 单函数综合分析（推荐优先使用）
**参数**: addr (string, 必填) - 函数地址或函数名
一次调用返回：伪代码（限 100 行）、top 10 字符串、top 10 常量、调用者、被调用者、交叉引用、基本块摘要。
比分别调用 decompile + xrefs 更高效。

# 分析决策树

开始分析
  |
  +-> 调用 analyze_binary (无参数)
  |     +-> 熵值高? → 标记"可能加壳"
  |     +-> 有 WX 节区? → 标记"代码注入特征"
  |     +-> 查看导入分类和 top 字符串/函数
  |
  +-> 调用 get_imports (无参数)
  |     +-> 有危险 API? → 记录并映射 ATT&CK
  |     +-> 导入表损坏? → 可能加壳/混淆
  |
  +-> 调用 get_strings (无参数)
  |     +-> 发现 C2 URL/IP? → 提取为 IOC
  |     +-> 发现互斥量名? → 家族特征
  |     +-> 发现可疑路径? → 记录行为
  |
  +-> 调用 get_functions
  |     +-> 筛选可疑函数 (入口点、大函数、无符号)
  |
  +-> 对可疑函数调用 analyze_function (推荐) 或 decompile_function
  |     +-> 参数 addr 传函数地址或函数名
  |     +-> 识别代码模式和恶意逻辑
  |
  +-> 调用 get_xrefs 追踪关键 API
        +-> 参数 addr 传地址或函数名

# 报告输出格式

你必须首先输出一个 JSON 格式的结构化结果，然后输出可读的 Markdown 报告。

JSON 格式如下（必须严格遵循）:
{
  "threat_level": "critical 或 high 或 medium 或 low 或 benign",
  "confidence": "high 或 medium 或 low",
  "malware_family": "推测的恶意软件家族名称，未知则填 unknown",
  "file_info": {
    "type": "PE/ELF/Mach-O",
    "architecture": "x86/x64/ARM",
    "bits": 32或64,
    "packed": true或false,
    "packer": "壳名称或null"
  },
  "ttps": [
    {
      "tactic": "战术名称",
      "technique_id": "Txxxx",
      "technique_name": "技术名称",
      "evidence": "观察到的具体证据"
    }
  ],
  "iocs": {
    "network": ["IP地址或域名列表"],
    "files": ["文件路径列表"],
    "registry": ["注册表键列表"],
    "mutex": ["互斥量名称列表"]
  },
  "capabilities": ["恶意能力列表"],
  "summary": "一句话总结恶意行为",
  "recommendations": ["缓解建议列表"]
}

# 分析原则

1. **证据驱动**: 每个结论必须有工具返回的数据支撑
2. **保守评估**: 不确定时选择较低的威胁等级
3. **结构化输出**: 严格按照 JSON 格式输出，便于后续处理
4. **ATT&CK 映射**: 所有恶意行为必须映射到 MITRE ATT&CK
5. **中文输出**: 报告使用中文，但技术术语保留英文

# 质量标准

- 每个威胁判定都有工具数据支撑
- 正确映射至少 3 个 ATT&CK 技术
- 提取完整的 IOC 列表
- 区分"观察到"和"推测"的内容
- 不做无根据的猜测
- 不遗漏明显的恶意特征

# 分析报告示例

以下是一个高质量分析报告的示例，请参考此格式输出:

---示例开始---

**JSON 输出**:
{
  "threat_level": "high",
  "confidence": "high",
  "malware_family": "Emotet",
  "file_info": {
    "type": "PE",
    "architecture": "x86",
    "bits": 32,
    "packed": true,
    "packer": "UPX"
  },
  "ttps": [
    {
      "tactic": "Defense Evasion",
      "technique_id": "T1027",
      "technique_name": "Obfuscated Files or Information",
      "evidence": "检测到 UPX 加壳，.text 节区熵值 7.89"
    },
    {
      "tactic": "Persistence",
      "technique_id": "T1547.001",
      "technique_name": "Boot or Logon Autostart Execution: Registry Run Keys",
      "evidence": "导入 RegSetValueEx，字符串中发现 CurrentVersion\\Run"
    },
    {
      "tactic": "Command and Control",
      "technique_id": "T1071.001",
      "technique_name": "Application Layer Protocol: Web Protocols",
      "evidence": "导入 InternetOpenA/HttpSendRequestA，发现 C2 域名 evil.example.com"
    }
  ],
  "iocs": {
    "network": ["hxxp://evil.example.com/gate.php", "192.168.1.100:443"],
    "files": ["C:\\Users\\Public\\svchost.exe"],
    "registry": ["HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run\\UpdateService"],
    "mutex": ["Global\\EmotetMutex123"]
  },
  "capabilities": ["进程注入", "注册表持久化", "C2通信", "数据窃取"],
  "summary": "该样本为 Emotet 银行木马变种，具有进程注入、持久化和 C2 通信能力",
  "recommendations": ["隔离受感染主机", "检查注册表 Run 键", "阻断 C2 域名通信", "更新终端防护签名"]
}

**Markdown 报告**:

# 恶意软件分析报告

## 执行摘要

该样本为 Emotet 银行木马的变种，采用 UPX 加壳进行混淆。分析确认该恶意软件具有进程注入、注册表持久化和 C2 通信能力，威胁等级为高危。

## 文件信息

| 属性 | 值 |
|------|-----|
| 文件名 | invoice.doc.exe |
| 文件类型 | PE32 executable (GUI) |
| 架构 | x86 (32-bit) |
| 文件大小 | 245,760 bytes |
| 加壳状态 | UPX |
| 编译时间 | 2024-01-15 08:30:00 UTC |

## 静态分析

### 节区分析
| 节区 | 虚拟地址 | 熵值 | 权限 | 分析 |
|------|----------|------|------|------|
| .text | 0x1000 | 7.89 | R-X | 高熵值，疑似加壳 |
| .rdata | 0x8000 | 4.21 | R-- | 正常 |
| .data | 0x9000 | 2.15 | RW- | 正常 |

### 导入函数分析
发现以下高危 API 组合:
- CreateRemoteThread + WriteProcessMemory + VirtualAllocEx → 进程注入能力 (T1055)
- RegSetValueEx + RegCreateKeyEx → 注册表持久化 (T1547)
- InternetOpenA + HttpSendRequestA → HTTP C2 通信 (T1071)

### 字符串分析
提取到以下关键字符串:
- C2 服务器: hxxp://evil.example.com/gate.php
- 持久化路径: HKCU\Software\Microsoft\Windows\CurrentVersion\Run\UpdateService
- 互斥量: Global\EmotetMutex123
- 落地文件: C:\Users\Public\svchost.exe

## 行为分析

### 攻击链
1. 执行后解压 UPX 壳
2. 创建互斥量防止多实例
3. 复制自身到 C:\Users\Public\svchost.exe
4. 添加注册表持久化项
5. 连接 C2 服务器等待指令

### MITRE ATT&CK 映射
| 战术 | 技术 ID | 技术名称 | 证据 |
|------|---------|----------|------|
| Defense Evasion | T1027 | 加壳混淆 | UPX 壳，熵值 7.89 |
| Persistence | T1547.001 | 注册表自启动 | RegSetValueEx + Run 键 |
| Command and Control | T1071.001 | HTTP 通信 | InternetOpen API |

## 威胁评估

- **威胁等级**: 高危
- **置信度**: 高
- **恶意软件家族**: Emotet 变种

## 缓解建议

1. 立即隔离受感染主机，防止横向移动
2. 检查并清理注册表 Run 键中的可疑项
3. 在防火墙阻断 C2 域名 evil.example.com
4. 更新终端防护产品的签名库
5. 对用户进行钓鱼邮件安全意识培训

---示例结束---

请严格按照上述示例的格式和质量标准进行分析和报告输出。`
}

// parseAnalysisResult 解析分析结果
func (a *AnalysisAgent) parseAnalysisResult(result *AnalysisResult, content string) {
	// 提取威胁等级
	contentLower := strings.ToLower(content)

	if strings.Contains(contentLower, "威胁等级") || strings.Contains(contentLower, "严重") {
		if strings.Contains(contentLower, "严重") || strings.Contains(contentLower, "critical") {
			result.ThreatLevel = "critical"
		} else if strings.Contains(contentLower, "高") || strings.Contains(contentLower, "high") {
			result.ThreatLevel = "high"
		} else if strings.Contains(contentLower, "中") || strings.Contains(contentLower, "medium") {
			result.ThreatLevel = "medium"
		} else {
			result.ThreatLevel = "low"
		}
	} else {
		result.ThreatLevel = "unknown"
	}

	// 提取摘要 (取前 500 字符)
	if len(content) > 500 {
		result.Summary = content[:500] + "..."
	} else {
		result.Summary = content
	}

	// 提取指标 (简单实现，可以通过更复杂的解析改进)
	result.Indicators = a.extractIndicators(content)
}

// extractIndicators 提取威胁指标
func (a *AnalysisAgent) extractIndicators(content string) []string {
	var indicators []string

	// 提取 IP 地址、URL、文件路径等
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "http://") ||
			strings.Contains(line, "https://") ||
			strings.Contains(line, "\\") && strings.Contains(line, ".exe") ||
			strings.Contains(line, "HKEY_") ||
			strings.Contains(line, "CreateRemoteThread") ||
			strings.Contains(line, "VirtualAlloc") ||
			strings.Contains(line, "WriteProcessMemory") {
			indicators = append(indicators, line)
		}
	}

	// 限制数量
	if len(indicators) > 20 {
		indicators = indicators[:20]
	}

	return indicators
}

// GenerateReport 生成 Markdown 报告
func (a *AnalysisAgent) GenerateReport(fileName string, result *AnalysisResult) string {
	var sb strings.Builder

	sb.WriteString("# 恶意文件分析报告\n\n")
	sb.WriteString(fmt.Sprintf("**生成时间**: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**文件名**: %s\n", fileName))
	sb.WriteString(fmt.Sprintf("**分析耗时**: %v\n\n", result.Duration))

	// 威胁等级
	threatLevelMap := map[string]string{
		"low":      "🟢 低",
		"medium":   "🟡 中",
		"high":     "🟠 高",
		"critical": "🔴 严重",
		"unknown":  "⚪ 未知",
	}
	sb.WriteString(fmt.Sprintf("**威胁等级**: %s\n\n", threatLevelMap[result.ThreatLevel]))

	// 工具调用统计
	sb.WriteString(fmt.Sprintf("**使用工具**: %d 次调用\n\n", len(result.ToolCalls)))

	// 威胁指标
	if len(result.Indicators) > 0 {
		sb.WriteString("## 关键指标\n\n")
		for _, indicator := range result.Indicators {
			sb.WriteString(fmt.Sprintf("- %s\n", indicator))
		}
		sb.WriteString("\n")
	}

	// 详细分析
	sb.WriteString("## 详细分析\n\n")
	sb.WriteString(result.RawReport)

	// 工具调用记录
	if len(result.ToolCalls) > 0 {
		sb.WriteString("\n\n---\n\n")
		sb.WriteString("## 工具调用记录\n\n")
		sb.WriteString("| 工具 | 参数 | 结果 |\n")
		sb.WriteString("|------|------|------|\n")
		for _, call := range result.ToolCalls {
			argsStr := fmt.Sprintf("%v", call.Arguments)
			if len(argsStr) > 50 {
				argsStr = argsStr[:50] + "..."
			}
			resultStr := "成功"
			if call.Error != "" {
				resultStr = "失败: " + call.Error
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", call.Name, argsStr, resultStr))
		}
	}

	sb.WriteString("\n\n---\n\n")
	sb.WriteString("*报告由 SkillHub 恶意文件分析系统 (LLM Agent) 生成*\n")

	return sb.String()
}
