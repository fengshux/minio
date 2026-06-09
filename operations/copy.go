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

// CopyObject 复制对象到目标位置，返回结构化数据
func (c *Copier) CopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string) (*CopyResult, error) {
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
		return nil, fmt.Errorf("复制对象失败: %w", err)
	}

	return &CopyResult{
		SrcBucket: srcBucket,
		SrcKey:    srcObject,
		DstBucket: destBucket,
		DstKey:    destObject,
		ETag:      result.ETag,
	}, nil
}

// Copy 复制对象到目标位置（CLI 直接输出）
func (c *Copier) Copy(ctx context.Context, srcBucket, srcObject, destBucket, destObject string) {
	result, err := c.CopyObject(ctx, srcBucket, srcObject, destBucket, destObject)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("复制成功: %s/%s -> %s/%s (ETag: %s)\n",
		result.SrcBucket, result.SrcKey, result.DstBucket, result.DstKey, result.ETag)
}