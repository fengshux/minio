package operations

import (
	"context"
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