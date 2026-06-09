package operations

import (
	"context"
	"fmt"
	"log"

	"github.com/minio/minio-go/v7"
)

// Stater 对象状态查询操作
type Stater struct {
	client *minio.Client
}

// NewStater 创建 Stater
func NewStater(client *minio.Client) *Stater {
	return &Stater{client: client}
}

// GetObjectInfo 获取对象信息，返回结构化数据
func (s *Stater) GetObjectInfo(ctx context.Context, bucketName, objectName string) (*ObjectInfo, error) {
	objInfo, err := s.client.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("查询对象状态失败: %w", err)
	}

	return &ObjectInfo{
		Key:          objInfo.Key,
		Size:         objInfo.Size,
		LastModified: objInfo.LastModified,
		ETag:         objInfo.ETag,
		ContentType:  objInfo.ContentType,
	}, nil
}

// Stat 查询对象元数据（CLI 直接输出）
func (s *Stater) Stat(ctx context.Context, bucketName, objectName string) {
	info, err := s.GetObjectInfo(ctx, bucketName, objectName)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("对象状态查询成功:\n")
	fmt.Printf("  对象名: %s\n", info.Key)
	fmt.Printf("  大小: %.2f MB (%.0f 字节)\n", float64(info.Size)/1024/1024, float64(info.Size))
	fmt.Printf("  ETag: %s\n", info.ETag)
	fmt.Printf("  最后修改时间: %s\n", info.LastModified.Local().Format("2006-01-02 15:04:05"))
	fmt.Printf("  Content-Type: %s\n", info.ContentType)
}