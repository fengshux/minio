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

// ListObjects 列出存储桶中的对象，返回结构化数据
func (l *Lister) ListObjects(ctx context.Context, bucketName, prefix string, recursive bool) (*ListResult, error) {
	opts := minio.ListObjectsOptions{
		Recursive: recursive,
		Prefix:    prefix,
	}

	result := &ListResult{}
	objectCh := l.client.ListObjects(ctx, bucketName, opts)

	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("列出对象失败: %w", object.Err)
		}

		result.Objects = append(result.Objects, ObjectInfo{
			Key:          object.Key,
			Size:         object.Size,
			LastModified: object.LastModified,
			ETag:         object.ETag,
			ContentType:  object.ContentType,
		})
	}

	return result, nil
}

// ListBuckets 列出所有存储桶
func (l *Lister) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	buckets, err := l.client.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出存储桶失败: %w", err)
	}

	result := make([]BucketInfo, 0, len(buckets))
	for _, b := range buckets {
		result = append(result, BucketInfo{
			Name:         b.Name,
			CreationDate: b.CreationDate,
		})
	}
	return result, nil
}

// List 列出存储桶中的对象（CLI 直接输出）
func (l *Lister) List(ctx context.Context, bucketName, prefix string, recursive bool) {
	result, err := l.ListObjects(ctx, bucketName, prefix, recursive)
	if err != nil {
		fmt.Printf("列出对象时出错: %v\n", err)
		return
	}
	for _, obj := range result.Objects {
		fmt.Printf("对象: %s\n  大小: %.2f MB\n  最后修改: %s\n  ETag: %s\n",
			obj.Key,
			float64(obj.Size)/1024/1024,
			obj.LastModified.Local().Format("2006-01-02 15:04:05"),
			obj.ETag,
		)
	}
}