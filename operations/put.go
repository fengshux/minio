package operations

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
)

// Putter 对象上传操作
type Putter struct {
	client *minio.Client
}

// NewPutter 创建 Putter
func NewPutter(client *minio.Client) *Putter {
	return &Putter{client: client}
}

// UploadObject 上传本地文件到存储桶，返回结构化数据
// progressCb 可以为 nil
func (p *Putter) UploadObject(ctx context.Context, bucketName, objectName, localPath, contentType string, progressCb ProgressCallback) (*UploadResult, error) {
	// 检查本地文件是否存在
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在或无法访问: %w", err)
	}

	// 如果是目录，报错
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("不支持上传目录，请指定文件")
	}

	// 自动检测 Content-Type
	if contentType == "" {
		contentType = detectContentType(localPath)
	}

	// 打开本地文件
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	reader := newProgressReader(file, fileInfo.Size(), progressCb)

	// 上传对象
	uploadInfo, err := p.client.PutObject(ctx, bucketName, objectName, reader, fileInfo.Size(),
		minio.PutObjectOptions{
			ContentType: contentType,
		})
	if err != nil {
		return nil, fmt.Errorf("上传对象失败: %w", err)
	}

	if progressCb != nil && uploadInfo.Size >= fileInfo.Size() {
		progressCb(fileInfo.Size(), fileInfo.Size())
	}

	return &UploadResult{
		Bucket:    bucketName,
		Key:       objectName,
		Size:      uploadInfo.Size,
		ETag:      strings.Trim(uploadInfo.ETag, "\""),
		VersionID: uploadInfo.VersionID,
	}, nil
}

// Put 上传本地文件到存储桶（CLI 直接输出）
func (p *Putter) Put(ctx context.Context, bucketName, objectName, localPath, contentType string) {
	result, err := p.UploadObject(ctx, bucketName, objectName, localPath, contentType, nil)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("上传成功: %s -> %s/%s (大小: %d 字节, ETag: %s)\n",
		localPath, result.Bucket, result.Key, result.Size, result.ETag)
}

// UploadDirectory 递归上传本地目录到存储桶，返回结构化数据
// localDir 为本地目录路径，prefix 为存储桶中的对象前缀
// concurrent 参数指定并发数，为 0 时逐个上传
func (p *Putter) UploadDirectory(ctx context.Context, bucketName, prefix, localDir string, concurrent int) (*BatchUploadResult, error) {
	// 确保 prefix 以 / 结尾
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	// 验证本地目录
	dirInfo, err := os.Stat(localDir)
	if err != nil {
		return nil, fmt.Errorf("目录不存在或无法访问: %w", err)
	}
	if !dirInfo.IsDir() {
		return nil, fmt.Errorf("指定的路径不是目录: %s", localDir)
	}

	// 收集所有文件
	var files []string
	err = filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("遍历目录失败: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("目录为空: %s", localDir)
	}

	batchResult := &BatchUploadResult{
		Bucket:   bucketName,
		Prefix:   prefix,
		LocalDir: localDir,
	}

	bar := NewProgressBar(len(files), "上传中")

	if concurrent > 0 {
		// 并发上传
		var wg sync.WaitGroup
		var mu sync.Mutex
		sem := make(chan struct{}, concurrent)

		for _, filePath := range files {
			wg.Add(1)
			go func(fp string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				// 计算相对路径和对象名
				relPath, _ := filepath.Rel(localDir, fp)
				objectName := prefix + filepath.ToSlash(relPath)
				contentType := detectContentType(fp)

				result, err := p.UploadObject(ctx, bucketName, objectName, fp, contentType, nil)

				mu.Lock()
				if err != nil {
					batchResult.Failed++
					batchResult.Errors = append(batchResult.Errors, UploadError{
						LocalPath: fp,
						Key:       objectName,
						Error:     err.Error(),
					})
				} else {
					batchResult.Success++
					batchResult.Results = append(batchResult.Results, *result)
				}
				bar.Increment()
				mu.Unlock()
			}(filePath)
		}

		wg.Wait()
	} else {
		// 顺序上传
		for _, filePath := range files {
			relPath, _ := filepath.Rel(localDir, filePath)
			objectName := prefix + filepath.ToSlash(relPath)
			contentType := detectContentType(filePath)

			result, err := p.UploadObject(ctx, bucketName, objectName, filePath, contentType, nil)
			if err != nil {
				batchResult.Failed++
				batchResult.Errors = append(batchResult.Errors, UploadError{
					LocalPath: filePath,
					Key:       objectName,
					Error:     err.Error(),
				})
			} else {
				batchResult.Success++
				batchResult.Results = append(batchResult.Results, *result)
			}
			bar.Increment()
		}
	}

	bar.Done()
	return batchResult, nil
}

var _ io.Reader = (*progressReader)(nil)

// PutDir 递归上传本地目录到存储桶（CLI 直接输出）
// concurrent 参数指定并发数，为 0 时逐个上传
func (p *Putter) PutDir(ctx context.Context, bucketName, prefix, localDir string, concurrent int) {
	result, err := p.UploadDirectory(ctx, bucketName, prefix, localDir, concurrent)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("目录上传完成: %s -> %s/%s\n", localDir, bucketName, prefix)
	if concurrent > 0 {
		fmt.Printf("并发数: %d, ", concurrent)
	}
	fmt.Printf("成功: %d, 失败: %d\n", result.Success, result.Failed)

	if len(result.Errors) > 0 {
		fmt.Println("\n失败的文件:")
		for _, e := range result.Errors {
			fmt.Printf("  %s -> %s: %s\n", e.LocalPath, e.Key, e.Error)
		}
	}
}

// detectContentType 根据文件扩展名检测 Content-Type
func detectContentType(filename string) string {
	ext := filepath.Ext(filename)
	if ext != "" {
		contentType := mime.TypeByExtension(ext)
		if contentType != "" {
			return contentType
		}
	}
	return "application/octet-stream"
}
