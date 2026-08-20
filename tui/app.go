package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"s3m/operations"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/minio/minio-go/v7"
)

// PersistCurrentContext 可选：用于 set-default 落盘的全局回调
var PersistCurrentContext func(name string) error

// Run 启动 TUI
// contextName 为启动时使用的 context 名称
// onContextChange 为 TUI 内切换 context 时调用的回调（参数：新 context 名称；返回：错误）
func Run(client *minio.Client, contextName string, onContextChange func(name string) (newClient *minio.Client, newCore *minio.Core, err error)) error {
	m := NewModel(client)
	m.contextName = contextName
	m.onContextChange = onContextChange
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

// 消息类型
type listStreamMsg struct {
	items  []list.Item
	path   string // 当前路径
	bucket string // 当前桶名
	prefix string // 当前前缀
	first  bool   // 是否是第一批（用于清空旧列表 + 设置路径）
	done   bool   // 是否加载完成
	err    error
}

type objectInfoLoadedMsg struct {
	info *operations.ObjectInfo
	err  error
}

type operationCompleteMsg struct {
	operation string
	object    string
	err       error
}

type contextSwitchedMsg struct {
	name        string
	newClient   *minio.Client
	newCore     *minio.Core
	persist     bool
	err         error
}

// Init 实现 tea.Model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadRootCmd(m.lister),
		m.objectList.StartSpinner(),
	)
}

// Update 实现 tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// 调整列表大小
		listHeight := m.height - 8                       // 留出标题栏、进度条、命令输入、状态栏
		m.objectList.SetSize(m.width*60/100, listHeight) // 左侧 60%
		return m, nil

	case listStreamMsg:
		if msg.err != nil {
			m.err = msg.err
			m.loading = false
			m.objectList.StopSpinner()
			return m, nil
		}
		if msg.first {
			// 第一批：清空旧列表，设置路径和标题
			m.currentPath = msg.path
			m.currentBucket = msg.bucket
			m.currentPrefix = msg.prefix
			m.objectList.SetItems(msg.items)
			m.objectList.Title = msg.path
			if msg.path == "/" {
				m.objectList.Title = "根目录"
			}
		} else if !msg.done {
			// 增量追加
			currentItems := m.objectList.Items()
			m.objectList.SetItems(append(currentItems, msg.items...))
		}
		if msg.done {
			m.loading = false
			m.objectList.StopSpinner()
			return m, nil
		}
		// 继续等待下一条
		return m, waitForNextItem(m.loadingCh)

	case objectInfoLoadedMsg:
		if msg.err == nil {
			m.selectedObject = msg.info
		}
		return m, nil

	case operationCompleteMsg:
		result := "success"
		if msg.err != nil {
			result = msg.err.Error()
		}
		m.AddHistory(msg.operation, msg.object, result)
		return m, nil

	case contextSwitchedMsg:
		if msg.err != nil {
			m.AddHistory("use", msg.name, "error: "+msg.err.Error())
			return m, nil
		}
		m.applyClient(msg.newClient, msg.newCore, msg.name)
		m.AddHistory("use", msg.name, "switched (persist="+boolStr(msg.persist)+")")
		// 回到根目录并刷新桶列表
		m.currentPath = "/"
		m.currentBucket = ""
		m.currentPrefix = ""
		return m, loadRootCmd(m.lister)
	}

	// 更新子组件
	if m.inputFocused {
		m.input, _ = m.input.Update(msg)
	}

	m.objectList, _ = m.objectList.Update(msg)

	return m, tea.Batch(cmds...)
}

// View 实现 tea.Model
func (m Model) View() tea.View {
	var b strings.Builder

	// 标题栏
	title := m.renderTitle()
	b.WriteString(title)
	b.WriteString("\n")

	// 主内容区
	switch m.viewState {
	case ViewStateObjectList:
		b.WriteString(m.renderSplitView())
	case ViewStateHistory:
		b.WriteString(m.renderHistoryView())
	}
	b.WriteString("\n")

	// 进度条（如果有活动任务）
	if m.progress.Active {
		b.WriteString(m.renderProgressBar())
		b.WriteString("\n")
	}

	// 上分隔线
	b.WriteString(m.renderSeparator())
	b.WriteString("\n")

	// 命令输入栏
	b.WriteString(m.renderCommandInput())
	b.WriteString("\n")

	// 下分隔线
	b.WriteString(m.renderSeparator())
	b.WriteString("\n")

	// 状态栏
	b.WriteString(m.renderStatusBar())

	return tea.NewView(b.String())
}

// handleKeyPress 处理按键
func (m *Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// 命令输入模式
	if m.inputFocused {
		switch msg.Code {
		case tea.KeyEnter:
			cmd := m.input.Value()
			m.input.SetValue("")
			m.inputFocused = false
			return m.executeCommand(cmd)
		case tea.KeyEsc:
			m.inputFocused = false
			m.input.SetValue("")
			return m, nil
		}
		m.input, _ = m.input.Update(msg)
		return m, nil
	}

	// Filter 输入模式
	if m.filterMode {
		switch msg.Code {
		case tea.KeyEnter:
			// 确认过滤，退出 filter 模式
			m.filterMode = false
			return m, nil
		case tea.KeyUp, tea.KeyDown:
			// 导航键传递给列表组件
			var cmd tea.Cmd
			m.objectList, cmd = m.objectList.Update(msg)
			return m, cmd
		case tea.KeyEsc:
			// 取消过滤，恢复原始列表
			m.filterMode = false
			m.input.SetValue("")
			m.restoreOriginalList()
			return m, nil
		case tea.KeyBackspace:
			// 正常删除字符
			currentValue := m.input.Value()
			if len(currentValue) > 0 {
				m.input.SetValue(currentValue[:len(currentValue)-1])
				m.applyFilter(m.input.Value())
			} else {
				// 空时退出 filter 模式
				m.filterMode = false
				m.restoreOriginalList()
			}
			return m, nil
		default:
			// 其他字符输入
			if msg.Text != "" {
				m.input, _ = m.input.Update(msg)
				m.applyFilter(m.input.Value())
			}
			return m, nil
		}
	}

	// 正常导航模式 - 处理自定义快捷键
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		// 忽略 ESC，不退出程序
		return m, nil
	case ":":
		m.inputFocused = true
		m.input.Prompt = ": "
		m.input.Focus()
		m.input.SetValue("")
		return m, nil
	case "/":
		// 进入 filter 模式
		m.filterMode = true
		m.input.Prompt = "/ "
		m.input.Focus()
		m.input.SetValue("")
		// 保存原始列表
		m.saveOriginalList()
		return m, nil
	case "enter":
		return m.handleSelect()
	case "backspace":
		return m.navigateBack()
	case "r":
		return m.refresh()
	case "h":
		m.viewState = ViewStateHistory
		return m, nil
	case "u":
		// 提示用户输入 context 名（通过命令模式）
		m.inputFocused = true
		m.input.Prompt = ": "
		m.input.Focus()
		m.input.SetValue("use ")
		return m, nil
	case "tab":
		if m.focusState == FocusList {
			m.focusState = FocusDetail
		} else {
			m.focusState = FocusList
		}
		return m, nil
	}

	// 其他按键（包括上下箭头）传递给列表组件处理
	var cmd tea.Cmd
	m.objectList, cmd = m.objectList.Update(msg)
	return m, cmd
}

// handleSelect 处理选择
func (m *Model) handleSelect() (tea.Model, tea.Cmd) {
	if item, ok := m.objectList.SelectedItem().(pathItem); ok {
		if item.isBucket {
			// 选择桶，进入桶根目录
			return m.startStreamingLoad(item.name, "")
		}
		if item.isDir {
			// 选择目录，进入子目录
			return m.startStreamingLoad(m.getBucketFromPath(), item.name)
		}
		// 选择文件，加载详情
		return m, loadObjectInfoCmd(m.stater, m.getBucketFromPath(), item.name)
	}
	return m, nil
}

// navigateBack 返回上一级
func (m *Model) navigateBack() (tea.Model, tea.Cmd) {
	switch m.viewState {
	case ViewStateObjectList:
		if m.currentPath == "/" {
			// 已经在根目录，不做任何操作
			return m, nil
		}
		if m.currentPrefix != "" {
			// 在桶内子目录，返回上级目录
			parts := strings.Split(strings.TrimSuffix(m.currentPrefix, "/"), "/")
			if len(parts) > 1 {
				m.currentPrefix = strings.Join(parts[:len(parts)-1], "/") + "/"
			} else {
				m.currentPrefix = ""
			}
			return m.startStreamingLoad(m.currentBucket, m.currentPrefix)
		}
		// 在桶根目录，返回根目录（桶列表）
		return m, loadRootCmd(m.lister)
	case ViewStateHistory:
		m.viewState = ViewStateObjectList
		return m, nil
	}
	return m, nil
}

// saveOriginalList 保存原始列表数据用于过滤
func (m *Model) saveOriginalList() {
	m.originalObjectItems = m.objectList.Items()
}

// restoreOriginalList 恢复原始列表数据
func (m *Model) restoreOriginalList() {
	if len(m.originalObjectItems) > 0 {
		m.objectList.SetItems(m.originalObjectItems)
	}
}

// applyFilter 应用过滤
func (m *Model) applyFilter(filterText string) {
	filterText = strings.ToLower(filterText)
	if len(m.originalObjectItems) == 0 {
		return
	}
	var filtered []list.Item
	for _, item := range m.originalObjectItems {
		if pi, ok := item.(pathItem); ok {
			if strings.Contains(strings.ToLower(pi.name), filterText) {
				filtered = append(filtered, item)
			}
		}
	}
	m.objectList.SetItems(filtered)
}

// refresh 刷新当前列表
func (m *Model) refresh() (tea.Model, tea.Cmd) {
	if m.currentPath == "/" {
		return m, loadRootCmd(m.lister)
	}
	return m.startStreamingLoad(m.currentBucket, m.currentPrefix)
}

// executeCommand 执行命令
func (m *Model) executeCommand(cmd string) (tea.Model, tea.Cmd) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return m, nil
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return m, nil
	}

	switch parts[0] {
	case "cd":
		if len(parts) < 2 {
			return m, nil
		}
		target := parts[1]
		switch target {
		case "/", "..":
			// 返回根目录或上级
			return m.navigateBack()
		default:
			// cd 到指定路径
			if strings.Contains(target, "/") {
				// 完整路径：bucket/prefix
				p := strings.SplitN(target, "/", 2)
				bucket := p[0]
				prefix := ""
				if len(p) > 1 {
					prefix = p[1]
					if !strings.HasSuffix(prefix, "/") {
						prefix += "/"
					}
				}
				return m.startStreamingLoad(bucket, prefix)
			}
			// 相对路径：可能是桶名或目录名
			if m.currentPath == "/" {
				// 当前在根目录，target 是桶名
				return m.startStreamingLoad(target, "")
			}
			// 在桶内，target 是目录
			prefix := target
			if !strings.HasSuffix(prefix, "/") {
				prefix += "/"
			}
			if m.currentPrefix != "" {
				prefix = m.currentPrefix + prefix
			}
			return m.startStreamingLoad(m.currentBucket, prefix)
		}

	case "ls", "list":
		if m.currentPath == "/" {
			return m, nil // 根目录不能 ls
		}
		return m.startStreamingLoad(m.currentBucket, m.currentPrefix)

	case "get":
		if len(parts) < 2 || m.currentPath == "/" {
			return m, nil
		}
		objectName := parts[1]
		if m.currentPrefix != "" && !strings.Contains(objectName, "/") {
			objectName = m.currentPrefix + objectName
		}
		return m, m.downloadObject(objectName)

	case "put":
		if len(parts) < 2 || m.currentPath == "/" {
			return m, nil
		}
		localPath := parts[1]
		objectName := ""
		if len(parts) > 2 {
			objectName = parts[2]
		}
		return m, m.uploadObject(localPath, objectName)

	case "sign":
		if len(parts) < 2 || m.currentPath == "/" {
			return m, nil
		}
		objectName := parts[1]
		if m.currentPrefix != "" && !strings.Contains(objectName, "/") {
			objectName = m.currentPrefix + objectName
		}
		return m, m.signObject(objectName)

	case "history":
		m.viewState = ViewStateHistory
		return m, nil

	case "exit", "quit":
		return m, tea.Quit

	case "use":
		if len(parts) < 2 {
			return m, nil
		}
		return m, m.switchContextCmd(parts[1], false)

	case "set-default":
		if len(parts) < 2 {
			return m, nil
		}
		return m, m.switchContextCmd(parts[1], true)
	}

	return m, nil
}

// getBucketFromPath 从当前路径提取桶名
func (m *Model) getBucketFromPath() string {
	return m.currentBucket
}

// 命令加载函数
func loadRootCmd(lister *operations.Lister) tea.Cmd {
	return func() tea.Msg {
		buckets, err := lister.ListBuckets(context.Background())
		if err != nil {
			return listStreamMsg{err: err, done: true}
		}
		items := make([]list.Item, len(buckets))
		for i, b := range buckets {
			items[i] = pathItem{
				name:     b.Name,
				isBucket: true,
				isDir:    true,
				created:  b.CreationDate,
			}
		}
		return listStreamMsg{items: items, path: "/", bucket: "", prefix: "", first: true, done: true}
	}
}

// startStreamingLoad 开始流式加载目录内容
func (m *Model) startStreamingLoad(bucket, prefix string) (tea.Model, tea.Cmd) {
	m.objectList.SetItems(nil)
	m.loading = true
	m.loadingCh = make(chan listStreamMsg, 100)

	path := bucket
	if prefix != "" {
		path = bucket + "/" + prefix
	}
	m.objectList.Title = path
	if path == "/" {
		m.objectList.Title = "根目录"
	}

	return m, tea.Batch(
		m.objectList.StartSpinner(),
		loadObjectsStreamingCmd(m.lister, bucket, prefix, m.loadingCh),
	)
}

// loadObjectsStreamingCmd 流式加载对象列表
func loadObjectsStreamingCmd(lister *operations.Lister, bucket, prefix string, ch chan listStreamMsg) tea.Cmd {
	go func() {
		var batch []list.Item
		batchCount := 0
		totalCount := 0

		resultCh := lister.ListObjectsStream(context.Background(), bucket, prefix, false)
		for result := range resultCh {
			if result.Err != nil {
				path := bucket
				if prefix != "" {
					path = bucket + "/" + prefix
				}
				ch <- listStreamMsg{done: true, err: result.Err, path: path, bucket: bucket, prefix: prefix}
				return
			}

			obj := result.Object
			item := pathItem{
				name:         obj.Key,
				isBucket:     false,
				isDir:        obj.IsDir,
				size:         obj.Size,
				lastModified: obj.LastModified,
			}
			batch = append(batch, item)
			batchCount++
			totalCount++

			// 第1个立即发，之后每20个发一批
			if batchCount == 1 || batchCount >= 20 {
				path := bucket
				if prefix != "" {
					path = bucket + "/" + prefix
				}
				ch <- listStreamMsg{
					items:  batch,
					path:   path,
					bucket: bucket,
					prefix: prefix,
					first:  totalCount == 1,
				}
				batch = nil
				batchCount = 0
			}
		}

		// 发送剩余的
		if len(batch) > 0 {
			path := bucket
			if prefix != "" {
				path = bucket + "/" + prefix
			}
			ch <- listStreamMsg{
				items:  batch,
				path:   path,
				bucket: bucket,
				prefix: prefix,
			}
		}

		// 发送完成消息
		path := bucket
		if prefix != "" {
			path = bucket + "/" + prefix
		}
		ch <- listStreamMsg{done: true, path: path, bucket: bucket, prefix: prefix}
	}()

	// 返回第一个 Cmd：从 channel 读第一条消息
	return func() tea.Msg {
		return <-ch
	}
}

// waitForNextItem 等待下一条流式消息
func waitForNextItem(ch chan listStreamMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func loadObjectInfoCmd(stater *operations.Stater, bucket, object string) tea.Cmd {
	return func() tea.Msg {
		info, err := stater.GetObjectInfo(context.Background(), bucket, object)
		return objectInfoLoadedMsg{info: info, err: err}
	}
}

// 操作方法
func (m *Model) downloadObject(objectName string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.getter.DownloadObject(context.Background(), m.currentBucket, objectName, "", nil)
		return operationCompleteMsg{operation: "get", object: objectName, err: err}
	}
}

func (m *Model) switchContextCmd(name string, persist bool) tea.Cmd {
	return func() tea.Msg {
		if m.onContextChange == nil {
			return contextSwitchedMsg{name: name, err: fmt.Errorf("context 切换回调未注册")}
		}
		if persist {
			// 落盘：调用外部注册的全局 hook（如 main 包）以修改 current-context
			if PersistCurrentContext != nil {
				if err := PersistCurrentContext(name); err != nil {
					return contextSwitchedMsg{name: name, persist: persist, err: err}
				}
			}
		}
		newClient, newCore, err := m.onContextChange(name)
		return contextSwitchedMsg{
			name:      name,
			newClient: newClient,
			newCore:   newCore,
			persist:   persist,
			err:       err,
		}
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func (m *Model) uploadObject(localPath, objectName string) tea.Cmd {
	return func() tea.Msg {
		if objectName == "" {
			objectName = localPath
		}
		_, err := m.putter.UploadObject(context.Background(), m.currentBucket, objectName, localPath, "", nil)
		return operationCompleteMsg{operation: "put", object: objectName, err: err}
	}
}

func (m *Model) signObject(objectName string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.signer.PresignURL(context.Background(), m.currentBucket, objectName, 0)
		return operationCompleteMsg{operation: "sign", object: objectName, err: err}
	}
}

// 渲染方法
func (m Model) renderSeparator() string {
	return separatorStyle.Render(strings.Repeat("─", m.width))
}

func (m Model) renderTitle() string {
	var title string
	if m.contextName != "" {
		if m.currentPath != "/" && m.currentPath != "" {
			title = fmt.Sprintf("S3M Explorer [ctx: %s] - %s", m.contextName, m.currentPath)
		} else {
			title = fmt.Sprintf("S3M Explorer [ctx: %s]", m.contextName)
		}
	} else if m.currentPath != "/" && m.currentPath != "" {
		title = fmt.Sprintf("S3M Explorer - %s", m.currentPath)
	} else {
		title = "S3M Explorer"
	}
	return titleStyle.Render(title)
}

func (m Model) renderSplitView() string {
	listWidth := m.width * 60 / 100
	detailWidth := m.width - listWidth - 3

	listView := m.objectList.View()
	detailView := m.renderDetailPanel(detailWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(listWidth).Render(listView),
		lipgloss.NewStyle().Width(1).Render("│"),
		lipgloss.NewStyle().Width(detailWidth).Render(detailView),
	)
}

func (m Model) renderDetailPanel(width int) string {
	var b strings.Builder

	b.WriteString(detailTitleStyle.Render("详情"))
	b.WriteString("\n\n")

	if m.selectedObject != nil {
		info := m.selectedObject
		b.WriteString(fmt.Sprintf("名称: %s\n", info.Key))
		b.WriteString(fmt.Sprintf("大小: %s\n", formatSize(info.Size)))
		b.WriteString(fmt.Sprintf("修改: %s\n", info.LastModified.Format("2006-01-02 15:04:05")))
		b.WriteString(fmt.Sprintf("ETag: %s\n", info.ETag))
		b.WriteString(fmt.Sprintf("类型: %s\n", info.ContentType))
	} else {
		b.WriteString("选择项目查看详情")
	}

	return b.String()
}

func (m Model) renderProgressBar() string {
	percent := float64(m.progress.DoneBytes) / float64(m.progress.TotalBytes) * 100
	bar := fmt.Sprintf("%.0f%% %s (%s/%s)",
		percent,
		m.progress.ObjectName,
		formatSize(m.progress.DoneBytes),
		formatSize(m.progress.TotalBytes),
	)
	return progressStyle.Render(bar)
}

func (m Model) renderCommandInput() string {
	// 确定模式标签
	var modeLabel string
	if m.filterMode {
		modeLabel = filterModeStyle.Render(" 过滤 ")
	} else if m.inputFocused {
		modeLabel = commandModeStyle.Render(" 命令 ")
	} else {
		modeLabel = waitingModeStyle.Render(" 等待 ")
	}

	// 输入内容（textinput.View() 会包含 Prompt）
	var inputContent string
	if m.inputFocused || m.filterMode {
		inputContent = inputTextStyle.Render(m.input.View())
	} else {
		inputContent = placeholderStyle.Render(m.input.Placeholder)
	}

	// 组合
	line := lipgloss.JoinHorizontal(lipgloss.Left,
		modeLabel,
		" ",
		inputContent,
	)

	return inputBoxStyle.Width(m.width).Render(line)
}

func (m Model) renderStatusBar() string {
	left := "[↑↓]导航 [Enter]选择 [Backspace]返回 [/]过滤 [:]命令 [u]切换ctx [r]刷新 [h]历史 [q]退出"
	right := fmt.Sprintf("历史: %d 条", len(m.history))
	return statusBarStyle.Render(
		lipgloss.JoinHorizontal(lipgloss.Left, left, " | ", right),
	)
}

func (m Model) renderHistoryView() string {
	var b strings.Builder
	b.WriteString("操作历史\n\n")
	if len(m.history) == 0 {
		b.WriteString("暂无历史记录")
	} else {
		for _, h := range m.history {
			b.WriteString(fmt.Sprintf("%s %s %s: %s\n",
				h.Timestamp.Format("15:04:05"),
				h.Operation,
				h.Object,
				h.Result,
			))
		}
	}
	return b.String()
}

// 辅助函数
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// pathItem 统一的列表项类型（桶、目录、文件）
type pathItem struct {
	name         string
	isBucket     bool
	isDir        bool
	size         int64
	lastModified time.Time
	created      time.Time // 仅桶使用
}

func (i pathItem) FilterValue() string { return i.name }
func (i pathItem) Title() string {
	if i.isBucket {
		return "[桶] " + i.name
	}
	if i.isDir {
		return "[目录] " + i.name
	}
	return "[文件] " + i.name
}
func (i pathItem) Description() string {
	if i.isBucket {
		return i.created.Format("2006-01-02 15:04:05")
	}
	if i.isDir {
		return ""
	}
	return fmt.Sprintf("%s | %s",
		formatSize(i.size),
		i.lastModified.Format("2006-01-02 15:04:05"))
}
