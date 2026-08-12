package tui

import (
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/minio/minio-go/v7"
	"s3m/operations"
)

// ViewState 视图状态
type ViewState int

const (
	ViewStateObjectList ViewState = iota
	ViewStateHistory
)

// FocusState 焦点状态（用于分屏布局）
type FocusState int

const (
	FocusList FocusState = iota
	FocusDetail
)

// ProgressState 进度状态
type ProgressState struct {
	Active     bool
	Operation  string // "upload" or "download"
	ObjectName string
	TotalBytes int64
	DoneBytes  int64
	Speed      float64 // bytes/sec
	StartTime  time.Time
}

// HistoryEntry 历史记录条目
type HistoryEntry struct {
	Timestamp time.Time
	Operation string // "get", "put", "copy", "sign", "delete"
	Object    string
	Result    string // "success" or error message
}

// Model TUI 主模型
type Model struct {
	client *minio.Client
	lister *operations.Lister
	stater *operations.Stater
	getter *operations.Getter
	putter *operations.Putter
	signer *operations.Signer

	// 状态
	viewState     ViewState
	focusState    FocusState
	currentPath   string // 当前路径：/ 为根目录（桶列表），bucket 或 bucket/prefix 为桶内路径
	currentBucket string // 当前桶名（仅在桶内时有效）
	currentPrefix string // 当前前缀（仅在桶内子目录时有效）
	loading       bool
	err           error
	width         int
	height        int

	// 进度和历史
	progress   ProgressState
	history    []HistoryEntry
	maxHistory int // 50

	// 选中的对象详情
	selectedObject *operations.ObjectInfo

	// 组件
	objectList   list.Model
	input        textinput.Model
	inputFocused bool
	filterMode   bool // 是否处于 filter 输入模式

	// 保存原始列表数据用于过滤
	originalObjectItems []list.Item

	// 流式加载 channel
	loadingCh chan listStreamMsg
}

// NewModel 创建 TUI 模型
func NewModel(client *minio.Client) Model {
	m := Model{
		client:      client,
		lister:      operations.NewLister(client),
		stater:      operations.NewStater(client),
		getter:      operations.NewGetter(client),
		putter:      operations.NewPutter(client),
		signer:      operations.NewSigner(client),
		viewState:   ViewStateObjectList, // 根目录显示桶列表
		focusState:  FocusList,
		currentPath: "/", // 初始为根目录
		maxHistory:  50,
	}

	// 初始化输入框
	m.input = textinput.New()
	m.input.Placeholder = "输入命令 (cd, ls, get, put, sign, history, exit)"
	m.input.Prompt = ": "
	m.input.CharLimit = 256

	// 初始化列表（统一使用 objectList）
	m.objectList = list.New(nil, list.NewDefaultDelegate(), 0, 0)
	m.objectList.Title = "/"
	m.objectList.SetShowStatusBar(false)
	m.objectList.SetFilteringEnabled(false) // 禁用内置 filter，使用统一输入栏

	// 设置自定义委托以拦截ESC键，阻止其退出程序
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("170")).BorderLeftForeground(lipgloss.Color("170"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("170")).BorderLeftForeground(lipgloss.Color("170"))
	m.objectList.SetDelegate(delegate)

	// 禁用列表的退出键绑定（包括 ESC 和 ctrl+c）
	m.objectList.DisableQuitKeybindings()

	return m
}

// AddHistory 添加历史记录
func (m *Model) AddHistory(op, object, result string) {
	entry := HistoryEntry{
		Timestamp: time.Now(),
		Operation: op,
		Object:    object,
		Result:    result,
	}
	m.history = append([]HistoryEntry{entry}, m.history...)
	if len(m.history) > m.maxHistory {
		m.history = m.history[:m.maxHistory]
	}
}
