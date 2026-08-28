package tui

import "strings"

// pane 可复用列表面板：统一管理条目、光标、滚动、过滤与多选。
// 通过 visible() 视图（过滤后的下标切片）对外提供可见条目。
type pane struct {
	items    []entry
	filter   string
	filtered []int // 过滤命中项在 items 中的下标；filter 为空时为 nil
	cursor   int   // 相对 visible 视图的光标位置
	offset   int   // 相对 visible 视图的滚动偏移
	rows     int   // 可见行数
}

// visible 返回当前视图下的条目下标列表
func (p *pane) visible() []int {
	if p.filter == "" {
		idx := make([]int, len(p.items))
		for i := range p.items {
			idx[i] = i
		}
		return idx
	}
	return p.filtered
}

// visibleCount 当前视图条目数
func (p *pane) visibleCount() int {
	if p.filter == "" {
		return len(p.items)
	}
	return len(p.filtered)
}

// current 当前光标条目（可能为 nil）
func (p *pane) current() *entry {
	vis := p.visible()
	if p.cursor < 0 || p.cursor >= len(vis) {
		return nil
	}
	return &p.items[vis[p.cursor]]
}

// setItems 替换条目集合，尽量保持光标停留在同名条目上
func (p *pane) setItems(items []entry) {
	var keep string
	if cur := p.current(); cur != nil {
		keep = cur.key
	}
	p.items = items
	p.cursor, p.offset = 0, 0
	p.applyFilter()
	if keep != "" {
		p.moveToKey(keep)
	}
}

// moveToKey 将光标移动到指定 key（不存在则忽略）
func (p *pane) moveToKey(key string) {
	for i, idx := range p.visible() {
		if p.items[idx].key == key {
			p.cursor = i
			p.clampView()
			return
		}
	}
}

// setFilter 设置过滤词并重建过滤视图
func (p *pane) setFilter(f string) {
	p.filter = strings.TrimSpace(f)
	p.cursor, p.offset = 0, 0
	p.applyFilter()
}

// applyFilter 根据当前 filter 重建 filtered 下标切片（返回行始终保留）
func (p *pane) applyFilter() {
	if p.filter == "" {
		p.filtered = nil
		return
	}
	needle := strings.ToLower(p.filter)
	p.filtered = p.filtered[:0]
	for i := range p.items {
		if p.items[i].isBack || strings.Contains(strings.ToLower(p.items[i].name), needle) {
			p.filtered = append(p.filtered, i)
		}
	}
}

// moveCursor 移动光标 delta 行（可为负），自动修正滚动窗口
func (p *pane) moveCursor(delta int) {
	n := p.visibleCount()
	if n == 0 {
		p.cursor, p.offset = 0, 0
		return
	}
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor > n-1 {
		p.cursor = n - 1
	}
	p.clampView()
}

// clampView 保证光标在滚动窗口内，窗口不越过末尾
func (p *pane) clampView() {
	if p.rows <= 0 {
		p.offset = 0
		return
	}
	n := p.visibleCount()
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+p.rows {
		p.offset = p.cursor - p.rows + 1
	}
	maxOffset := n - p.rows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if p.offset > maxOffset {
		p.offset = maxOffset
	}
}

// halfPage 半页行数（至少 1）
func (p *pane) halfPage() int {
	h := p.rows / 2
	if h < 1 {
		h = 1
	}
	return h
}

// top / bottom 跳转首/末行
func (p *pane) top()    { p.moveCursor(-p.visibleCount() - 1) }
func (p *pane) bottom() { p.moveCursor(p.visibleCount() + 1) }

// toggleSelectCurrent 切换当前条目的多选标记
func (p *pane) toggleSelectCurrent() {
	if cur := p.current(); cur != nil && !cur.isBack {
		cur.selected = !cur.selected
	}
}

// selectedEntries 返回所有被多选标记的条目
func (p *pane) selectedEntries() []entry {
	var out []entry
	for i := range p.items {
		if p.items[i].selected && !p.items[i].isBack {
			out = append(out, p.items[i])
		}
	}
	return out
}

// clearSelection 清空多选标记
func (p *pane) clearSelection() {
	for i := range p.items {
		p.items[i].selected = false
	}
}

// jumpMatch 光标跳到上/下一个过滤匹配项（循环），返回是否生效
func (p *pane) jumpMatch(next bool) bool {
	if p.filter == "" || len(p.filtered) == 0 {
		return false
	}
	if next {
		p.cursor = (p.cursor + 1) % len(p.filtered)
	} else {
		p.cursor = (p.cursor - 1 + len(p.filtered)) % len(p.filtered)
	}
	p.clampView()
	return true
}

// removeKeys 从条目集中移除指定 key（删除后刷新用）
func (p *pane) removeKeys(keys map[string]bool) {
	kept := p.items[:0]
	for _, it := range p.items {
		if !keys[it.key] {
			kept = append(kept, it)
		}
	}
	p.items = kept
	p.applyFilter()
	p.clampView()
}
