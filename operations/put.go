package operations

import (
	"context"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"

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

	// 上传对象
	uploadInfo, err := p.client.PutObject(ctx, bucketName, objectName, file, fileInfo.Size(),
		minio.PutObjectOptions{
			ContentType: contentType,
		})
	if err != nil {
		return nil, fmt.Errorf("上传对象失败: %w", err)
	}

	// 如果有进度回调，模拟完成通知
	if progressCb != nil {
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
