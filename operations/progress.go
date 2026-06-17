package operations

import (
	"fmt"
	"sync"
)

// ProgressBar 终端进度条
type ProgressBar struct {
	total    int
	current  int
	desc     string
	mu       sync.Mutex
	width    int
	disabled bool
}

// NewProgressBar 创建进度条
func NewProgressBar(total int, desc string) *ProgressBar {
	return &ProgressBar{
		total: total,
		desc:  desc,
		width: 40, // 进度条宽度
	}
}

// Disable 禁用进度条显示
func (p *ProgressBar) Disable() {
	p.disabled = true
}

// Increment 增加进度并更新显示（线程安全）
func (p *ProgressBar) Increment() {
	p.mu.Lock()
	p.current++
	if !p.disabled {
		p.render()
	}
	p.mu.Unlock()
}

// SetCurrent 设置当前进度（线程安全）
func (p *ProgressBar) SetCurrent(current int) {
	p.mu.Lock()
	p.current = current
	if !p.disabled {
		p.render()
	}
	p.mu.Unlock()
}

// render 渲染进度条
func (p *ProgressBar) render() {
	percent := float64(p.current) / float64(p.total) * 100
	filled := int(percent / 100 * float64(p.width))

	// 构建进度条
	bar := ""
	for i := 0; i < p.width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	// 输出到终端（\r 回到行首覆盖）
	fmt.Printf("\r%s: [%s] %.0f%% %d/%d", p.desc, bar, percent, p.current, p.total)
}

// Done 完成进度条，换行
func (p *ProgressBar) Done() {
	if !p.disabled {
		fmt.Println()
	}
}

// AddTotal 增加总数（线程安全）
func (p *ProgressBar) AddTotal(delta int) {
	p.mu.Lock()
	p.total += delta
	if !p.disabled {
		p.render()
	}
	p.mu.Unlock()
}