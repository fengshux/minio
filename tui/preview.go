package tui

import "strings"

// renderPreviewSplit 预览分屏：上 40% 对象列表 + 下 60% 预览（spec 4.4）
func (m Model) renderPreviewSplit() string {
	mainH := mainAreaHeight(m.height)
	listRows := maxInt(mainH*previewListRatio/100, 3)
	previewH := maxInt(mainH-listRows, 3)

	listLines := m.renderObjectPane(m.width, listRows)
	previewLines := m.renderPreviewBody(previewH - 2)

	sep := previewBorderStyle.Render(strings.Repeat("─", m.width))
	out := append(listLines, sep)
	out = append(out, previewLines...)
	out = append(out, sep)
	return strings.Join(out, "\n")
}

// renderPreviewBody 预览正文（标题行 + 内容 + 底部提示），返回固定行数
func (m Model) renderPreviewBody(rows int) []string {
	p := m.preview
	var out []string

	// 标题行
	title := previewTitleStyle.Render("Preview: ") +
		fileStyle.Render(shorten(p.name, 40)) + "  " +
		modalDimStyle.Render("("+formatSize(p.size)+")") + "  " +
		previewHintStyle.Render("行 "+p.position(len(p.displayLines(p.cols)), p.rows)) + "  " +
		previewHintStyle.Render("[w]rap"+wrapMark(p.wrapped))
	out = append(out, truncateWidth(title, m.width))

	switch {
	case p.err != nil:
		out = append(out, truncateWidth("  加载失败: "+p.err.Error(), m.width))
	case len(p.lines) == 0 && m.loading:
		out = append(out, "  "+spinnerStyle.Render(m.spinner.View())+" 加载中…")
	default:
		lines := p.displayLines(p.cols)
		contentRows := rows - 1 // 留出底部提示行
		for i := p.offset; i < p.offset+contentRows && i < len(lines); i++ {
			num := previewGutterStyle.Render(padLeft(itoa(i+1), 6) + " │ ")
			out = append(out, num+truncateWidth(lines[i], m.width-9))
		}
	}

	// 底部提示
	footer := previewHintStyle.Render("[Esc] Close  [j/k] Scroll  [Ctrl+D/U] Page  [w] Wrap")
	if p.truncated {
		footer = previewHintStyle.Render("⚠ 内容已截断（仅前 1000 行/64KB）  ") + footer
	}
	for len(out) < rows-1 {
		out = append(out, "")
	}
	return append(out, truncateWidth(footer, m.width))
}

// wrapMark 换行开启标记
func wrapMark(wrapped bool) string {
	if wrapped {
		return " ✓"
	}
	return ""
}

// previewState 文本预览状态（spec 4.4 上下分屏预览）
type previewState struct {
	key       string // 对象完整 key
	name      string // 显示名
	size      int64
	lines     []string // 原始行
	wrapLines []string // 换行开启后的软换行行（懒构建）
	wrapped   bool
	offset    int
	truncated bool
	err       error
	rows      int // 可视行数
	cols      int // 可视列宽
}

// newPreviewState 创建预览状态（内容稍后经 previewLoadedMsg 填充）
func newPreviewState(key string) *previewState {
	name := key
	if i := strings.LastIndex(key, "/"); i >= 0 {
		name = key[i+1:]
	}
	return &previewState{key: key, name: name}
}

// setContent 填充预览内容并初始化尺寸
func (p *previewState) setContent(lines []string, size int64, truncated bool) {
	p.lines = lines
	p.size = size
	p.truncated = truncated
	p.offset = 0
	p.invalidateWrap()
}

// setBounds 设置预览可视区尺寸
func (p *previewState) setBounds(rows, cols int) {
	p.rows = rows
	p.cols = cols
	p.invalidateWrap()
	p.clamp(len(p.displayLines(cols)), rows)
}

// toggleWrap 切换自动换行
func (p *previewState) toggleWrap() {
	p.wrapped = !p.wrapped
	p.offset = 0
	p.invalidateWrap()
}

// invalidateWrap 失效软换行缓存
func (p *previewState) invalidateWrap() {
	p.wrapLines = nil
}

// displayLines 当前模式下的展示行
func (p *previewState) displayLines(cols int) []string {
	if !p.wrapped {
		return p.lines
	}
	if p.wrapLines == nil || p.cols != cols {
		p.cols = cols
		p.wrapLines = softWrapAll(p.lines, cols)
	}
	return p.wrapLines
}

// scroll 滚动 delta 行
func (p *previewState) scroll(delta int, total int, rows int) {
	p.offset += delta
	p.clamp(total, rows)
}

// clamp 限制滚动范围
func (p *previewState) clamp(total, rows int) {
	if rows <= 0 {
		p.offset = 0
		return
	}
	max := total - rows
	if max < 0 {
		max = 0
	}
	if p.offset > max {
		p.offset = max
	}
	if p.offset < 0 {
		p.offset = 0
	}
}

// position 当前位置描述，如 "12/47"
func (p *previewState) position(total, rows int) string {
	if total == 0 {
		return "空"
	}
	end := p.offset + rows
	if end > total {
		end = total
	}
	return itoa(p.offset+1) + "-" + itoa(end) + "/" + itoa(total)
}

// softWrapAll 批量软换行
func softWrapAll(lines []string, cols int) []string {
	if cols <= 0 {
		return lines
	}
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, softWrapLine(l, cols)...)
	}
	return out
}

// softWrapLine 单行软换行（按显示宽度）
func softWrapLine(line string, cols int) []string {
	if cols <= 0 || displayWidth(line) <= cols {
		return []string{line}
	}
	var out []string
	var cur []rune
	w := 0
	for _, r := range line {
		rw := runeWidth(r)
		if w+rw > cols {
			out = append(out, string(cur))
			cur = cur[:0]
			w = 0
		}
		cur = append(cur, r)
		w += rw
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}
