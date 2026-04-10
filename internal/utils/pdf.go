package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signintech/gopdf"
)

const (
	pw = 595.28 // A4 宽度 (pt)
	ph = 841.89 // A4 高度 (pt)
	ml = 65     // 左边距
	mt = 80     // 上边距
	mr = 65     // 右边距
)

// findFontFile 查找中文字体文件
func findFontFile() string {
	candidates := []string{
		"fonts/simhei.ttf",
	}
	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), "fonts", "simhei.ttf"))
	}
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

// findTemplatePDF 查找报告模板 PDF
func findTemplatePDF() string {
	candidates := []string{
		"assets/report_template.pdf",
	}
	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), "assets", "report_template.pdf"))
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// pdfWriter 封装 PDF 写入状态
type pdfWriter struct {
	pdf   *gopdf.GoPdf
	lineH float64
	y     float64
}

func newPDFWriter() (*pdfWriter, error) {
	fontPath := findFontFile()
	if fontPath == "" {
		return nil, fmt.Errorf("未找到中文字体文件")
	}

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: gopdf.Rect{W: pw, H: ph}})

	if err := pdf.AddTTFFont("simhei", fontPath); err != nil {
		return nil, fmt.Errorf("加载字体失败: %w", err)
	}

	return &pdfWriter{pdf: pdf, lineH: 16, y: float64(mt)}, nil
}

// font 设置字体大小
func (w *pdfWriter) font(size int) {
	_ = w.pdf.SetFont("simhei", "", size)
}

// checkPage 检查换页
func (w *pdfWriter) checkPage() {
	if w.y+w.lineH > ph-float64(mt) {
		w.pdf.AddPage()
		w.y = float64(mt)
	}
}

// text 写入一行文本（自动换行）
func (w *pdfWriter) text(str string) {
	maxW := pw - float64(ml) - float64(mr)
	runes := []rune(str)
	start := 0
	for start < len(runes) {
		w.checkPage()
		w.pdf.SetXY(float64(ml), w.y)

		// 找到当前行能放下的最多字符
		end := len(runes)
		for end > start+1 {
			width, _ := w.pdf.MeasureTextWidth(string(runes[start:end]))
			if width <= maxW {
				break
			}
			end--
		}
		if end == start {
			end = start + 1
		}

		w.pdf.Cell(nil, string(runes[start:end]))
		w.y += w.lineH
		start = end
	}
}

// br 换行
func (w *pdfWriter) br(h float64) {
	w.y += h
}

// line 画水平线
func (w *pdfWriter) line() {
	w.pdf.Line(float64(ml), w.y, pw-float64(mr), w.y)
}

// stripMD 去除 Markdown 格式标记
func stripMD(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return s
}

// GeneratePDFReport 生成 PDF 分析报告
func GeneratePDFReport(filename, reportMD, outputPath string) error {
	if dir := filepath.Dir(outputPath); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	w, err := newPDFWriter()
	if err != nil {
		return err
	}

	// 第一页：导入模板首页
	tplPath := findTemplatePDF()
	if tplPath != "" {
		tplID := w.pdf.ImportPage(tplPath, 1, "/MediaBox")
		w.pdf.AddPage()
		w.pdf.UseImportedTemplate(tplID, 0, 0, pw, ph)
	}

	// 第二页起：正文内容
	w.pdf.AddPage()
	w.y = float64(mt)

	// 报告标题
	w.font(16)
	w.lineH = 24
	w.text("恶意文件分析报告")
	w.br(4)
	w.font(11)
	w.lineH = 16
	w.text(fmt.Sprintf("分析文件: %s", filename))
	w.br(2)
	w.line()
	w.br(8)

	// 解析 Markdown
	lines := strings.Split(reportMD, "\n")
	inCode := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 代码块切换
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			w.font(7)
			w.lineH = 10
			r := []rune(line)
			if len(r) > 90 {
				line = string(r[:90]) + "..."
			}
			w.text(line)
			continue
		}

		if trimmed == "" {
			continue
		}

		// 表格分隔线
		if strings.HasPrefix(trimmed, "|--") || strings.HasPrefix(trimmed, "| --") || strings.HasPrefix(trimmed, "|---") {
			continue
		}

		// 标题
		if strings.HasPrefix(trimmed, "### ") {
			w.br(3)
			w.font(11)
			w.lineH = 16
			w.text(stripMD(strings.TrimPrefix(trimmed, "### ")))
			w.br(1)
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			w.br(4)
			w.line()
			w.br(4)
			w.font(13)
			w.lineH = 18
			w.text(stripMD(strings.TrimPrefix(trimmed, "## ")))
			w.br(2)
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			w.br(6)
			w.font(16)
			w.lineH = 22
			w.text(stripMD(strings.TrimPrefix(trimmed, "# ")))
			w.br(4)
			continue
		}

		// 分隔线
		if strings.HasPrefix(trimmed, "---") && strings.Count(trimmed, "-") == len(trimmed) {
			w.br(4)
			w.line()
			w.br(4)
			continue
		}

		// 表格行
		if strings.HasPrefix(trimmed, "|") {
			cells := strings.Split(trimmed, "|")
			var parts []string
			for _, c := range cells {
				c = strings.TrimSpace(c)
				if c != "" {
					parts = append(parts, stripMD(c))
				}
			}
			w.font(8)
			w.lineH = 12
			w.text(strings.Join(parts, "  |  "))
			continue
		}

		// 列表项
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			t := "• " + stripMD(strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* "))
			w.font(10)
			w.lineH = 14
			w.text(t)
			continue
		}

		// 数字列表
		if len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' && trimmed[1] == '.' {
			w.font(10)
			w.lineH = 14
			w.text(stripMD(trimmed))
			continue
		}

		// 普通文本
		w.font(10)
		w.lineH = 14
		w.text(stripMD(trimmed))
	}

	// 页脚
	w.br(20)
	w.line()
	w.br(6)
	w.font(8)
	w.lineH = 12
	w.text("由 SkillHub 恶意文件分析系统 (LLM Agent) 生成")

	return w.pdf.WritePdf(outputPath)
}

// GenerateSimplePDF 生成简单的 PDF 报告
func GenerateSimplePDF(filename, content, outputPath string) error {
	if dir := filepath.Dir(outputPath); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	w, err := newPDFWriter()
	if err != nil {
		return err
	}

	// 模板首页
	tplPath := findTemplatePDF()
	if tplPath != "" {
		tplID := w.pdf.ImportPage(tplPath, 1, "/MediaBox")
		w.pdf.AddPage()
		w.pdf.UseImportedTemplate(tplID, 0, 0, pw, ph)
	}

	// 正文
	w.pdf.AddPage()
	w.y = float64(mt)
	w.font(16)
	w.lineH = 24
	w.text("恶意文件分析报告")
	w.br(4)
	w.font(11)
	w.lineH = 16
	w.text(fmt.Sprintf("文件: %s", filename))
	w.br(8)
	w.font(8)
	w.lineH = 12
	w.text(content)
	w.br(20)
	w.font(8)
	w.text("由 SkillHub 恶意文件分析系统 (LLM Agent) 生成")

	return w.pdf.WritePdf(outputPath)
}
