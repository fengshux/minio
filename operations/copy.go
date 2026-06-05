package operations

import (
	"context"
	"fmt"
	"log"

	"github.com/minio/minio-go/v7"
)

// Copier 对象复制操作
type Copier struct {
	client *minio.Client
}

// NewCopier 创建 Copier
func NewCopier(client *minio.Client) *Copier {
	return &Copier{client: client}
}

// Copy 复制对象到目标位置
func (c *Copier) Copy(ctx context.Context, srcBucket, srcObject, destBucket, destObject string) {
	// 使用 CopyObject 方法复制对象
	src := minio.CopySrcOptions{
		Bucket: srcBucket,
		Object: srcObject,
	}

	dst := minio.CopyDestOptions{
		Bucket: destBucket,
		Object: destObject,
	}

	result, err := c.client.CopyObject(ctx, dst, src)
	if err != nil {
		log.Fatalf("复制对象失败: %v", err)
	}

	fmt.Printf("复制成功: %s/%s -> %s/%s (ETag: %s)\n",
		srcBucket, srcObject, destBucket, destObject, result.ETag)
}