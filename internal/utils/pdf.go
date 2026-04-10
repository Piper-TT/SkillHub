package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signintech/gopdf"
)

const (
	pw = 595.28
	ph = 841.89
	ml = 60.0
	mr = 60.0
	mt = 60.0
)

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

// w 封装 PDF 写入
type w struct {
	p *gopdf.GoPdf
	y float64
}

func newW() (*w, error) {
	fp := findFontFile()
	if fp == "" {
		return nil, fmt.Errorf("未找到中文字体文件")
	}
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: gopdf.Rect{W: pw, H: ph}})
	if err := pdf.AddTTFFont("sh", fp); err != nil {
		return nil, err
	}
	return &w{p: pdf, y: mt}, nil
}

func (w *w) sf(size int) { _ = w.p.SetFont("sh", "", size) }

func (w *w) ck(need float64) {
	if w.y+need > ph-mt {
		w.p.AddPage()
		w.y = mt
	}
}

// t 写一行自动换行文本
func (w *w) t(str string, size int, lh float64) {
	w.sf(size)
	maxW := pw - ml - mr
	runes := []rune(str)
	s := 0
	for s < len(runes) {
		w.ck(lh)
		w.p.SetXY(ml, w.y)
		e := len(runes)
		for e > s+1 {
			ww, _ := w.p.MeasureTextWidth(string(runes[s:e]))
			if ww <= maxW {
				break
			}
			e--
		}
		if e == s {
			e = s + 1
		}
		w.p.Cell(nil, string(runes[s:e]))
		w.y += lh
		s = e
	}
}

// heading 写标题
func (w *w) heading(text string, size int, lh float64) {
	w.br(lh * 0.4)
	// 标题下划线
	w.p.Line(ml, w.y, pw-mr, w.y)
	w.y += 3
	w.t(text, size, lh)
	w.br(lh * 0.3)
}

// subheading 写子标题
func (w *w) subheading(text string, size int, lh float64) {
	w.br(lh * 0.3)
	w.t(text, size, lh)
	w.br(lh * 0.15)
}

// br 换行
func (w *w) br(h float64) { w.y += h }

// hr 画分隔线
func (w *w) hr() {
	w.p.Line(ml, w.y, pw-mr, w.y)
}

// table 写表格（带边框）
func (w *w) table(headers []string, rows [][]string) {
	n := len(headers)
	if n == 0 {
		return
	}
	totalW := pw - ml - mr
	colW := totalW / float64(n)
	rowH := 18.0

	w.sf(9)
	w.ck(rowH * 2)

	// 表头（灰色背景 + 边框）
	x := ml
	for _, h := range headers {
		w.p.SetXY(x, w.y)
		w.p.Cell(nil, h)
		w.p.Rectangle(x, w.y, x+colW, w.y+rowH, "D", 0, 0)
		x += colW
	}
	w.y += rowH

	// 数据行
	for _, row := range rows {
		w.ck(rowH)
		x = ml
		for i := 0; i < n; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			// 截断过长内容
			w.sf(8)
			r := []rune(cell)
			for len(r) > 0 {
				ww, _ := w.p.MeasureTextWidth(string(r))
				if ww <= colW-6 {
					break
				}
				r = r[:len(r)-1]
			}
			cell = string(r)

			w.p.SetXY(x+3, w.y+2)
			w.p.Cell(nil, cell)
			w.p.Rectangle(x, w.y, x+colW, w.y+rowH, "D", 0, 0)
			x += colW
		}
		w.y += rowH
	}
}

// parseMDTable 解析 Markdown 表格
func parseMDTable(lines []string) (headers []string, rows [][]string) {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if strings.HasPrefix(line, "|--") || strings.HasPrefix(line, "| --") || strings.HasPrefix(line, "|---") {
			continue
		}
		cells := strings.Split(line, "|")
		var row []string
		for _, c := range cells {
			c = strings.TrimSpace(c)
			if c != "" {
				row = append(row, stripMD(c))
			}
		}
		if len(row) == 0 {
			continue
		}
		if headers == nil {
			headers = row
		} else {
			rows = append(rows, row)
		}
	}
	return
}

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

	wt, err := newW()
	if err != nil {
		return err
	}

	// 导入模板首页
	if tpl := findTemplatePDF(); tpl != "" {
		id := wt.p.ImportPage(tpl, 1, "/MediaBox")
		wt.p.AddPage()
		wt.p.UseImportedTemplate(id, 0, 0, pw, ph)
	}

	// 正文页
	wt.p.AddPage()
	wt.y = mt

	// 解析 Markdown 为内容块
	type block struct {
		kind  string
		text  string
		lines []string
	}
	lines := strings.Split(reportMD, "\n")
	inCode := false
	var tableBuf []string
	var blocks []block

	flushTable := func() {
		if len(tableBuf) > 0 {
			blocks = append(blocks, block{kind: "table", lines: tableBuf})
			tableBuf = nil
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			flushTable()
			inCode = !inCode
			continue
		}
		if inCode {
			blocks = append(blocks, block{kind: "code", text: line})
			continue
		}

		// 表格行
		if strings.HasPrefix(trimmed, "|") {
			if strings.HasPrefix(trimmed, "|--") || strings.HasPrefix(trimmed, "| ---") || strings.HasPrefix(trimmed, "|---") {
				continue
			}
			tableBuf = append(tableBuf, line)
			continue
		}
		flushTable()

		if trimmed == "" {
			continue
		}

		// 标题
		if strings.HasPrefix(trimmed, "### ") {
			blocks = append(blocks, block{kind: "h3", text: stripMD(strings.TrimPrefix(trimmed, "### "))})
		} else if strings.HasPrefix(trimmed, "## ") {
			blocks = append(blocks, block{kind: "h2", text: stripMD(strings.TrimPrefix(trimmed, "## "))})
		} else if strings.HasPrefix(trimmed, "# ") {
			blocks = append(blocks, block{kind: "h1", text: stripMD(strings.TrimPrefix(trimmed, "# "))})
		} else if strings.HasPrefix(trimmed, "---") && strings.Count(trimmed, "-") == len(trimmed) {
			blocks = append(blocks, block{kind: "hr"})
		} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			t := "  \u2022 " + stripMD(strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* "))
			blocks = append(blocks, block{kind: "li", text: t})
		} else if len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' && trimmed[1] == '.' {
			blocks = append(blocks, block{kind: "li", text: "  " + stripMD(trimmed)})
		} else {
			blocks = append(blocks, block{kind: "p", text: stripMD(trimmed)})
		}
	}
	flushTable()

	// 渲染
	for _, b := range blocks {
		switch b.kind {
		case "h1":
			wt.heading(b.text, 16, 22)
		case "h2":
			wt.heading(b.text, 13, 18)
		case "h3":
			wt.subheading(b.text, 11, 15)
		case "p":
			wt.t(b.text, 10, 15)
			wt.br(3)
		case "li":
			wt.t(b.text, 10, 14)
			wt.br(1)
		case "code":
			wt.ck(10)
			wt.sf(7)
			r := []rune(b.text)
			t := b.text
			if len(r) > 95 {
				t = string(r[:95]) + "..."
			}
			wt.p.SetXY(ml+10, wt.y)
			wt.p.Cell(nil, t)
			wt.y += 10
		case "table":
			headers, rows := parseMDTable(b.lines)
			if len(headers) > 0 {
				wt.table(headers, rows)
				wt.br(6)
			}
		case "hr":
			wt.br(4)
			wt.hr()
			wt.br(6)
		}
	}

	// 页脚
	wt.br(20)
	wt.hr()
	wt.br(4)
	wt.t("由 SkillHub 恶意文件分析系统 (LLM Agent) 生成", 8, 12)

	return wt.p.WritePdf(outputPath)
}

// GenerateSimplePDF 生成简单的 PDF 报告
func GenerateSimplePDF(filename, content, outputPath string) error {
	if dir := filepath.Dir(outputPath); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	wt, err := newW()
	if err != nil {
		return err
	}

	if tpl := findTemplatePDF(); tpl != "" {
		id := wt.p.ImportPage(tpl, 1, "/MediaBox")
		wt.p.AddPage()
		wt.p.UseImportedTemplate(id, 0, 0, pw, ph)
	}

	wt.p.AddPage()
	wt.y = mt
	wt.heading("恶意文件分析报告", 16, 22)
	wt.t(fmt.Sprintf("分析文件: %s", filename), 11, 16)
	wt.br(8)
	wt.t(content, 9, 13)
	wt.br(20)
	wt.hr()
	wt.br(4)
	wt.t("由 SkillHub 恶意文件分析系统 (LLM Agent) 生成", 8, 12)

	return wt.p.WritePdf(outputPath)
}
