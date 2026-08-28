package tui

import (
	"github.com/minio/minio-go/v7"
	"s3m/operations"
)

// bucketsLoadedMsg 桶列表加载完成
type bucketsLoadedMsg struct {
	buckets []operations.BucketInfo
	err     error
}

// listStreamMsg 对象列表流式批次消息
type listStreamMsg struct {
	gen    int // 加载代次（丢弃过期批次）
	items  []entry
	bucket string
	prefix string
	first  bool // 首批（清空旧列表）
	done   bool // 全部完成
	err    error
}

// objectInfoLoadedMsg Meta 信息加载完成
type objectInfoLoadedMsg struct {
	info *operations.ObjectInfo
	err  error
}

// previewLoadedMsg 预览内容加载完成
type previewLoadedMsg struct {
	key       string
	lines     []string
	size      int64
	truncated bool // 超过行数/字节上限被截断
	err       error
}

// operationCompleteMsg 异步操作（上传/下载/删除/签名/复制）完成
type operationCompleteMsg struct {
	op      string // get/put/put-dir/get-dir/del/del-dir/sign/clip
	object  string
	details string // 补充说明（如批量统计）
	refresh bool   // 完成后是否刷新对象列表
	err     error
}

// transferProgressMsg 上传/下载进行中的字节进度
type transferProgressMsg struct {
	op         string
	object     string
	doneBytes  int64
	totalBytes int64
}

// contextSwitchedMsg context 切换完成
type contextSwitchedMsg struct {
	name      string
	newClient *minio.Client
	newCore   *minio.Core
	persist   bool
	err       error
}
