package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

// fontDir 返回字体文件所在目录
func fontDir() string {
	// 优先使用可执行文件同级的 fonts/ 目录
	exePath, err := os.Executable()
	if err == nil {
		dir := filepath.Join(filepath.Dir(exePath), "fonts")
		if _, err := os.Stat(filepath.Join(dir, "simhei.json")); err == nil {
			return dir
		}
	}
	// 其次使用工作目录下的 fonts/
	if _, err := os.Stat("fonts/simhei.json"); err == nil {
		return "fonts"
	}
	// 开发模式下使用项目根目录
	if _, err := os.Stat("internal/utils/../../fonts/simhei.json"); err == nil {
		return "fonts"
	}
	return "fonts"
}

// newPDFWithChinese 创建支持中文的 PDF 实例
func newPDFWithChinese() *gofpdf.Fpdf {
	fd := fontDir()
	pdf := gofpdf.New("P", "mm", "A4", fd)
	pdf.AddPage()
	pdf.SetAutoPageBreak(true, 15)

	// 加载中文字体 (SimHei 黑体)
	pdf.AddUTF8Font("simhei", "", filepath.Join(fd, "simhei.json"))
	pdf.AddUTF8Font("simhei", "B", filepath.Join(fd, "simhei.json"))

	return pdf
}

// setChineseFont 设置中文字体
func setChineseFont(pdf *gofpdf.Fpdf, style string, size float64) {
	if runtime.GOOS == "windows" {
		pdf.SetFont("simhei", style, size)
	} else {
		// 非 Windows 系统也使用 simhei
		pdf.SetFont("simhei", style, size)
	}
}

// GeneratePDFReport 生成 PDF 分析报告
func GeneratePDFReport(filename, reportMD, outputPath string) error {
	pdf := newPDFWithChinese()

	// 确保输出目录存在
	if dir := filepath.Dir(outputPath); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	// 标题
	setChineseFont(pdf, "B", 16)
	pdf.Cell(40, 10, "恶意文件分析报告")
	pdf.Ln(12)

	// 文件名
	setChineseFont(pdf, "", 12)
	pdf.Cell(40, 8, fmt.Sprintf("文件: %s", filename))
	pdf.Ln(10)

	// 处理 Markdown 内容
	lines := strings.Split(reportMD, "\n")
	inCodeBlock := false

	for _, line := range lines {
		// 跳过空行
		if strings.TrimSpace(line) == "" {
			if !inCodeBlock {
				pdf.Ln(4)
			}
			continue
		}

		// 代码块切换
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		// 代码块内容
		if inCodeBlock {
			pdf.SetFont("Courier", "", 7)
			// 截断过长行
			runes := []rune(line)
			if len(runes) > 100 {
				line = string(runes[:100]) + "..."
			}
			pdf.MultiCell(0, 4, line, "", "L", false)
			continue
		}

		// 处理标题
		if strings.HasPrefix(line, "# ") {
			setChineseFont(pdf, "B", 14)
			pdf.Ln(4)
			pdf.MultiCell(0, 7, strings.TrimPrefix(line, "# "), "", "L", false)
			pdf.Ln(2)
			continue
		}
		if strings.HasPrefix(line, "## ") {
			setChineseFont(pdf, "B", 12)
			pdf.Ln(3)
			pdf.MultiCell(0, 6, strings.TrimPrefix(line, "## "), "", "L", false)
			pdf.Ln(2)
			continue
		}
		if strings.HasPrefix(line, "### ") {
			setChineseFont(pdf, "B", 11)
			pdf.Ln(2)
			pdf.MultiCell(0, 5, strings.TrimPrefix(line, "### "), "", "L", false)
			pdf.Ln(1)
			continue
		}

		// 处理粗体行
		if strings.HasPrefix(line, "**") && strings.HasSuffix(line, "**") {
			setChineseFont(pdf, "B", 10)
			text := strings.TrimPrefix(strings.TrimSuffix(line, "**"), "**")
			pdf.MultiCell(0, 5, text, "", "L", false)
			continue
		}

		// 处理表格分隔线
		if strings.HasPrefix(line, "|--") || strings.HasPrefix(line, "| --") || strings.HasPrefix(line, "|---") {
			continue
		}

		// 处理表格行
		if strings.HasPrefix(line, "|") {
			setChineseFont(pdf, "", 8)
			cells := strings.Split(line, "|")
			var rowText string
			for _, cell := range cells {
				if strings.TrimSpace(cell) != "" {
					rowText += strings.TrimSpace(cell) + " | "
				}
			}
			if rowText != "" {
				pdf.MultiCell(0, 4, rowText, "", "L", false)
			}
			continue
		}

		// 处理分隔线
		if strings.HasPrefix(line, "---") {
			pdf.Ln(2)
			pdf.Line(10, pdf.GetY(), 200, pdf.GetY())
			pdf.Ln(2)
			continue
		}

		// 处理列表项
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			setChineseFont(pdf, "", 10)
			text := "• " + strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
			pdf.MultiCell(0, 5, text, "", "L", false)
			continue
		}

		// 处理 JSON 内容
		if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "}") || strings.HasPrefix(line, "  \"") {
			pdf.SetFont("Courier", "", 7)
			runes := []rune(line)
			if len(runes) > 100 {
				line = string(runes[:100]) + "..."
			}
			pdf.MultiCell(0, 4, line, "", "L", false)
			continue
		}

		// 普通文本
		setChineseFont(pdf, "", 10)
		runes := []rune(line)
		if len(runes) > 120 {
			line = string(runes[:120]) + "..."
		}
		pdf.MultiCell(0, 5, line, "", "L", false)
	}

	// 页脚
	pdf.Ln(10)
	setChineseFont(pdf, "", 8)
	pdf.Cell(0, 10, "由 SkillHub 恶意文件分析系统 (LLM Agent) 生成")

	return pdf.OutputFileAndClose(outputPath)
}

// GenerateSimplePDF 生成简单的 PDF 报告（仅包含基本信息）
func GenerateSimplePDF(filename, content, outputPath string) error {
	// 确保目录存在
	if dir := filepath.Dir(outputPath); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	pdf := newPDFWithChinese()

	// 标题
	setChineseFont(pdf, "B", 16)
	pdf.Cell(0, 10, "恶意文件分析报告")
	pdf.Ln(12)

	// 文件名
	setChineseFont(pdf, "", 12)
	pdf.Cell(0, 8, fmt.Sprintf("文件: %s", filename))
	pdf.Ln(10)

	// 内容
	setChineseFont(pdf, "", 8)
	pdf.MultiCell(0, 4, content, "", "L", false)

	// 页脚
	pdf.Ln(10)
	setChineseFont(pdf, "", 8)
	pdf.Cell(0, 10, "由 SkillHub 恶意文件分析系统 (LLM Agent) 生成")

	return pdf.OutputFileAndClose(outputPath)
}
