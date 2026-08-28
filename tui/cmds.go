package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"s3m/operations"

	tea "charm.land/bubbletea/v2"
)

// 预览约束（spec 第八节大文件策略）
const (
	previewMaxBytes      = 64 * 1024        // 首屏最多读取 64KB
	previewMaxLines      = 1000             // 最多展示行数
	previewWarnSize      = 1024 * 1024      // > 1MB 提示确认
	previewRejectSize    = 10 * 1024 * 1024 // > 10MB 拒绝，建议 cat
	listStreamBatchSize  = 20               // 流式加载批大小
	uploadDirConcurrency = 4                // 目录上传并发数
)

// loadBucketsCmd 加载桶列表
func loadBucketsCmd(lister *operations.Lister) tea.Cmd {
	return func() tea.Msg {
		buckets, err := lister.ListBuckets(context.Background())
		return bucketsLoadedMsg{buckets: buckets, err: err}
	}
}

// loadObjectsCmd 流式加载对象列表（复用 operations.ListObjectsStream）。
// 结果写入 ch；stop 关闭时中止写入并退出；gen 用于丢弃过期批次。
func loadObjectsCmd(lister *operations.Lister, bucket, prefix string, gen int, ch chan listStreamMsg, stop chan struct{}) tea.Cmd {
	return func() tea.Msg {
		go func() {
			send := func(msg listStreamMsg) bool {
				select {
				case ch <- msg:
					return true
				case <-stop:
					return false
				}
			}

			resultCh := lister.ListObjectsStream(context.Background(), bucket, prefix, false)
			var batch []entry
			total := 0

			for result := range resultCh {
				if result.Err != nil {
					send(listStreamMsg{gen: gen, done: true, err: result.Err})
					return
				}
				batch = append(batch, newEntryFromObject(result.Object, prefix))
				total++
				if total == 1 || len(batch) >= listStreamBatchSize {
					if !send(listStreamMsg{gen: gen, items: batch, first: total == 1}) {
						return
					}
					batch = nil
				}
			}
			if len(batch) > 0 {
				if !send(listStreamMsg{gen: gen, items: batch}) {
					return
				}
			}
			send(listStreamMsg{gen: gen, done: true})
		}()
		return <-ch
	}
}

// newEntryFromObject 将流式对象转换为面板条目（显示名去掉当前前缀）
func newEntryFromObject(obj operations.ObjectInfo, prefix string) entry {
	name := strings.TrimPrefix(obj.Key, prefix)
	if obj.IsDir {
		name = strings.TrimSuffix(name, "/")
	}
	return entry{
		key:      obj.Key,
		name:     name,
		isDir:    obj.IsDir,
		size:     obj.Size,
		modified: obj.LastModified,
	}
}

// sortEntries 排序：返回行最前，目录其次，文件最后；同类按名称
func sortEntries(items []entry) {
	sort.SliceStable(items, func(i, j int) bool { return entryLess(items[i], items[j]) })
}

// entryLess 统一排序比较器（back < 目录 < 文件，同类按名称）
func entryLess(a, b entry) bool {
	if a.isBack != b.isBack {
		return a.isBack
	}
	if a.isDir != b.isDir {
		return a.isDir
	}
	return a.name < b.name
}

// statObjectCmd 查询对象 Meta
func statObjectCmd(stater *operations.Stater, bucket, key string) tea.Cmd {
	return func() tea.Msg {
		info, err := stater.GetObjectInfo(context.Background(), bucket, key)
		return objectInfoLoadedMsg{info: info, err: err}
	}
}

// loadPreviewCmd 加载文本预览内容（限量读取，不整文件载入内存）
func loadPreviewCmd(getter *operations.Getter, bucket, key string) tea.Cmd {
	return func() tea.Msg {
		rc, info, err := getter.ReadObject(context.Background(), bucket, key)
		if err != nil {
			return previewLoadedMsg{key: key, err: err}
		}
		defer rc.Close()

		buf := make([]byte, previewMaxBytes)
		n, rerr := io.ReadFull(rc, buf)
		if rerr != nil && rerr != io.ErrUnexpectedEOF && rerr != io.EOF {
			return previewLoadedMsg{key: key, err: rerr}
		}
		buf = buf[:n]

		lines, truncated := previewLines(buf, info.Size)
		return previewLoadedMsg{key: key, lines: lines, size: info.Size, truncated: truncated}
	}
}

func transferCmd(ch chan tea.Msg, op, object string, run func(progressCb operations.ProgressCallback) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			var lastDone int64 = -1
			progressCb := func(doneBytes, totalBytes int64) {
				if lastDone >= 0 && doneBytes != totalBytes && doneBytes-lastDone < 64*1024 {
					return
				}
				lastDone = doneBytes
				ch <- transferProgressMsg{op: op, object: object, doneBytes: doneBytes, totalBytes: totalBytes}
			}
			ch <- run(progressCb)
		}()
		return <-ch
	}
}

// previewLines 将预览字节切分为受限行集
func previewLines(data []byte, totalSize int64) (lines []string, truncated bool) {
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if len(lines) >= previewMaxLines {
			truncated = true
			break
		}
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		lines = append(lines, "… (内容包含无法解码的字节，已截断)")
		truncated = true
	}
	if int64(len(data)) < totalSize {
		truncated = true
	}
	return lines, truncated
}

// uploadCmd 上传单个文件
func uploadCmd(putter *operations.Putter, bucket, key, localPath string, ch chan tea.Msg) tea.Cmd {
	object := bucket + "/" + key
	return transferCmd(ch, "put", object, func(progressCb operations.ProgressCallback) tea.Msg {
		_, err := putter.UploadObject(context.Background(), bucket, key, localPath, "", progressCb)
		return operationCompleteMsg{op: "put", object: object, refresh: true, err: err}
	})
}

// uploadDirCmd 递归上传目录
func uploadDirCmd(putter *operations.Putter, bucket, prefix, localDir string) tea.Cmd {
	object := bucket + "/" + prefix
	return func() tea.Msg {
		res, err := putter.UploadDirectory(context.Background(), bucket, prefix, localDir, uploadDirConcurrency)
		if err != nil {
			return operationCompleteMsg{op: "put-dir", object: object, refresh: true, err: err}
		}
		details := fmt.Sprintf("成功 %d, 失败 %d", res.Success, res.Failed)
		var rerr error
		if res.Failed > 0 {
			rerr = fmt.Errorf("%d 个文件上传失败", res.Failed)
		}
		return operationCompleteMsg{op: "put-dir", object: object, details: details, refresh: true, err: rerr}
	}
}

// downloadCmd 下载单个对象
func downloadCmd(getter *operations.Getter, bucket, key, localPath string, ch chan tea.Msg) tea.Cmd {
	object := bucket + "/" + key
	return transferCmd(ch, "get", object, func(progressCb operations.ProgressCallback) tea.Msg {
		_, err := getter.DownloadObject(context.Background(), bucket, key, localPath, progressCb)
		return operationCompleteMsg{op: "get", object: object, err: err}
	})
}

// downloadDirCmd 递归下载目录
func downloadDirCmd(getter *operations.Getter, bucket, prefix, localDir string) tea.Cmd {
	object := bucket + "/" + prefix
	return func() tea.Msg {
		res, err := getter.DownloadDirectory(context.Background(), bucket, prefix, localDir, 0)
		if err != nil {
			return operationCompleteMsg{op: "get-dir", object: object, err: err}
		}
		details := fmt.Sprintf("成功 %d, 失败 %d", res.Success, res.Failed)
		var rerr error
		if res.Failed > 0 {
			rerr = fmt.Errorf("%d 个对象下载失败", res.Failed)
		}
		return operationCompleteMsg{op: "get-dir", object: object, details: details, err: rerr}
	}
}

// deleteCmd 删除单个对象
func deleteCmd(deleter *operations.Deleter, bucket, key string) tea.Cmd {
	object := bucket + "/" + key
	return func() tea.Msg {
		err := deleter.DeleteObject(context.Background(), bucket, key)
		return operationCompleteMsg{op: "del", object: object, refresh: true, err: err}
	}
}

// deleteDirCmd 递归删除目录
func deleteDirCmd(deleter *operations.Deleter, bucket, prefix string) tea.Cmd {
	object := bucket + "/" + prefix
	return func() tea.Msg {
		count, errs := deleter.DeleteDirectoryObjects(context.Background(), bucket, prefix, 0)
		var rerr error
		if len(errs) > 0 {
			rerr = fmt.Errorf("%d 个对象删除失败: %s", len(errs), errs[0].Error)
		}
		return operationCompleteMsg{
			op: "del-dir", object: object,
			details: fmt.Sprintf("已删除 %d 个对象", count),
			refresh: true, err: rerr,
		}
	}
}

// deleteBatchCmd 批量删除（目录递归、文件逐个）
func deleteBatchCmd(deleter *operations.Deleter, bucket string, targets []entry) tea.Cmd {
	object := fmt.Sprintf("%s 中 %d 个对象", bucket, len(targets))
	return func() tea.Msg {
		var failed int
		var firstErr string
		for _, t := range targets {
			var err error
			if t.isDir {
				_, errs := deleter.DeleteDirectoryObjects(context.Background(), bucket, t.key, 0)
				if len(errs) > 0 {
					err = fmt.Errorf("%s: %s", errs[0].Key, errs[0].Error)
				}
			} else {
				err = deleter.DeleteObject(context.Background(), bucket, t.key)
			}
			if err != nil {
				failed++
				if firstErr == "" {
					firstErr = err.Error()
				}
			}
		}
		var rerr error
		if failed > 0 {
			rerr = fmt.Errorf("%d 个失败: %s", failed, firstErr)
		}
		return operationCompleteMsg{
			op: "del", object: object,
			details: fmt.Sprintf("共 %d 个目标", len(targets)),
			refresh: true, err: rerr,
		}
	}
}

// signCmd 生成预签名 URL（命令模式使用）
func signCmd(signer *operations.Signer, bucket, key string) tea.Cmd {
	object := bucket + "/" + key
	return func() tea.Msg {
		_, err := signer.PresignURL(context.Background(), bucket, key, 0)
		return operationCompleteMsg{op: "sign", object: object, err: err}
	}
}

// switchContextCmd 切换 context（复用外部回调）
func switchContextCmd(m *Model, name string, persist bool) tea.Cmd {
	return func() tea.Msg {
		if m.onContextChange == nil {
			return contextSwitchedMsg{name: name, persist: persist, err: fmt.Errorf("context 切换回调未注册")}
		}
		if persist && m.readOnly {
			return contextSwitchedMsg{name: name, persist: persist, err: fmt.Errorf("conf 为只读模式，不允许 set-default")}
		}
		if persist && PersistCurrentContext != nil {
			if err := PersistCurrentContext(name); err != nil {
				return contextSwitchedMsg{name: name, persist: persist, err: err}
			}
		}
		newClient, newCore, err := m.onContextChange(name)
		return contextSwitchedMsg{name: name, newClient: newClient, newCore: newCore, persist: persist, err: err}
	}
}

// objectKeyOf 拼接对象完整 key
func objectKeyOf(prefix, name string) string {
	return prefix + name
}

// localUploadTarget 计算上传到当前前缀后的对象 key
func localUploadTarget(prefix, localPath string) string {
	return objectKeyOf(prefix, filepath.Base(localPath))
}
