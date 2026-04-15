package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// --- policys 数据库查询工具 ---

// PolicysQueryTool policys 数据库查询工具
type PolicysQueryTool struct {
	mgr *MultiVulnDBManager
}

// NewPolicysQueryTool 创建 policys 查询工具
func NewPolicysQueryTool(mgr *MultiVulnDBManager) *PolicysQueryTool {
	return &PolicysQueryTool{mgr: mgr}
}

func (t *PolicysQueryTool) Name() string {
	return "query_policys_db"
}

func (t *PolicysQueryTool) Definition() *ToolDefinition {
	return &ToolDefinition{
		Type: "function",
		Function: &FunctionDef{
			Name:        "query_policys_db",
			Description: "根据 CVE 编号查询漏洞补丁策略数据库（policys），返回漏洞详细信息、受影响产品、版本和操作SQL。这是主数据库，包含 CVE 到补丁策略的完整映射。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"cve_id": map[string]interface{}{
						"type":        "string",
						"description": "CVE 编号，例如 CVE-2025-8088",
					},
				},
				"required": []string{"cve_id"},
			},
		},
	}
}

func (t *PolicysQueryTool) Execute(args map[string]interface{}) (string, error) {
	cveID, ok := args["cve_id"].(string)
	if !ok || cveID == "" {
		return "", fmt.Errorf("缺少 cve_id 参数")
	}

	db := t.mgr.GetDB(DBPolicys)
	if db == nil {
		return "", fmt.Errorf("policys 数据库未加载，请确认 ./data/vuln/policys.db 文件存在")
	}

	// 安全处理：转义单引号防止 SQL 注入
	safeCVE := strings.ReplaceAll(cveID, "'", "''")

	sqlStr := fmt.Sprintf(
		`SELECT single.arch, single.vulid, single.operation, product.name AS product_name, version.version, op.sql AS operator_sql FROM single JOIN product ON single.product = product.id JOIN version ON single.mv = version.id LEFT JOIN operation op ON single.operation = op.id WHERE single.vulid IN (SELECT id FROM policys WHERE cve = '%s')`,
		safeCVE,
	)

	rawResult, err := executeQuery(db, sqlStr)
	if err != nil {
		return "", err
	}

	// 解析原始结果并翻译 operator_sql
	return translatePolicysResult(rawResult)
}

// --- product_auth 数据库查询工具 ---

// ProductAuthQueryTool product_auth 数据库查询工具
type ProductAuthQueryTool struct {
	mgr *MultiVulnDBManager
}

// NewProductAuthQueryTool 创建 product_auth 查询工具
func NewProductAuthQueryTool(mgr *MultiVulnDBManager) *ProductAuthQueryTool {
	return &ProductAuthQueryTool{mgr: mgr}
}

func (t *ProductAuthQueryTool) Name() string {
	return "query_product_auth_db"
}

func (t *ProductAuthQueryTool) Definition() *ToolDefinition {
	return &ToolDefinition{
		Type: "function",
		Function: &FunctionDef{
			Name:        "query_product_auth_db",
			Description: "根据产品名查询版本检测规则数据库（products_auth），返回该产品在各系统上的版本检测方法（检测命令、文件路径、注册表路径等）。会自动从产品名中提取关键词。当用户询问'如何检测'、'怎么查出受影响版本'、'检测命令'时调用此工具。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"product": map[string]interface{}{
						"type":        "string",
						"description": "产品/系统名称，如 centos、tencentos、winrar、Microsoft office 2016。会自动提取关键词匹配。",
					},
					"system": map[string]interface{}{
						"type":        "string",
						"description": "可选。限定操作系统类型，如 centos、redhat、Windows、Windowsx64、Debian 等。传入后只返回该系统的检测规则，精确匹配。",
					},
				},
				"required": []string{"product"},
			},
		},
	}
}

func (t *ProductAuthQueryTool) Execute(args map[string]interface{}) (string, error) {
	product, ok := args["product"].(string)
	if !ok || product == "" {
		return "", fmt.Errorf("缺少 product 参数")
	}

	db := t.mgr.GetDB(DBProductAuth)
	if db == nil {
		return "", fmt.Errorf("products_auth 数据库未加载，请确认 ./data/vuln/products_auth.db 文件存在")
	}

	keyword := extractProductKeyword(product)
	safeKeyword := strings.ReplaceAll(keyword, "'", "''")

	// 构建查询：product 模糊匹配
	sqlStr := fmt.Sprintf(`SELECT * FROM data WHERE product LIKE '%%%s%%'`, safeKeyword)

	// 可选 system 参数，精确匹配操作系统
	if sys, ok := args["system"].(string); ok && sys != "" {
		safeSys := strings.ReplaceAll(sys, "'", "''")
		sqlStr += fmt.Sprintf(` AND system = '%s'`, safeSys)
	}

	return executeQuery(db, sqlStr)
}

// extractProductKeyword 从 policys 库的产品名中提取用于匹配 products_auth 的关键词
func extractProductKeyword(name string) string {
	// 去掉 # 后面的组件名：tencentos-2.4#kernel → tencentos-2.4
	if idx := strings.Index(name, "#"); idx > 0 {
		name = name[:idx]
	}
	name = strings.TrimSpace(name)

	// 去掉横杠版本后缀：tencentos-2.4 → tencentos
	if idx := strings.Index(name, "-"); idx > 0 {
		afterDash := name[idx+1:]
		isVerSuffix := true
		for _, c := range afterDash {
			if c != '.' && (c < '0' || c > '9') {
				isVerSuffix = false
				break
			}
		}
		if isVerSuffix {
			name = name[:idx]
		}
	}

	// 去掉末尾的纯数字版本：centos linux 3 → centos linux
	parts := strings.Fields(name)
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		isVersion := true
		for _, c := range last {
			if c != '.' && (c < '0' || c > '9') {
				isVersion = false
				break
			}
		}
		if isVersion {
			parts = parts[:len(parts)-1]
		}
	}

	result := strings.Join(parts, " ")

	// 已知的 OS 别名映射：centos linux → centos
	aliases := map[string]string{
		"centos linux":                    "centos",
		"centos stream":                   "centos",
		"red hat enterprise linux":        "redhat",
		"red hat enterprise linux server": "redhat",
	}
	if alias, ok := aliases[strings.ToLower(result)]; ok {
		return alias
	}

	return result
}

// translatePolicysResult 翻译原始查询结果：翻译 operator_sql 并按操作系统分组聚合
func translatePolicysResult(rawJSON string) (string, error) {
	if rawJSON == "查询结果为空，没有匹配的数据。" {
		return rawJSON, nil
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &rows); err != nil {
		return rawJSON, nil // 解析失败则返回原始结果
	}

	// 按 OS 分组：key = "OS名|version|condition"
	type group struct {
		osName    string
		version   string
		condition string
		packages  []string
	}
	groupMap := make(map[string]*group)
	var groupOrder []string

	for _, row := range rows {
		productName, _ := row["product_name"].(string)
		version, _ := row["version"].(string)
		operatorSQL, _ := row["operator_sql"].(string)

		// 拆分 OS 和包名：centos-7#hivex → osName=centos-7, pkg=hivex
		osName, pkgName := splitProductOS(productName)
		condition := translateOperatorSQL(operatorSQL, version)

		key := osName + "|" + version + "|" + condition
		if g, ok := groupMap[key]; ok {
			// 去重：同一包名不重复添加
			dup := false
			for _, p := range g.packages {
				if p == pkgName {
					dup = true
					break
				}
			}
			if !dup {
				g.packages = append(g.packages, pkgName)
			}
		} else {
			groupMap[key] = &group{
				osName:    osName,
				version:   version,
				condition: condition,
				packages:  []string{pkgName},
			}
			groupOrder = append(groupOrder, key)
		}
	}

	// 构建人类可读的汇总文本（限制最多 30 个分组，避免超出 LLM token 限制）
	var sb strings.Builder
	const maxGroups = 30
	total := len(groupOrder)
	if total > maxGroups {
		fmt.Fprintf(&sb, "共 %d 个操作系统受影响，显示前 %d 个：\n\n", total, maxGroups)
	} else {
		fmt.Fprintf(&sb, "共 %d 个操作系统受影响，分组如下：\n\n", total)
	}

	limit := total
	if limit > maxGroups {
		limit = maxGroups
	}

	for i := 0; i < limit; i++ {
		g := groupMap[groupOrder[i]]
		fmt.Fprintf(&sb, "%d. 【%s】\n", i+1, g.osName)
		fmt.Fprintf(&sb, "   版本条件：%s\n", g.condition)
		fmt.Fprintf(&sb, "   修复版本：%s\n", g.version)
		if len(g.packages) <= 8 {
			fmt.Fprintf(&sb, "   受影响包：%s\n", strings.Join(g.packages, ", "))
		} else {
			fmt.Fprintf(&sb, "   受影响包（%d个）：%s 等\n", len(g.packages), strings.Join(g.packages[:8], ", "))
		}
		sb.WriteString("\n")
	}

	if total > maxGroups {
		fmt.Fprintf(&sb, "... 还有 %d 个操作系统未显示。用户如需查询特定系统，请告知系统名称。\n", total-maxGroups)
	}

	return sb.String(), nil
}

// splitProductOS 拆分产品名为 OS 和包名
// centos-7#hivex → ("centos-7", "hivex")
// red hat enterprise linux 7#hivex → ("red hat enterprise linux 7", "hivex")
// winrar → ("winrar", "")
func splitProductOS(name string) (string, string) {
	if idx := strings.Index(name, "#"); idx > 0 {
		return name[:idx], name[idx+1:]
	}
	return name, ""
}

// translateOperatorSQL 将 operator_sql 翻译为中文自然语言
func translateOperatorSQL(op string, version string) string {
	if op == "" {
		return "未指定版本条件"
	}

	// 单条件
	switch op {
	case "version_is_equal(ver,M,0)":
		return fmt.Sprintf("已安装版本 = %s（仅此版本受影响）", version)
	case "version_is_less(ver,M,0)":
		return fmt.Sprintf("已安装版本 < %s（低于此版本受影响）", version)
	case "version_is_less_equal(ver,M,0)":
		return fmt.Sprintf("已安装版本 ≤ %s（低于或等于此版本受影响）", version)
	case "version_is_greater(ver,M,0)":
		return fmt.Sprintf("已安装版本 > %s（高于此版本受影响）", version)
	case `version_is_less(ver,M,0) and version_is_greater(ver,"0.0",0)`:
		return fmt.Sprintf("0.0 < 已安装版本 < %s（介于之间受影响）", version)
	case `version_is_less_equal(ver,M,0) and version_is_greater(ver,"0.0",0)`:
		return fmt.Sprintf("0.0 < 已安装版本 ≤ %s（介于之间受影响，含上界）", version)
	}

	// 范围条件：需要从 version 字段解析 L 和 R
	// version 字段格式可能是 "1.0|2.0" 或 "1.0" 等
	return translateRangeOperator(op, version)
}

// translateRangeOperator 处理范围条件的翻译
func translateRangeOperator(op string, version string) string {
	// 尝试从 version 中提取范围上下界
	parts := strings.Split(version, "|")
	var left, right string
	if len(parts) >= 2 {
		left = parts[0]
		right = parts[1]
	} else {
		right = version
	}

	switch op {
	case "version_is_less_equal(ver,R,0) and version_is_greater_equal(ver,L,0)":
		if left != "" {
			return fmt.Sprintf("%s ≤ 已安装版本 ≤ %s", left, right)
		}
		return fmt.Sprintf("已安装版本 ≤ %s（含边界）", right)
	case "version_is_less(ver,R,0) and version_is_greater_equal(ver,L,0)":
		if left != "" {
			return fmt.Sprintf("%s ≤ 已安装版本 < %s", left, right)
		}
		return fmt.Sprintf("已安装版本 < %s", right)
	case "version_is_less_equal(ver,R,0) and version_is_greater(ver,L,0)":
		if left != "" {
			return fmt.Sprintf("%s < 已安装版本 ≤ %s", left, right)
		}
		return fmt.Sprintf("已安装版本 ≤ %s", right)
	case "version_is_less(ver,R,0) and version_is_greater(ver,L,0)":
		if left != "" {
			return fmt.Sprintf("%s < 已安装版本 < %s", left, right)
		}
		return fmt.Sprintf("已安装版本 < %s", right)
	}

	return op + "（版本：" + version + "）"
}

// --- 公共查询执行函数 ---

func executeQuery(db *sql.DB, sqlStr string) (string, error) {
	rows, err := db.Query(sqlStr)
	if err != nil {
		return "", fmt.Errorf("SQL 执行失败: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("获取列信息失败: %w", err)
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}
		row := make(map[string]interface{})
		for i, col := range cols {
			if b, ok := values[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = values[i]
			}
		}
		results = append(results, row)
	}

	if len(results) == 0 {
		return "查询结果为空，没有匹配的数据。", nil
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("序列化结果失败: %w", err)
	}
	return string(jsonBytes), nil
}
