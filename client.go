package main

import (
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// NewClient 创建 MinIO 客户端
func NewClient(cfg *Config, debug bool) (*minio.Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	if debug {
		client.TraceOn(os.Stderr) // 输出 HTTP 请求详情到标准错误
	}
	return client, nil
}

// NewCoreClient 创建 MinIO Core 客户端（用于分片操作）
func NewCoreClient(cfg *Config, debug bool) (*minio.Core, error) {
	core, err := minio.NewCore(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	if debug {
		core.TraceOn(os.Stderr)
	}
	return core, nil
}