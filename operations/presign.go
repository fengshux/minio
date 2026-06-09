package operations

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
)

// Signer 签名 URL 操作
type Signer struct {
	client *minio.Client
}

// NewSigner 创建 Signer
func NewSigner(client *minio.Client) *Signer {
	return &Signer{client: client}
}

// PresignURL 生成带签名的下载链接，返回结构化数据
func (s *Signer) PresignURL(ctx context.Context, bucketName, objectName string, expire time.Duration) (*SignResult, error) {
	if expire <= 0 {
		expire = 7 * 24 * time.Hour // 默认 7 天
	}

	url, err := s.client.PresignedGetObject(ctx, bucketName, objectName, expire, nil)
	if err != nil {
		return nil, fmt.Errorf("生成签名 URL 失败: %w", err)
	}

	return &SignResult{
		Bucket:    bucketName,
		Key:       objectName,
		URL:       url.String(),
		ExpiresAt: time.Now().Add(expire),
	}, nil
}

// Sign 生成带签名的下载链接（CLI 直接输出）
func (s *Signer) Sign(ctx context.Context, bucketName, objectName string, expire time.Duration) {
	result, err := s.PresignURL(ctx, bucketName, objectName, expire)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("对象: %s/%s\n", result.Bucket, result.Key)
	fmt.Printf("有效期: %v\n", expire)
	fmt.Printf("签名 URL:\n%s\n", result.URL)
}
