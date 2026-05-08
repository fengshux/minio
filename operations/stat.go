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

// Stat 查询对象元数据
func (s *Stater) Stat(ctx context.Context, bucketName, objectName string) {
	opts := minio.StatObjectOptions{}
	objInfo, err := s.client.StatObject(ctx, bucketName, objectName, opts)
	if err != nil {
		log.Fatalf("查询对象状态失败: %v", err)
	}

	fmt.Printf("对象状态查询成功:\n")
	fmt.Printf("  对象名: %s\n", objInfo.Key)
	fmt.Printf("  大小: %.2f MB (%.0f 字节)\n", float64(objInfo.Size)/1024/1024, float64(objInfo.Size))
	fmt.Printf("  ETag: %s\n", objInfo.ETag)
	fmt.Printf("  最后修改时间: %s\n", objInfo.LastModified.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Content-Type: %s\n", objInfo.ContentType)
}