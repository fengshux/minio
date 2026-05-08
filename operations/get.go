package operations

import (
	"context"
	"fmt"
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

// Get 下载对象到本地
func (g *Getter) Get(ctx context.Context, bucketName, objectName, outputPath string) {
	if outputPath == "" {
		paths := strings.Split(objectName, "/")
		outputPath = paths[len(paths)-1]
	}

	err := g.client.FGetObject(ctx, bucketName, objectName, outputPath, minio.GetObjectOptions{})
	if err != nil {
		log.Fatalf("下载对象失败: %v", err)
	}
	fmt.Printf("下载成功: %s -> %s\n", objectName, outputPath)
}