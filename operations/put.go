package operations

import (
	"context"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
)

// Putter 对象上传操作
type Putter struct {
	client *minio.Client
}

// NewPutter 创建 Putter
func NewPutter(client *minio.Client) *Putter {
	return &Putter{client: client}
}

// Put 上传本地文件到存储桶
func (p *Putter) Put(ctx context.Context, bucketName, objectName, localPath, contentType string) {
	// 检查本地文件是否存在
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		log.Fatalf("文件不存在或无法访问: %v", err)
	}

	// 如果是目录，报错
	if fileInfo.IsDir() {
		log.Fatal("不支持上传目录，请指定文件")
	}

	// 自动检测 Content-Type
	if contentType == "" {
		contentType = detectContentType(localPath)
	}

	// 打开本地文件
	file, err := os.Open(localPath)
	if err != nil {
		log.Fatalf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 上传对象
	uploadInfo, err := p.client.PutObject(ctx, bucketName, objectName, file, fileInfo.Size(),
		minio.PutObjectOptions{
			ContentType: contentType,
		})
	if err != nil {
		log.Fatalf("上传对象失败: %v", err)
	}

	fmt.Printf("上传成功: %s -> %s/%s (大小: %d 字节, ETag: %s)\n",
		localPath, bucketName, objectName, uploadInfo.Size, strings.Trim(uploadInfo.ETag, "\""))
}

// detectContentType 根据文件扩展名检测 Content-Type
func detectContentType(filename string) string {
	ext := filepath.Ext(filename)
	if ext != "" {
		contentType := mime.TypeByExtension(ext)
		if contentType != "" {
			return contentType
		}
	}
	return "application/octet-stream"
}
