package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signintech/gopdf"
)

// 确保 gopdf.GoPdf 有 GetY 方法
var _ = (*gopdf.GoPdf)(nil)

const (
	pageWidth  = 595.28 // A4 宽度 (pt)
	pageHeight = 841.89 // A4 高度 (pt)
	marginLeft = 50
	marginTop  = 50
	marginRight = 50
	lineHeight = 16
)

// findFontFile 查找中文字体文件
func findFontFile() string {
	candidates := []string{
		"fonts/simhei.ttf",
	}

	// 可执行文件同级目录
	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), "fonts", "simhei.ttf"))
	}

	// Windows 系统字体
	candidates = append(candidates,
		`C:\Windows\Fonts\simhei.ttf`,
		`C:\Windows\Fonts\msyh.ttc`,
		`C:\Windows\Fonts\simsun.ttc`,
	)

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// newPDFWithChinese 创建支持中文的 PDF
func newPDFWithChinese() (*gopdf.GoPdf, error) {
	fontPath := findFontFile()
	if fontPath == "" {
		return nil, fmt.Errorf("未找到中文字体文件 (simhei.ttf)")
	}

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{
		PageSize: gopdf.Rect{W: pageWidth, H: pageHeight},
	})
	pdf.AddPage()

	if err := pdf.AddTTFFont("simhei", fontPath); err != nil {
		return nil, fmt.Errorf("加载字体失败: %w", err)
	}

	return pdf, nil
}

// textWidth 计算文本渲染宽度
func textWidth(pdf *gopdf.GoPdf, text string) float64 {
	w, _ := pdf.MeasureTextWidth(text)
	return w
}

// wrapText 自动换行，返回按行分割的文本
func wrapText(pdf *gopdf.GoPdf, text string, maxWidth float64) []string {
	var lines []string
	maxCharsPerCheck := 80

	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}

		runes := []rune(paragraph)
		start := 0
		for start < len(runes) {
			end := start + maxCharsPerCheck
			if end > len(runes) {
				end = len(runes)
			}

			chunk := string(runes[start:end])
			w := textWidth(pdf, chunk)

			if w <= maxWidth {
				// 尝试扩展更多字符
				for end < len(runes) {
					nextEnd := end + 10
					if nextEnd > len(runes) {
						nextEnd = len(runes)
					}
					nextChunk := string(runes[start:nextEnd])
					if textWidth(pdf, nextChunk) > maxWidth {
						break
					}
					end = nextEnd
				}
				lines = append(lines, string(runes[start:end]))
				start = end
			} else {
				// 缩减到合适宽度
				for end > start+1 && textWidth(pdf, string(runes[start:end])) > maxWidth {
					end--
				}
				if end == start {
					end = start + 1
				}
				lines = append(lines, string(runes[start:end]))
				start = end
			}
		}
	}

	return lines
}

// curY 获取当前 Y 坐标
func curY(pdf *gopdf.GoPdf) float64 {
	return pdf.GetY()
}

// needNewPage 检查是否需要换页
func needNewPage(pdf *gopdf.GoPdf, lh float64) bool {
	return curY(pdf)+lh > pageHeight-marginTop
}

// writeLine 写入一行文本，自动处理分页
func writeLine(pdf *gopdf.GoPdf, text string, fontSize int, lineHeight float64) {
	if err := pdf.SetFont("simhei", "", fontSize); err != nil {
		fmt.Printf("[PDF] SetFont error: %v\n", err)
		return
	}

	maxWidth := pageWidth - marginLeft - marginRight
	wrappedLines := wrapText(pdf, text, maxWidth)

	for _, line := range wrappedLines {
		if needNewPage(pdf, lineHeight) {
			pdf.AddPage()
			pdf.SetXY(marginLeft, marginTop)
		}
		pdf.Cell(nil, line)
		pdf.Br(lineHeight)
	}
}

// writeLineWithStyle 写入一行文本（可指定大小和行高）
func writeLineWithStyle(pdf *gopdf.GoPdf, text string, fontSize int, lh float64) {
	writeLine(pdf, text, fontSize, lh)
}

// GeneratePDFReport 生成 PDF 分析报告
func GeneratePDFReport(filename, reportMD, outputPath string) error {
	// 确保输出目录存在
	if dir := filepath.Dir(outputPath); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	pdf, err := newPDFWithChinese()
	if err != nil {
		return err
	}

	// 标题
	pdf.SetXY(marginLeft, marginTop)
	writeLineWithStyle(pdf, "恶意文件分析报告", 18, 24)

	// 文件名
	writeLineWithStyle(pdf, fmt.Sprintf("文件: %s", filename), 12, 20)
	pdf.Br(4)

	// 解析并渲染 Markdown
	lines := strings.Split(reportMD, "\n")
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 空行
		if trimmed == "" && !inCodeBlock {
			pdf.Br(6)
			continue
		}

		// 代码块切换
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			if inCodeBlock {
				pdf.Br(2)
			}
			continue
		}

		// 代码块内容
		if inCodeBlock {
			runes := []rune(line)
			if len(runes) > 90 {
				line = string(runes[:90]) + "..."
			}
			writeLineWithStyle(pdf, line, 7, 10)
			continue
		}

		// 跳过纯 Markdown 格式标记中不需要的
		if trimmed == "" {
			continue
		}

		// 标题
		if strings.HasPrefix(trimmed, "### ") {
			text := strings.TrimPrefix(trimmed, "### ")
			text = stripMarkdown(text)
			pdf.Br(4)
			writeLineWithStyle(pdf, text, 12, 18)
			pdf.Br(2)
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			text := strings.TrimPrefix(trimmed, "## ")
			text = stripMarkdown(text)
			pdf.Br(6)
			writeLineWithStyle(pdf, text, 14, 20)
			pdf.Br(2)
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			text := strings.TrimPrefix(trimmed, "# ")
			text = stripMarkdown(text)
			pdf.Br(8)
			writeLineWithStyle(pdf, text, 16, 22)
			pdf.Br(4)
			continue
		}

		// 表格分隔线
		if strings.HasPrefix(trimmed, "|--") || strings.HasPrefix(trimmed, "| --") || strings.HasPrefix(trimmed, "|---") {
			continue
		}

		// 表格行
		if strings.HasPrefix(trimmed, "|") {
			cells := strings.Split(trimmed, "|")
			var parts []string
			for _, c := range cells {
				c = strings.TrimSpace(c)
				if c != "" {
					parts = append(parts, c)
				}
			}
			rowText := strings.Join(parts, " | ")
			writeLineWithStyle(pdf, rowText, 8, 12)
			continue
		}

		// 分隔线
		if strings.HasPrefix(trimmed, "---") && len(trimmed) >= 3 && strings.Count(trimmed, "-") == len(trimmed) {
			pdf.Br(4)
			pdf.Line(marginLeft, pdf.GetY(), pageWidth-marginRight, pdf.GetY())
			pdf.Br(4)
			continue
		}

		// 列表项
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			text := "• " + strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* ")
			text = stripMarkdown(text)
			writeLineWithStyle(pdf, text, 10, 14)
			continue
		}

		// 数字列表
		if len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' && trimmed[1] == '.' {
			text := stripMarkdown(trimmed)
			writeLineWithStyle(pdf, text, 10, 14)
			continue
		}

		// 普通文本
		text := stripMarkdown(trimmed)
		writeLineWithStyle(pdf, text, 10, 14)
	}

	// 页脚
	pdf.Br(20)
	writeLineWithStyle(pdf, "由 SkillHub 恶意文件分析系统 (LLM Agent) 生成", 8, 12)

	// 保存
	return pdf.WritePdf(outputPath)
}

// stripMarkdown 去除简单的 Markdown 格式标记
func stripMarkdown(text string) string {
	// 去除粗体
	text = strings.ReplaceAll(text, "**", "")
	// 去除斜体
	text = strings.ReplaceAll(text, "*", "")
	// 去除行内代码
	text = strings.ReplaceAll(text, "`", "")
	return text
}

// GenerateSimplePDF 生成简单的 PDF 报告
func GenerateSimplePDF(filename, content, outputPath string) error {
	if dir := filepath.Dir(outputPath); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	pdf, err := newPDFWithChinese()
	if err != nil {
		return err
	}

	pdf.SetXY(marginLeft, marginTop)
	writeLineWithStyle(pdf, "恶意文件分析报告", 18, 24)
	writeLineWithStyle(pdf, fmt.Sprintf("文件: %s", filename), 12, 20)
	pdf.Br(8)

	writeLineWithStyle(pdf, content, 8, 12)

	pdf.Br(20)
	writeLineWithStyle(pdf, "由 SkillHub 恶意文件分析系统 (LLM Agent) 生成", 8, 12)

	return pdf.WritePdf(outputPath)
}
