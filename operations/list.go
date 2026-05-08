package operations

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

// Lister 对象列表操作
type Lister struct {
	client *minio.Client
}

// NewLister 创建 Lister
func NewLister(client *minio.Client) *Lister {
	return &Lister{client: client}
}

// List 列出存储桶中的对象
func (l *Lister) List(ctx context.Context, bucketName, prefix string, recursive bool) {
	opts := minio.ListObjectsOptions{
		Recursive: recursive,
		Prefix:    prefix,
	}

	objectCh := l.client.ListObjects(ctx, bucketName, opts)
	for object := range objectCh {
		if object.Err != nil {
			fmt.Printf("列出对象时出错: %v\n", object.Err)
			continue
		}
		fmt.Printf("对象: %s\n  大小: %.2f MB\n  最后修改: %s\n  ETag: %s\n",
			object.Key,
			float64(object.Size)/1024/1024,
			object.LastModified.Format("2006-01-02 15:04:05"),
			object.ETag,
		)
	}
}