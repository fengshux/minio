package tui

import "charm.land/lipgloss/v2"

// Tokyo Night 暗色主题（spec/tui.md 第六节）
var (
	colBg       = lipgloss.Color("#1e1e1e") // 主背景 暖灰黑
	colPanel    = lipgloss.Color("#2a2a2a") // 面板背景 暖灰
	colModalBg  = lipgloss.Color("#2a2b31") // 弹窗背景（比面板更中性，降低偏蓝感）
	colSelected = lipgloss.Color("#3d3528") // 选中行 暖棕底
	colFg       = lipgloss.Color("#e2ddd2") // 主文字 米白
	colFgDim    = lipgloss.Color("#c5beb0") // 次要文字（降低蓝感）
	colFaint    = lipgloss.Color("#8a8478") // 弱化文字（中性灰）
	colBlue     = lipgloss.Color("#c9a87c") // 蓝色高亮
	colGreen    = lipgloss.Color("#8fb07a") // 成功
	colYellow   = lipgloss.Color("#d4a857") // 警告
	colRed      = lipgloss.Color("#c97a7a") // 错误
	colCyan     = lipgloss.Color("#8ab8c9") // 信息
	colBorder   = lipgloss.Color("#404040") // 边框
)

var (
	titleBarStyle = lipgloss.NewStyle().
			Foreground(colFg).
			Background(colPanel)

	titleBrandStyle = lipgloss.NewStyle().
			Foreground(colBlue).
			Background(colPanel).
			Bold(true)

	titleItemStyle = lipgloss.NewStyle().
			Foreground(colFgDim).
			Background(colPanel)

	titleHelpStyle = lipgloss.NewStyle().
			Foreground(colCyan).
			Background(colPanel)

	crumbStyle = lipgloss.NewStyle().
			Foreground(colFgDim).
			Padding(0, 1)

	crumbBucketStyle = lipgloss.NewStyle().
				Foreground(colBlue).
				Bold(true)

	crumbPrefixStyle = lipgloss.NewStyle().
				Foreground(colFgDim)

	paneHeaderStyle = lipgloss.NewStyle().
			Foreground(colBlue).
			Bold(true)

	paneHeaderDimStyle = lipgloss.NewStyle().
				Foreground(colFaint)

	paneFocusMarkStyle = lipgloss.NewStyle().
				Foreground(colCyan)

	// 列表行
	rowSelectedStyle = lipgloss.NewStyle().
				Background(colSelected).
				Foreground(colFg)

	dirStyle = lipgloss.NewStyle().
			Foreground(colBlue)

	fileStyle = lipgloss.NewStyle().
			Foreground(colFgDim)

	backStyle = lipgloss.NewStyle().
			Foreground(colFaint)

	markStyle = lipgloss.NewStyle().
			Foreground(colYellow)

	activeStyle = lipgloss.NewStyle().
			Foreground(colGreen)

	// 底部区域
	actionBarStyle = lipgloss.NewStyle().
			Foreground(colFgDim).
			Background(colPanel)

	actionKeyStyle = lipgloss.NewStyle().
			Foreground(colCyan).
			Background(colPanel)

	actionSepStyle = lipgloss.NewStyle().
			Foreground(colFaint).
			Background(colPanel)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colFgDim).
			Background(colPanel)

	statusOkStyle = lipgloss.NewStyle().
			Foreground(colGreen).
			Background(colPanel)

	statusErrStyle = lipgloss.NewStyle().
			Foreground(colRed).
			Background(colPanel)

	separatorStyle = lipgloss.NewStyle().
			Foreground(colBorder)

	footerStyle = lipgloss.NewStyle().
			Foreground(colFaint)

	filterStyle = lipgloss.NewStyle().
			Foreground(colYellow)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(colCyan)

	// 预览
	previewBorderStyle = lipgloss.NewStyle().
				Foreground(colBorder)

	previewTitleStyle = lipgloss.NewStyle().
				Foreground(colCyan).
				Bold(true)

	previewHintStyle = lipgloss.NewStyle().
				Foreground(colFaint)

	previewGutterStyle = lipgloss.NewStyle().
				Foreground(colFaint)

	// 弹窗
	modalBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colBorder).
			Background(colModalBg).
			Foreground(colFg).
			Padding(1, 2)

	modalTitleStyle = lipgloss.NewStyle().
			Foreground(colBlue).
			Bold(true)

	modalTitleWarnStyle = lipgloss.NewStyle().
				Foreground(colYellow).
				Bold(true)

	modalTitleErrStyle = lipgloss.NewStyle().
				Foreground(colRed).
				Bold(true)

	modalKeyStyle = lipgloss.NewStyle().
			Foreground(colCyan)

	modalDimStyle = lipgloss.NewStyle().
			Foreground(colFaint)

	modalLabelStyle = lipgloss.NewStyle().
			Foreground(colFgDim)

	modalValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e4e6eb"))

	modalYesStyle = lipgloss.NewStyle().
			Foreground(colGreen).
			Bold(true)

	// 输入行
	inputPromptStyle = lipgloss.NewStyle().
				Foreground(colCyan).
				Bold(true)

	inputModeStyle = lipgloss.NewStyle().
			Foreground(colBg).
			Background(colCyan).
			Bold(true).
			Padding(0, 1)

	inputTextStyle = lipgloss.NewStyle().
			Foreground(colFg)

	smallTermStyle = lipgloss.NewStyle().
			Foreground(colYellow).
			Align(lipgloss.Center)
)
