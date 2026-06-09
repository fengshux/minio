package operations

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/minio/minio-go/v7"
)

// Getter 对象下载操作
type Getter struct {
	client *minio.Client
}

// NewGetter 创建 Getter
func NewGetter(client *minio.Client) *Getter {
	return &Getter{client: client}
}

// DownloadObject 下载对象到本地，返回结构化数据
// progressCb 可以为 nil，如果不为 nil 则在下载过程中调用
func (g *Getter) DownloadObject(ctx context.Context, bucketName, objectName, outputPath string, progressCb ProgressCallback) (*DownloadResult, error) {
	if outputPath == "" {
		paths := strings.Split(objectName, "/")
		outputPath = paths[len(paths)-1]
	}

	// 获取对象信息
	info, err := g.client.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取对象信息失败: %w", err)
	}

	// 下载对象
	err = g.client.FGetObject(ctx, bucketName, objectName, outputPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("下载对象失败: %w", err)
	}

	// 如果有进度回调，模拟完成通知
	if progressCb != nil {
		progressCb(info.Size, info.Size)
	}

	return &DownloadResult{
		Bucket:      bucketName,
		Key:         objectName,
		LocalPath:   outputPath,
		Size:        info.Size,
		ContentType: info.ContentType,
	}, nil
}

// ReadObject 读取对象内容，返回 io.ReadCloser
func (g *Getter) ReadObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, *ObjectInfo, error) {
	obj, err := g.client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("获取对象失败: %w", err)
	}

	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, nil, fmt.Errorf("获取对象信息失败: %w", err)
	}

	return obj, &ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		LastModified: info.LastModified,
		ETag:         info.ETag,
		ContentType:  info.ContentType,
	}, nil
}

// Get 下载对象到本地（CLI 直接输出）
func (g *Getter) Get(ctx context.Context, bucketName, objectName, outputPath string) {
	result, err := g.DownloadObject(ctx, bucketName, objectName, outputPath, nil)
	if err != nil {
		log.Fatalf("%v", err)
	}
	fmt.Printf("下载成功: %s -> %s\n", result.Key, result.LocalPath)
}