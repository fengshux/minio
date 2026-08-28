package tui

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// executeCommand 执行 ':' 命令模式命令
func (m *Model) executeCommand(cmdline string) (tea.Model, tea.Cmd) {
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "" {
		return m, nil
	}
	parts := strings.Fields(cmdline)

	switch parts[0] {
	case "cd":
		return m.commandCd(parts)

	case "ls", "list":
		if m.inBucket() {
			return m, m.startObjectLoad()
		}
		m.setStatus(statusNone, "根目录无对象列表，请先 cd <bucket> 或在桶面板选择")

	case "get":
		return m.commandGet(parts)

	case "put":
		return m.commandPut(parts)

	case "sign":
		return m.commandSign(parts)

	case "use":
		if len(parts) < 2 {
			m.setStatus(statusErr, "用法: use <context-name>")
			return m, nil
		}
		return m, switchContextCmd(m, parts[1], false)

	case "set-default":
		if len(parts) < 2 {
			m.setStatus(statusErr, "用法: set-default <context-name>")
			return m, nil
		}
		return m, switchContextCmd(m, parts[1], true)

	case "history":
		m.activeModal = newHistoryModal(m.history)

	case "help":
		m.activeModal = newHelpModal()

	case "exit", "quit", "q":
		return m, tea.Quit

	default:
		m.setStatus(statusErr, "未知命令: %s（可用: cd/ls/get/put/sign/use/history/exit）", parts[0])
	}
	return m, nil
}

// commandCd cd 命令：/ 回根、.. 上级、bucket/prefix 任意跳转
func (m *Model) commandCd(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		return m, nil
	}
	target := strings.TrimSuffix(parts[1], "/")

	switch target {
	case "", "/", "..":
		return m, m.goUp()
	}

	if !strings.Contains(target, "/") {
		// 单段：根目录下是桶名；桶内是子目录
		if !m.inBucket() {
			m.focus = FocusObjects
			return m, m.openBucket(target)
		}
		prefix := m.currentPrefix + target + "/"
		return m.navigateWithinBucket(prefix)
	}

	// 多段：bucket/prefix
	p := strings.SplitN(target, "/", 2)
	if p[0] == m.currentBucket && m.inBucket() {
		return m.navigateWithinBucket(m.currentPrefix + p[1] + "/")
	}
	m.focus = FocusObjects
	return m.openBucketPrefix(p[0], p[1]+"/")
}

// navigateWithinBucket 桶内跳转前缀
func (m *Model) navigateWithinBucket(prefix string) (tea.Model, tea.Cmd) {
	m.currentPrefix = prefix
	return m, m.startObjectLoadReset()
}

// openBucketPrefix 进入指定桶和前缀
func (m *Model) openBucketPrefix(bucket, prefix string) (tea.Model, tea.Cmd) {
	m.currentBucket = bucket
	m.currentPrefix = prefix
	return m, m.startObjectLoadReset()
}

// currentPath 当前路径描述（根目录为 /）
func (m *Model) currentPath() string {
	if !m.inBucket() {
		return "/"
	}
	if m.currentPrefix == "" {
		return m.currentBucket
	}
	return m.currentBucket + "/" + strings.TrimSuffix(m.currentPrefix, "/")
}

// commandGet get 命令：下载对象到当前目录
func (m *Model) commandGet(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 || !m.inBucket() {
		m.setStatus(statusErr, "用法: get <object>（需在桶内）")
		return m, nil
	}
	objectName := m.resolveObjectArg(parts[1])
	local := baseName(objectName)

	m.transferCh = make(chan tea.Msg, 64)
	m.setBusy("get", baseName(objectName))
	return m, downloadCmd(m.getter, m.currentBucket, objectName, local, m.transferCh)
}

// commandPut put 命令：上传本地文件到当前前缀
func (m *Model) commandPut(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 || !m.inBucket() {
		m.setStatus(statusErr, "用法: put <local-file>（需在桶内）")
		return m, nil
	}
	localPath := parts[1]
	objectName := m.currentPrefix + baseName(localPath)
	if len(parts) > 2 {
		objectName = m.resolveObjectArg(parts[2])
	}

	m.transferCh = make(chan tea.Msg, 64)
	m.setBusy("put", baseName(localPath))
	return m, uploadCmd(m.putter, m.currentBucket, objectName, localPath, m.transferCh)
}

// commandSign sign 命令：生成预签名 URL
func (m *Model) commandSign(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 || !m.inBucket() {
		m.setStatus(statusErr, "用法: sign <object>（需在桶内）")
		return m, nil
	}
	objectName := m.resolveObjectArg(parts[1])
	m.setBusy("sign", baseName(objectName))
	return m, signCmd(m.signer, m.currentBucket, objectName)
}

// resolveObjectArg 解析对象参数：相对名补当前前缀，绝对名直传
func (m *Model) resolveObjectArg(name string) string {
	if strings.Contains(name, "/") {
		return strings.TrimPrefix(name, "/")
	}
	return m.currentPrefix + name
}

// baseName 取路径最后一段
func baseName(p string) string {
	return filepath.Base(strings.TrimSuffix(p, "/"))
}
