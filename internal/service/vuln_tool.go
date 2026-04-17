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
			Description: "查询漏洞补丁策略数据库（policys）。支持两种模式：1) 按 CVE 编号精确查询，返回单个漏洞完整信息；2) 按产品名模糊搜索，返回该产品相关的所有漏洞列表。cve_id 和 product 至少提供一个。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"cve_id": map[string]interface{}{
						"type":        "string",
						"description": "CVE 编号，例如 CVE-2025-8088。提供时精确查询单个漏洞。",
					},
					"product": map[string]interface{}{
						"type":        "string",
						"description": "产品名称关键词，用于模糊搜索。例如 'apache'、'linux kernel'、'nginx'。返回匹配产品的所有漏洞列表。当没有 CVE 编号或想查看某产品的所有漏洞时使用。",
					},
				},
			},
		},
	}
}

func (t *PolicysQueryTool) Execute(args map[string]interface{}) (string, error) {
	cveID, _ := args["cve_id"].(string)
	product, _ := args["product"].(string)

	if cveID == "" && product == "" {
		return "", fmt.Errorf("请提供 cve_id 或 product 参数")
	}

	db := t.mgr.GetDB(DBPolicys)
	if db == nil {
		return "", fmt.Errorf("policys 数据库未加载，请确认数据库文件存在")
	}

	// 模式1: 按产品名模糊搜索漏洞列表
	if cveID == "" && product != "" {
		return t.searchByProduct(db, product)
	}

	// 模式2: 按 CVE 精确查询
	safeCVE := strings.ReplaceAll(cveID, "'", "''")

	// 1. 查询漏洞基本信息（包含所有有意义的字段）
	infoSQL := fmt.Sprintf(
		`SELECT id, name, risk, cve, type, cnnvd, cnvd, cncve, bid, cvss_base, cvss_base_vector,
		desc, advice, name_en, desc_en, advice_en, ref, published_date, creation_date,
		threat_type, vendor, vuln_type, exploitdb, msf, ext FROM policys WHERE cve = '%s' LIMIT 10`,
		safeCVE,
	)
	infoResult, err := executeQuery(db, infoSQL)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	if infoResult == "查询结果为空，没有匹配的数据。" {
		return fmt.Sprintf("未找到 CVE: %s 的记录。", cveID), nil
	}

	sb.WriteString("## 漏洞基本信息\n\n")

	// 解析基本信息
	var infoRows []map[string]interface{}
	if err := json.Unmarshal([]byte(infoResult), &infoRows); err == nil && len(infoRows) > 0 {
		info := infoRows[0]
		vulnID := getFloat64(info, "id")
		name, _ := info["name"].(string)
		nameEn, _ := info["name_en"].(string)
		risk, _ := info["risk"].(string)
		vulnType, _ := info["type"].(string)
		cnnvd, _ := info["cnnvd"].(string)
		cnvd, _ := info["cnvd"].(string)
		cncve, _ := info["cncve"].(string)
		bid := getFloat64(info, "bid")
		cvssBase, _ := info["cvss_base"].(string)
		cvssVector, _ := info["cvss_base_vector"].(string)
		desc, _ := info["desc"].(string)
		descEn, _ := info["desc_en"].(string)
		advice, _ := info["advice"].(string)
		adviceEn, _ := info["advice_en"].(string)
		ref, _ := info["ref"].(string)
		publishedDate, _ := info["published_date"].(string)
		creationDate, _ := info["creation_date"].(string)
		threatType, _ := info["threat_type"].(string)
		vendor, _ := info["vendor"].(string)
		vulnTypeDetail, _ := info["vuln_type"].(string)
		exploitdb, _ := info["exploitdb"].(string)
		msf, _ := info["msf"].(string)
		ext, _ := info["ext"].(string)

		fmt.Fprintf(&sb, "- **CVE**: %s\n", cveID)
		fmt.Fprintf(&sb, "- **漏洞名称**: %s\n", name)
		if nameEn != "" {
			fmt.Fprintf(&sb, "- **English Name**: %s\n", nameEn)
		}
		fmt.Fprintf(&sb, "- **风险等级**: %s\n", risk)
		if vulnType != "" {
			fmt.Fprintf(&sb, "- **分类**: %s\n", vulnType)
		}
		if cvssBase != "" {
			fmt.Fprintf(&sb, "- **CVSS 评分**: %s\n", cvssBase)
		}
		if cvssVector != "" {
			fmt.Fprintf(&sb, "- **CVSS 向量**: %s\n", cvssVector)
		}
		if threatType != "" {
			fmt.Fprintf(&sb, "- **威胁类型**: %s\n", threatType)
		}
		if vendor != "" {
			fmt.Fprintf(&sb, "- **厂商**: %s\n", vendor)
		}
		if vulnTypeDetail != "" {
			fmt.Fprintf(&sb, "- **漏洞类型**: %s\n", vulnTypeDetail)
		}
		if cnnvd != "" {
			fmt.Fprintf(&sb, "- **CNNVD**: %s\n", cnnvd)
		}
		if cnvd != "" {
			fmt.Fprintf(&sb, "- **CNVD**: %s\n", cnvd)
		}
		if cncve != "" {
			fmt.Fprintf(&sb, "- **CNCVE**: %s\n", cncve)
		}
		if bid > 0 {
			fmt.Fprintf(&sb, "- **BID**: %.0f\n", bid)
		}
		if publishedDate != "" {
			fmt.Fprintf(&sb, "- **发布日期**: %s\n", publishedDate)
		}
		if creationDate != "" {
			fmt.Fprintf(&sb, "- **创建日期**: %s\n", creationDate)
		}
		if exploitdb != "" {
			fmt.Fprintf(&sb, "- **ExploitDB**: %s\n", exploitdb)
		}
		if msf != "" {
			fmt.Fprintf(&sb, "- **MSF**: %s\n", msf)
		}
		if ext != "" && ext != `{"poc": "0", "exp": "0", "poc_url": [], "exp_url": []}` {
			fmt.Fprintf(&sb, "- **扩展信息**: %s\n", ext)
		}
		if ref != "" {
			fmt.Fprintf(&sb, "- **参考链接**: %s\n", ref)
		}
		if desc != "" {
			fmt.Fprintf(&sb, "- **描述**: %s\n", desc)
		}
		if descEn != "" {
			fmt.Fprintf(&sb, "- **Description**: %s\n", descEn)
		}
		if advice != "" {
			fmt.Fprintf(&sb, "- **修复建议**: %s\n", advice)
		}
		if adviceEn != "" {
			fmt.Fprintf(&sb, "- **Advice**: %s\n", adviceEn)
		}
		sb.WriteString("\n")

		// 2. 查询 single 表的受影响产品（子查询先过滤 vulid 减少 JOIN 参与量，加 LIMIT 防止过多结果）
		singleSQL := fmt.Sprintf(
			`SELECT s.arch, s.vulid, s.operation, p.name AS product_name, v.version, op.sql AS operator_sql
			FROM (SELECT arch, vulid, operation, product, mv FROM single WHERE vulid = %d LIMIT 5000) s
			JOIN product p ON s.product = p.id
			JOIN version v ON s.mv = v.id
			LEFT JOIN operation op ON s.operation = op.id`,
			int(vulnID),
		)
		singleResult, err := executeQuery(db, singleSQL)
		singleRaw := ""
		if err == nil && singleResult != "查询结果为空，没有匹配的数据。" {
			sb.WriteString("## 受影响产品版本（单版本条件）\n\n")
			translated, _ := translatePolicysResult(singleResult)
			sb.WriteString(translated)
			singleRaw = singleResult
		}

		// 3. 查询 double 表的受影响产品（子查询先过滤 vulid 减少 JOIN 参与量，加 LIMIT 防止过多结果）
		doubleSQL := fmt.Sprintf(
			`SELECT d.arch, d.vulid, d.operation, p.name AS product_name, lv.version AS left_version, rv.version AS right_version, op.sql AS operator_sql
			FROM (SELECT arch, vulid, operation, product, lv, rv FROM double WHERE vulid = %d LIMIT 1000) d
			JOIN product p ON d.product = p.id
			JOIN version lv ON d.lv = lv.id
			JOIN version rv ON d.rv = rv.id
			LEFT JOIN operation op ON d.operation = op.id`,
			int(vulnID),
		)
		doubleResult, err := executeQuery(db, doubleSQL)
		doubleRaw := ""
		if err == nil && doubleResult != "查询结果为空，没有匹配的数据。" {
			sb.WriteString("## 受影响产品版本（范围条件）\n\n")
			translated, _ := translatePolicysResult(doubleResult)
			sb.WriteString(translated)
			doubleRaw = doubleResult
		}

		// 4. 自动从 products_auth 查询版本检测方法
		if t.mgr.GetDB(DBProductAuth) != nil {
			osCommands := t.collectOSDetectionCommands(singleRaw, doubleRaw)
			if osCommands != "" {
				sb.WriteString("## 版本检测方法\n\n")
				sb.WriteString(osCommands)
			}
		}
	}

	return sb.String(), nil
}

// searchByProduct 按产品名模糊搜索漏洞列表
func (t *PolicysQueryTool) searchByProduct(db *sql.DB, product string) (string, error) {
	safeKeyword := strings.ReplaceAll(product, "'", "''")

	// 模糊匹配：搜索 policys 表的 name、name_en、vendor 字段
	sqlStr := fmt.Sprintf(
		`SELECT id, name, risk, cve, type, cvss_base, published_date, threat_type, vendor, vuln_type FROM policys WHERE name LIKE '%%%s%%' OR name_en LIKE '%%%s%%' OR vendor LIKE '%%%s%%' ORDER BY published_date DESC LIMIT 50`,
		safeKeyword, safeKeyword, safeKeyword,
	)

	result, err := executeQuery(db, sqlStr)
	if err != nil {
		return "", err
	}

	if result == "查询结果为空，没有匹配的数据。" {
		return fmt.Sprintf("未找到与「%s」相关的漏洞记录。", product), nil
	}

	// 格式化为可读列表
	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &rows); err != nil {
		return result, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "找到 %d 条与「%s」相关的漏洞记录：\n\n", len(rows), product)
	for i, row := range rows {
		name, _ := row["name"].(string)
		cve, _ := row["cve"].(string)
		risk, _ := row["risk"].(string)
		vulnType, _ := row["type"].(string)
		cvss, _ := row["cvss_base"].(string)
		published, _ := row["published_date"].(string)
		vendor, _ := row["vendor"].(string)
		vulnTypeDetail, _ := row["vuln_type"].(string)

		fmt.Fprintf(&sb, "%d. **%s**\n", i+1, name)
		fmt.Fprintf(&sb, "   CVE: %s | 风险: %s", cve, risk)
		if cvss != "" {
			fmt.Fprintf(&sb, " | CVSS: %s", cvss)
		}
		if vulnType != "" {
			fmt.Fprintf(&sb, " | 分类: %s", vulnType)
		}
		if vendor != "" {
			fmt.Fprintf(&sb, " | 厂商: %s", vendor)
		}
		if vulnTypeDetail != "" {
			fmt.Fprintf(&sb, " | 漏洞类型: %s", vulnTypeDetail)
		}
		if published != "" {
			fmt.Fprintf(&sb, " | 发布: %s", published)
		}
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "\n如需查看某个漏洞的详细信息，请使用 cve_id 参数查询。\n")

	return sb.String(), nil
}

// collectOSDetectionCommands 从 single/double 查询结果中提取操作系统名，查询 products_auth 获取检测方法
func (t *PolicysQueryTool) collectOSDetectionCommands(singleResult, doubleResult string) string {
	osSet := make(map[string]bool)

	// 从 single/double 结果中提取 product_name，解析出 OS 名称
	collectOSFromJSON := func(jsonStr string) {
		if jsonStr == "" || jsonStr == "查询结果为空，没有匹配的数据。" {
			return
		}
		var rows []map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &rows); err != nil {
			return
		}
		for _, row := range rows {
			productName, _ := row["product_name"].(string)
			if productName == "" {
				continue
			}
			// 拆分 os#package 格式，取得 OS 名称
			osName, _ := splitProductOS(productName)
			// 提取关键词（去版本后缀、别名映射）
			keyword := extractProductKeyword(osName)
			if keyword != "" {
				osSet[keyword] = true
			}
		}
	}

	collectOSFromJSON(singleResult)
	collectOSFromJSON(doubleResult)

	if len(osSet) == 0 {
		return ""
	}

	// 查询 products_auth 获取检测命令
	authDB := t.mgr.GetDB(DBProductAuth)
	if authDB == nil {
		return ""
	}

	var sb strings.Builder
	for osName := range osSet {
		safeName := strings.ReplaceAll(osName, "'", "''")
		sqlStr := fmt.Sprintf(`SELECT product, system, cmd, filepath, registrypath FROM data WHERE product LIKE '%%%s%%' LIMIT 50`, safeName)
		result, err := executeQuery(authDB, sqlStr)
		if err != nil || result == "查询结果为空，没有匹配的数据。" {
			continue
		}

		var rows []map[string]interface{}
		if err := json.Unmarshal([]byte(result), &rows); err != nil {
			continue
		}

		for _, row := range rows {
			product, _ := row["product"].(string)
			system, _ := row["system"].(string)
			cmd, _ := row["cmd"].(string)
			filepath, _ := row["filepath"].(string)
			registrypath, _ := row["registrypath"].(string)

			if cmd == "" && filepath == "" && registrypath == "" {
				continue
			}

			displayName := product
			if system != "" {
				displayName = fmt.Sprintf("%s (%s)", product, system)
			}

			fmt.Fprintf(&sb, "### %s\n\n", displayName)
			if cmd != "" {
				fmt.Fprintf(&sb, "- **检测命令**: `%s`\n", cmd)
			}
			if filepath != "" {
				fmt.Fprintf(&sb, "- **文件路径**: `%s`\n", filepath)
			}
			if registrypath != "" {
				fmt.Fprintf(&sb, "- **注册表路径**: `%s`\n", registrypath)
			}
			sb.WriteString("\n")
			break // 每个 OS 只显示第一条检测规则
		}
	}

	return sb.String()
}

func getFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
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
		operatorSQL, _ := row["operator_sql"].(string)

		// 兼容 single 表（version 列）和 double 表（left_version + right_version 列）
		version, _ := row["version"].(string)
		if version == "" {
			leftVer, _ := row["left_version"].(string)
			rightVer, _ := row["right_version"].(string)
			if leftVer != "" && rightVer != "" {
				version = leftVer + "|" + rightVer
			} else if rightVer != "" {
				version = rightVer
			}
		}

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
