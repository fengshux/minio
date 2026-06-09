package tui

import "github.com/charmbracelet/lipgloss"

// 样式定义
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			Padding(0, 1)

	detailTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#3B82F6")).
				MarginBottom(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1)

	progressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true)

	dirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6"))

	fileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	// 分隔线样式
	separatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#374151"))

	// 输入框容器样式
	inputBoxStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1E3A5F")).
				Padding(0, 1)

	// 模式指示器样式
	commandModeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981")).
				Background(lipgloss.Color("#065F46")).
				Padding(0, 1).
				Bold(true)

	filterModeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F59E0B")).
				Background(lipgloss.Color("#78350F")).
				Padding(0, 1).
				Bold(true)

	waitingModeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				Padding(0, 1)

	// 输入文本样式
	inputTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F9FAFB"))

	placeholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9CA3AF"))
)
