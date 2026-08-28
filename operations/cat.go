package operations

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/minio/minio-go/v7"
)

// Catter 对象内容输出操作
type Catter struct {
	client *minio.Client
}

// NewCatter 创建 Catter
func NewCatter(client *minio.Client) *Catter {
	return &Catter{client: client}
}

// ReadContent 读取对象内容，返回字节切片
func (c *Catter) ReadContent(ctx context.Context, bucketName, objectName string) ([]byte, *ObjectInfo, error) {
	object, err := c.client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("获取对象失败: %w", err)
	}
	defer object.Close()

	info, err := object.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("获取对象信息失败: %w", err)
	}

	data, err := io.ReadAll(object)
	if err != nil {
		return nil, nil, fmt.Errorf("读取对象内容失败: %w", err)
	}

	return data, &ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		LastModified: info.LastModified,
		ETag:         info.ETag,
		ContentType:  info.ContentType,
	}, nil
}

// Cat 输出对象内容到标准输出
func (c *Catter) Cat(ctx context.Context, bucketName, objectName string) {
	object, err := c.client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		log.Fatalf("获取对象失败: %v", err)
	}
	defer object.Close()

	_, err = io.Copy(os.Stdout, object)
	if err != nil {
		log.Fatalf("输出对象内容失败: %v", err)
	}
}
