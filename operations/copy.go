package operations

import (
	"context"
	"fmt"
	"log"
	"math"
	"path"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
)

// Copier 对象复制操作
type Copier struct {
	client *minio.Client
	core   *minio.Core
}

// NewCopier 创建 Copier
func NewCopier(client *minio.Client, core *minio.Core) *Copier {
	return &Copier{client: client, core: core}
}

// CopyObject 复制对象到目标位置，返回结构化数据
func (c *Copier) CopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string) (*CopyResult, error) {
	src := minio.CopySrcOptions{
		Bucket: srcBucket,
		Object: srcObject,
	}

	dst := minio.CopyDestOptions{
		Bucket: destBucket,
		Object: destObject,
	}

	result, err := c.client.CopyObject(ctx, dst, src)
	if err != nil {
		return nil, fmt.Errorf("复制对象失败: %w", err)
	}

	return &CopyResult{
		SrcBucket: srcBucket,
		SrcKey:    srcObject,
		DstBucket: destBucket,
		DstKey:    destObject,
		ETag:      result.ETag,
	}, nil
}

// Copy 复制对象到目标位置（CLI 直接输出）
func (c *Copier) Copy(ctx context.Context, srcBucket, srcObject, destBucket, destObject string) {
	result, err := c.CopyObject(ctx, srcBucket, srcObject, destBucket, destObject)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("复制成功: %s/%s -> %s/%s (ETag: %s)\n",
		result.SrcBucket, result.SrcKey, result.DstBucket, result.DstKey, result.ETag)
}

// CopyDirectory 递归复制整个目录，返回结构化数据
// concurrent 参数指定并发数，为 0 时逐个复制
func (c *Copier) CopyDirectory(ctx context.Context, srcBucket, srcPrefix, destBucket, destPrefix string, concurrent int) (*BatchCopyResult, error) {
	// 确保 srcPrefix 以 / 结尾表示目录
	if srcPrefix != "" && !strings.HasSuffix(srcPrefix, "/") {
		srcPrefix = srcPrefix + "/"
	}

	// 列出源目录下所有对象
	lister := NewLister(c.client)
	result, err := lister.ListObjects(ctx, srcBucket, srcPrefix, true)
	if err != nil {
		return nil, fmt.Errorf("列出源目录失败: %w", err)
	}

	if len(result.Objects) == 0 {
		return nil, fmt.Errorf("源目录为空或不存在: %s/%s", srcBucket, srcPrefix)
	}

	// 过滤出需要复制的对象，构建复制任务列表
	var tasks []copyTask
	for _, obj := range result.Objects {
		if obj.IsDir {
			continue
		}
		relPath := strings.TrimPrefix(obj.Key, srcPrefix)
		if relPath == obj.Key {
			continue
		}
		destObject := path.Join(destPrefix, relPath)
		if destPrefix == "" {
			destObject = relPath
		}
		tasks = append(tasks, copyTask{srcKey: obj.Key, dstKey: destObject})
	}

	batchResult := &BatchCopyResult{
		SrcBucket:  srcBucket,
		DestBucket: destBucket,
		SrcPrefix:  srcPrefix,
		DestPrefix: destPrefix,
	}

	if concurrent > 0 {
		// 并发复制
		c.copyConcurrent(ctx, srcBucket, destBucket, tasks, batchResult, concurrent)
	} else {
		// 逐个复制
		c.copySequential(ctx, srcBucket, destBucket, tasks, batchResult)
	}

	return batchResult, nil
}

// copyTask 复制任务
type copyTask struct {
	srcKey string
	dstKey string
}

// copySequential 逐个复制
func (c *Copier) copySequential(ctx context.Context, srcBucket, destBucket string, tasks []copyTask, result *BatchCopyResult) {
	for _, task := range tasks {
		copyResult, err := c.CopyObject(ctx, srcBucket, task.srcKey, destBucket, task.dstKey)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, CopyError{
				SrcKey: task.srcKey,
				DstKey: task.dstKey,
				Error:  err.Error(),
			})
			continue
		}
		result.Success++
		result.Results = append(result.Results, *copyResult)
	}
}

// copyConcurrent 并发复制
func (c *Copier) copyConcurrent(ctx context.Context, srcBucket, destBucket string, tasks []copyTask, result *BatchCopyResult, concurrent int) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 使用信号量控制并发数
	sem := make(chan struct{}, concurrent)

	for _, task := range tasks {
		wg.Add(1)
		go func(t copyTask) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			copyResult, err := c.CopyObject(ctx, srcBucket, t.srcKey, destBucket, t.dstKey)

			mu.Lock()
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, CopyError{
					SrcKey: t.srcKey,
					DstKey: t.dstKey,
					Error:  err.Error(),
				})
			} else {
				result.Success++
				result.Results = append(result.Results, *copyResult)
			}
			mu.Unlock()
		}(task)
	}

	wg.Wait()
}

// CopyDir 递归复制整个目录（CLI 直接输出）
// concurrent 参数指定并发数，为 0 时逐个复制
func (c *Copier) CopyDir(ctx context.Context, srcBucket, srcPrefix, destBucket, destPrefix string, concurrent int) {
	result, err := c.CopyDirectory(ctx, srcBucket, srcPrefix, destBucket, destPrefix, concurrent)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("目录复制完成: %s/%s -> %s/%s\n", srcBucket, srcPrefix, destBucket, destPrefix)
	if concurrent > 0 {
		fmt.Printf("并发数: %d, ", concurrent)
	}
	fmt.Printf("成功: %d, 失败: %d\n", result.Success, result.Failed)

	if len(result.Errors) > 0 {
		fmt.Println("\n失败的对象:")
		for _, e := range result.Errors {
			fmt.Printf("  %s -> %s: %s\n", e.SrcKey, e.DstKey, e.Error)
		}
	}
}

// ==================== 分片复制（大文件） ====================

const (
	defaultPartSize = 64 * 1024 * 1024 // 64 MiB 默认分片大小
	maxPartsCount   = 10000            // S3 最大分片数量
)

// MultipartCopyObject 分片复制大文件，返回结构化数据
func (c *Copier) MultipartCopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string) (*CopyResult, error) {
	// 1. 获取源对象信息
	srcInfo, err := c.core.StatObject(ctx, srcBucket, srcObject, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取源对象信息失败: %w", err)
	}

	objectSize := srcInfo.Size

	// 2. 计算分片大小
	partSize := calculatePartSize(objectSize)

	// 3. 初始化分片上传
	uploadID, err := c.core.NewMultipartUpload(ctx, destBucket, destObject, minio.PutObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("初始化分片上传失败: %w", err)
	}

	// 4. 分片复制，出错时清理
	var completed bool
	defer func() {
		if !completed {
			_ = c.core.AbortMultipartUpload(ctx, destBucket, destObject, uploadID)
		}
	}()

	var parts []minio.CompletePart
	partCount := int(math.Ceil(float64(objectSize) / float64(partSize)))

	for i := range partCount {
		partNumber := i + 1
		startOffset := int64(i) * partSize

		// 计算当前分片长度
		remaining := objectSize - startOffset
		length := min(remaining, partSize)

		// 执行分片复制
		part, err := c.core.CopyObjectPart(ctx, srcBucket, srcObject,
			destBucket, destObject, uploadID, partNumber, startOffset, length, nil)
		if err != nil {
			return nil, fmt.Errorf("分片 %d 复制失败: %w", partNumber, err)
		}

		parts = append(parts, part)
	}

	// 5. 完成分片上传
	uploadInfo, err := c.core.CompleteMultipartUpload(ctx, destBucket, destObject, uploadID, parts, minio.PutObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("完成分片上传失败: %w", err)
	}

	completed = true

	return &CopyResult{
		SrcBucket: srcBucket,
		SrcKey:    srcObject,
		DstBucket: destBucket,
		DstKey:    destObject,
		Size:      objectSize,
		ETag:      uploadInfo.ETag,
	}, nil
}

// MultipartCopy 分片复制大文件（CLI 直接输出）
func (c *Copier) MultipartCopy(ctx context.Context, srcBucket, srcObject, destBucket, destObject string) {
	result, err := c.MultipartCopyObject(ctx, srcBucket, srcObject, destBucket, destObject)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("分片复制成功: %s/%s -> %s/%s (大小: %d 字节, ETag: %s)\n",
		result.SrcBucket, result.SrcKey, result.DstBucket, result.DstKey, result.Size, result.ETag)
}

// calculatePartSize 计算合适的分片大小
func calculatePartSize(objectSize int64) int64 {
	partSize := int64(defaultPartSize)

	// 确保分片数量不超过 10000
	for objectSize/partSize >= maxPartsCount {
		partSize *= 2
	}

	return partSize
}