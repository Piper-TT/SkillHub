package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// VulnAPIClient 漏洞查询 REST API 客户端
type VulnAPIClient struct {
	APIBase    string
	Token      string
	HTTPClient *http.Client
}

// NewVulnAPIClient 创建漏洞查询 API 客户端
func NewVulnAPIClient(apiBase, token string) *VulnAPIClient {
	return &VulnAPIClient{
		APIBase: strings.TrimRight(apiBase, "/"),
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *VulnAPIClient) get(path string) (map[string]interface{}, error) {
	reqURL := c.APIBase + path
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func (c *VulnAPIClient) getVersion() string {
	data, err := c.get("/version")
	if err != nil {
		return ""
	}
	if code, ok := data["code"].(float64); ok && code == 0 {
		if d, ok := data["data"].(map[string]interface{}); ok {
			if v, ok := d["version"].(string); ok {
				return v
			}
		}
	}
	return ""
}

// --- PolicysQueryTool 漏洞查询工具（REST API） ---

// PolicysQueryTool 通过 REST API 查询漏洞数据库
type PolicysQueryTool struct {
	client *VulnAPIClient
}

// NewPolicysQueryTool 创建漏洞查询工具
func NewPolicysQueryTool(client *VulnAPIClient) *PolicysQueryTool {
	return &PolicysQueryTool{client: client}
}

func (t *PolicysQueryTool) Name() string {
	return "query_policys_db"
}

func (t *PolicysQueryTool) Definition() *ToolDefinition {
	return &ToolDefinition{
		Type: "function",
		Function: &FunctionDef{
			Name:        "query_policys_db",
			Description: "查询漏洞补丁策略数据库。支持两种模式：1) 按 CVE 编号精确查询，返回漏洞完整信息（风险等级、CVSS、描述、修复建议、受影响版本、公开EXP/POC等）；2) 按产品名+版本查询，返回该产品的所有已知漏洞列表。cve_id 和 product 至少提供一个。注意：按产品查询时 version 必填。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"cve_id": map[string]interface{}{
						"type":        "string",
						"description": "CVE 编号，例如 CVE-2024-3094。提供时精确查询单个漏洞的完整信息。",
					},
					"product": map[string]interface{}{
						"type":        "string",
						"description": "产品名称，例如 openssh、nginx、apache。配合 version 参数查询产品的已知漏洞。当没有 CVE 编号时使用。",
					},
					"version": map[string]interface{}{
						"type":        "string",
						"description": "产品版本号，例如 8.9p1、1.24.0。配合 product 参数使用，必填。如用户未提供版本，请先向用户确认。",
					},
				},
			},
		},
	}
}

func (t *PolicysQueryTool) Execute(args map[string]interface{}) (string, error) {
	cveID, _ := args["cve_id"].(string)
	product, _ := args["product"].(string)
	version, _ := args["version"].(string)

	if cveID == "" && product == "" {
		return "", fmt.Errorf("请提供 cve_id 或 product 参数")
	}

	if cveID != "" {
		return t.queryByCVE(cveID)
	}
	if version == "" {
		return "", fmt.Errorf("按产品查询时必须提供 version 参数，请向用户确认产品版本号")
	}
	return t.queryByProduct(product, version)
}

func (t *PolicysQueryTool) queryByCVE(cveID string) (string, error) {
	var sb strings.Builder

	// 获取漏洞库版本
	if v := t.client.getVersion(); v != "" {
		fmt.Fprintf(&sb, "📦 漏洞库版本: %s\n\n", v)
	}

	data, err := t.client.get("/query/cve?cve=" + url.QueryEscape(cveID))
	if err != nil {
		return "", err
	}

	if code, ok := data["code"].(float64); !ok || code != 0 {
		msg, _ := data["message"].(string)
		return "", fmt.Errorf("查询失败: %s", msg)
	}

	d, ok := data["data"].(map[string]interface{})
	if !ok || !toBool(d["found"]) {
		sb.WriteString(fmt.Sprintf("📋 收录状态：❌ 未收录\n\nCVE %s 暂未纳入 EDR 漏洞库，可能原因：\n", cveID))
		sb.WriteString("  • 该漏洞较新，漏洞库尚未更新\n")
		sb.WriteString("  • 不在 EDR 版本匹配引擎覆盖范围内\n")
		sb.WriteString("  • policys.db 不是最新版本\n\n建议：联系策略组确认漏洞库更新计划")
		return sb.String(), nil
	}

	vulnsRaw, ok := d["vulnerabilities"].([]interface{})
	if !ok || len(vulnsRaw) == 0 {
		return fmt.Sprintf("CVE %s 查询结果异常：无漏洞数据", cveID), nil
	}

	total := len(vulnsRaw)
	fmt.Fprintf(&sb, "📋 收录状态：✅ 已收录 | 共 %d 条记录\n\n", total)

	// 统计风险分布
	riskCounts := make(map[string]int)
	for _, v := range vulnsRaw {
		if m, ok := v.(map[string]interface{}); ok {
			risk, _ := m["risk"].(string)
			if risk != "" {
				riskCounts[risk]++
			}
		}
	}
	if len(riskCounts) > 0 {
		var parts []string
		for _, level := range []string{"Critical", "High", "Medium", "Low", "Info"} {
			if c, ok := riskCounts[level]; ok && c > 0 {
				emoji := riskEmoji(level)
				parts = append(parts, fmt.Sprintf("%s %s: %d", emoji, level, c))
			}
		}
		if len(parts) > 0 {
			fmt.Fprintf(&sb, "风险分布：%s\n\n", strings.Join(parts, " | "))
		}
	}

	// 输出每条漏洞详情
	for i, v := range vulnsRaw {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		name := strVal(m, "name")
		risk := strVal(m, "risk")
		cvss := strVal(m, "cvssBase")
		cnnvd := strVal(m, "cnnvd")
		vulnType := strVal(m, "vulnType")
		threatType := strVal(m, "threatType")
		pubDate := strVal(m, "publishedDate")
		desc := strVal(m, "desc")
		advice := strVal(m, "advice")
		exp := strVal(m, "exp")
		poc := strVal(m, "poc")

		fmt.Fprintf(&sb, "%s\n", strings.Repeat("═", 60))
		fmt.Fprintf(&sb, "记录 %d/%d  %s %s | CVSS %s\n", i+1, total, riskEmoji(risk), risk, cvss)
		fmt.Fprintf(&sb, "%s\n", strings.Repeat("─", 60))
		fmt.Fprintf(&sb, "漏洞名称：%s\n", name)
		fmt.Fprintf(&sb, "风险等级：%s\n", risk)
		fmt.Fprintf(&sb, "CVSS 评分：%s\n", cvss)
		if cnnvd != "" {
			fmt.Fprintf(&sb, "CNNVD：%s\n", cnnvd)
		}
		if vulnType != "" || threatType != "" {
			fmt.Fprintf(&sb, "漏洞类型：%s | 威胁类型：%s\n", vulnType, threatType)
		}
		if pubDate != "" {
			fmt.Fprintf(&sb, "发布日期：%s\n", pubDate)
		}
		fmt.Fprintf(&sb, "公开 EXP：%s | 公开 POC：%s\n", boolEmoji(exp), boolEmoji(poc))
		if desc != "" {
			fmt.Fprintf(&sb, "\n漏洞描述：\n  %s\n", desc)
		}
		if advice != "" {
			fmt.Fprintf(&sb, "\n修复建议：\n  %s\n", advice)
		}
	}
	fmt.Fprintf(&sb, "%s\n", strings.Repeat("═", 60))

	return sb.String(), nil
}

func (t *PolicysQueryTool) queryByProduct(product, version string) (string, error) {
	var sb strings.Builder

	if v := t.client.getVersion(); v != "" {
		fmt.Fprintf(&sb, "📦 漏洞库版本: %s\n\n", v)
	}

	apiURL := "/query/product?name=" + url.QueryEscape(product)
	if version != "" {
		apiURL += "&version=" + url.QueryEscape(version)
	}

	data, err := t.client.get(apiURL)
	if err != nil {
		return "", err
	}

	if code, ok := data["code"].(float64); !ok || code != 0 {
		msg, _ := data["message"].(string)
		return "", fmt.Errorf("查询失败: %s", msg)
	}

	d, ok := data["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("响应格式异常")
	}

	vulnsRaw, ok := d["vulnerabilities"].([]interface{})
	if !ok || len(vulnsRaw) == 0 {
		fmt.Fprintf(&sb, "✅ %s %s 未发现已知漏洞", product, version)
		return sb.String(), nil
	}

	total := len(vulnsRaw)
	fmt.Fprintf(&sb, "📋 收录状态：✅ 已收录 | 共 %d 条\n\n", total)

	// 风险分布
	riskCounts := make(map[string]int)
	for _, v := range vulnsRaw {
		if m, ok := v.(map[string]interface{}); ok {
			risk, _ := m["risk"].(string)
			if risk != "" {
				riskCounts[risk]++
			}
		}
	}
	var parts []string
	for _, level := range []string{"Critical", "High", "Medium", "Low", "Info"} {
		if c, ok := riskCounts[level]; ok && c > 0 {
			parts = append(parts, fmt.Sprintf("%s %s: %d", riskEmoji(level), level, c))
		}
	}
	if len(parts) > 0 {
		fmt.Fprintf(&sb, "风险分布：%s\n\n", strings.Join(parts, " | "))
	}

	// 表格输出
	fmt.Fprintf(&sb, "%s\n", strings.Repeat("═", 60))
	fmt.Fprintf(&sb, " CVE | 风险等级 | CVSS | 漏洞类型 | 组件\n")
	fmt.Fprintf(&sb, "%s\n", strings.Repeat("─", 60))
	for _, v := range vulnsRaw {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		cve := strVal(m, "cve")
		risk := strVal(m, "risk")
		cvss := strVal(m, "cvssBase")
		vulnType := strVal(m, "vulnType")
		name := strVal(m, "name")
		if len(name) > 22 {
			name = name[:20] + ".."
		}
		if len(vulnType) > 14 {
			vulnType = vulnType[:12] + ".."
		}
		fmt.Fprintf(&sb, " %s | %s%s | %s | %s | %s\n", cve, riskEmoji(risk), risk, cvss, vulnType, name)
	}
	fmt.Fprintf(&sb, "%s\n", strings.Repeat("═", 60))
	fmt.Fprintf(&sb, "\n查看详情请使用 cve_id 参数查询具体漏洞")

	return sb.String(), nil
}

// --- helper functions ---

func riskEmoji(level string) string {
	switch level {
	case "Critical":
		return "🔴"
	case "High":
		return "🟠"
	case "Medium":
		return "🟡"
	case "Low":
		return "⚪"
	case "Info":
		return "🔵"
	default:
		return "⚪"
	}
}

func boolEmoji(val string) string {
	if val == "1" || val == "true" {
		return "✅"
	}
	return "❌"
}

func toBool(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	if f, ok := v.(float64); ok {
		return f != 0
	}
	return false
}

func strVal(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}
