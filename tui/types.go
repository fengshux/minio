package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// version 显示在标题栏的程序版本
const version = "v0.1.0"

// 终端尺寸约束（spec 第八节）
const (
	minTermWidth  = 80
	minTermHeight = 24
	smallTermCols = 100 // 低于此宽度隐藏非核心列
)

// FocusPane 面板焦点
type FocusPane int

const (
	FocusBuckets FocusPane = iota
	FocusObjects
)

// InputMode 输入行模式
type InputMode int

const (
	InputNone    InputMode = iota
	InputCommand           // ':' 命令模式
	InputFilter            // '/' 过滤模式
	InputPath              // 下载/目标路径输入模式
)

// ModalKind 弹窗类型
type ModalKind int

const (
	ModalNone          ModalKind = iota
	ModalHelp                    // 帮助
	ModalMeta                    // 对象 Meta 信息
	ModalConfirm                 // 通用确认（删除/退出/大文件预览等）
	ModalError                   // 错误提示
	ModalHistory                 // 操作历史
	ModalUploadConfirm           // 上传确认
)

// modal 弹窗状态（kind + 载荷）
type modal struct {
	kind    ModalKind
	title   string
	body    []string             // 正文行
	meta    *objectMeta          // ModalMeta 载荷
	targets []entry              // 删除/上传目标
	local   string               // 上传本地路径（ModalUploadConfirm）
	offset  int                  // 可滚动弹窗（帮助/历史）当前偏移
	onYes   func(*Model) tea.Cmd // ModalConfirm 确认后的动作
}

// objectMeta Meta 弹窗展示的对象元数据
type objectMeta struct {
	Key          string
	Size         int64
	ContentType  string
	LastModified time.Time
	ETag         string
	StorageClass string
	Headers      [][2]string
}

// entry 面板行条目（桶 / 目录 / 文件 / 返回行）
type entry struct {
	key      string // 完整 key（桶名或对象全路径），back 行为空
	name     string // 显示名（去掉当前前缀后的短名）
	isBucket bool
	isDir    bool
	isBack   bool
	size     int64
	modified time.Time
	selected bool // 多选标记
}

// newBackEntry 构造 "← .." 返回行
func newBackEntry() entry {
	return entry{isBack: true, name: ".."}
}

// HistoryEntry 操作历史条目
type HistoryEntry struct {
	Timestamp time.Time
	Operation string
	Object    string
	Result    string
}

// statusKind 状态栏消息类型
type statusKind int

const (
	statusNone statusKind = iota
	statusOk
	statusErr
)

// pathInputPurpose 路径输入框的用途
type pathInputPurpose int

const (
	purposeDownloadFile pathInputPurpose = iota
	purposeDownloadDir
)

// pathInput 路径输入状态
type pathInput struct {
	purpose pathInputPurpose
	target  entry // 目标对象/目录
}

// transferState 进行中的异步任务（上传/下载/删除）
type transferState struct {
	op         string
	object     string
	label      string // 展示文案，如 "下载中 file.txt"
	doneBytes  int64
	totalBytes int64
	startedAt  time.Time
}

// busyLabel 生成任务提示文案
func busyLabel(op, object string) string {
	switch op {
	case "get":
		return "下载中 " + object
	case "put":
		return "上传中 " + object
	case "put-dir":
		return "上传目录中 " + object
	case "get-dir":
		return "下载目录中 " + object
	case "del":
		return "删除中 " + object
	case "del-dir":
		return "递归删除中 " + object
	default:
		return op + " " + object
	}
}
