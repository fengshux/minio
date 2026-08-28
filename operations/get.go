package operations

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

// DownloadObject 下载对象到本地，返回结构化数据
// progressCb 可以为 nil，如果不为 nil 则在下载过程中调用
func (g *Getter) DownloadObject(ctx context.Context, bucketName, objectName, outputPath string, progressCb ProgressCallback) (*DownloadResult, error) {
	if outputPath == "" {
		paths := strings.Split(objectName, "/")
		outputPath = paths[len(paths)-1]
	}

	// 获取对象信息
	info, err := g.client.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取对象信息失败: %w", err)
	}

	object, err := g.client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取对象失败: %w", err)
	}
	defer object.Close()

	file, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("创建本地文件失败: %w", err)
	}
	defer file.Close()

	writer := newProgressWriter(file, info.Size, progressCb)
	if _, err = io.Copy(writer, object); err != nil {
		return nil, fmt.Errorf("下载对象失败: %w", err)
	}

	return &DownloadResult{
		Bucket:      bucketName,
		Key:         objectName,
		LocalPath:   outputPath,
		Size:        info.Size,
		ContentType: info.ContentType,
	}, nil
}

// ReadObject 读取对象内容，返回 io.ReadCloser
func (g *Getter) ReadObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, *ObjectInfo, error) {
	obj, err := g.client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("获取对象失败: %w", err)
	}

	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, nil, fmt.Errorf("获取对象信息失败: %w", err)
	}

	return obj, &ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		LastModified: info.LastModified,
		ETag:         info.ETag,
		ContentType:  info.ContentType,
	}, nil
}

// Get 下载对象到本地（CLI 直接输出）
func (g *Getter) Get(ctx context.Context, bucketName, objectName, outputPath string) {
	result, err := g.DownloadObject(ctx, bucketName, objectName, outputPath, nil)
	if err != nil {
		log.Fatalf("%v", err)
	}
	fmt.Printf("下载成功: %s -> %s\n", result.Key, result.LocalPath)
}

// DownloadDirectory 递归下载整个目录，返回结构化数据
// 使用流式方式列出对象，边列出边下载
// concurrent 参数指定并发数，为 0 时逐个下载
func (g *Getter) DownloadDirectory(ctx context.Context, bucketName, prefix, localDir string, concurrent int) (*BatchDownloadResult, error) {
	// 确保 prefix 以 / 结尾表示目录
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	// 创建本地目录
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return nil, fmt.Errorf("创建本地目录失败: %w", err)
	}

	lister := NewLister(g.client)
	objectCh := lister.ListObjectsStream(ctx, bucketName, prefix, true)

	batchResult := &BatchDownloadResult{
		Bucket:   bucketName,
		Prefix:   prefix,
		LocalDir: localDir,
	}

	bar := NewProgressBar(0, "下载中")

	if concurrent > 0 {
		// 并发下载
		var wg sync.WaitGroup
		var mu sync.Mutex
		sem := make(chan struct{}, concurrent)

		for result := range objectCh {
			if result.Err != nil {
				mu.Lock()
				batchResult.Failed++
				batchResult.Errors = append(batchResult.Errors, DownloadError{
					Key:   prefix,
					Error: result.Err.Error(),
				})
				mu.Unlock()
				continue
			}
			if result.Object.IsDir {
				continue
			}

			// 计算本地路径
			relPath := strings.TrimPrefix(result.Object.Key, prefix)
			if relPath == result.Object.Key {
				continue
			}
			localPath := filepath.Join(localDir, relPath)

			// 创建本地子目录
			localSubDir := filepath.Dir(localPath)
			if err := os.MkdirAll(localSubDir, 0755); err != nil {
				mu.Lock()
				batchResult.Failed++
				batchResult.Errors = append(batchResult.Errors, DownloadError{
					Key:       result.Object.Key,
					LocalPath: localPath,
					Error:     fmt.Sprintf("创建目录失败: %v", err),
				})
				mu.Unlock()
				continue
			}

			bar.AddTotal(1)

			wg.Add(1)
			go func(key, local string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				dlResult, err := g.DownloadObject(ctx, bucketName, key, local, nil)

				mu.Lock()
				if err != nil {
					batchResult.Failed++
					batchResult.Errors = append(batchResult.Errors, DownloadError{
						Key:       key,
						LocalPath: local,
						Error:     err.Error(),
					})
				} else {
					batchResult.Success++
					batchResult.Results = append(batchResult.Results, *dlResult)
				}
				bar.Increment()
				mu.Unlock()
			}(result.Object.Key, localPath)
		}

		wg.Wait()
	} else {
		// 顺序下载
		for result := range objectCh {
			if result.Err != nil {
				batchResult.Failed++
				batchResult.Errors = append(batchResult.Errors, DownloadError{
					Key:   prefix,
					Error: result.Err.Error(),
				})
				continue
			}
			if result.Object.IsDir {
				continue
			}

			relPath := strings.TrimPrefix(result.Object.Key, prefix)
			if relPath == result.Object.Key {
				continue
			}
			localPath := filepath.Join(localDir, relPath)

			// 创建本地子目录
			localSubDir := filepath.Dir(localPath)
			if err := os.MkdirAll(localSubDir, 0755); err != nil {
				batchResult.Failed++
				batchResult.Errors = append(batchResult.Errors, DownloadError{
					Key:       result.Object.Key,
					LocalPath: localPath,
					Error:     fmt.Sprintf("创建目录失败: %v", err),
				})
				continue
			}

			bar.AddTotal(1)

			dlResult, err := g.DownloadObject(ctx, bucketName, result.Object.Key, localPath, nil)
			if err != nil {
				batchResult.Failed++
				batchResult.Errors = append(batchResult.Errors, DownloadError{
					Key:       result.Object.Key,
					LocalPath: localPath,
					Error:     err.Error(),
				})
			} else {
				batchResult.Success++
				batchResult.Results = append(batchResult.Results, *dlResult)
			}
			bar.Increment()
		}
	}

	bar.Done()
	return batchResult, nil
}

// GetDir 递归下载整个目录（CLI 直接输出）
// concurrent 参数指定并发数，为 0 时逐个下载
func (g *Getter) GetDir(ctx context.Context, bucketName, prefix, localDir string, concurrent int) {
	result, err := g.DownloadDirectory(ctx, bucketName, prefix, localDir, concurrent)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("目录下载完成: %s/%s -> %s\n", bucketName, prefix, localDir)
	if concurrent > 0 {
		fmt.Printf("并发数: %d, ", concurrent)
	}
	fmt.Printf("成功: %d, 失败: %d\n", result.Success, result.Failed)

	if len(result.Errors) > 0 {
		fmt.Println("\n失败的对象:")
		for _, e := range result.Errors {
			fmt.Printf("  %s -> %s: %s\n", e.Key, e.LocalPath, e.Error)
		}
	}
}
