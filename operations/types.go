package operations

import "time"

// ObjectInfo 对象信息
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	ContentType  string
	IsDir        bool // 是否为"目录"（S3 实际上没有目录概念，通过前缀模拟）

	// 以下字段仅 StatObject 查询时填充
	StorageClass string              // 存储类型（如 STANDARD）
	Metadata     map[string][]string // 响应头元数据
}

// ListResult 列表结果
type ListResult struct {
	Objects  []ObjectInfo
	Prefixes []string // 子目录前缀
}

// UploadResult 上传结果
type UploadResult struct {
	Bucket    string
	Key       string
	Size      int64
	ETag      string
	VersionID string
}

// BucketInfo 存储桶信息
type BucketInfo struct {
	Name         string
	CreationDate time.Time
}

// DownloadResult 下载结果
type DownloadResult struct {
	Bucket      string
	Key         string
	LocalPath   string
	Size        int64
	ContentType string
}

// SignResult 签名结果
type SignResult struct {
	Bucket    string
	Key       string
	URL       string
	ExpiresAt time.Time
}

// CopyResult 复制结果
type CopyResult struct {
	SrcBucket string
	SrcKey    string
	DstBucket string
	DstKey    string
	Size      int64
	ETag      string
}

// BatchCopyResult 批量复制结果
type BatchCopyResult struct {
	SrcBucket  string
	DestBucket string
	SrcPrefix  string
	DestPrefix string
	Success    int
	Failed     int
	Results    []CopyResult
	Errors     []CopyError
}

// CopyError 复制错误
type CopyError struct {
	SrcKey string
	DstKey string
	Error  string
}

// ProgressCallback 进度回调函数
type ProgressCallback func(doneBytes, totalBytes int64)

// DeleteError 删除错误
type DeleteError struct {
	Key   string
	Error string
}

// BatchDownloadResult 批量下载结果
type BatchDownloadResult struct {
	Bucket   string
	Prefix   string
	LocalDir string
	Success  int
	Failed   int
	Results  []DownloadResult
	Errors   []DownloadError
}

// DownloadError 下载错误
type DownloadError struct {
	Key       string
	LocalPath string
	Error     string
}

// BatchUploadResult 批量上传结果
type BatchUploadResult struct {
	Bucket   string
	Prefix   string
	LocalDir string
	Success  int
	Failed   int
	Results  []UploadResult
	Errors   []UploadError
}

// UploadError 上传错误
type UploadError struct {
	LocalPath string
	Key       string
	Error     string
}
