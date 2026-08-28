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

// objectCopyFn 单个对象的复制实现，由 copyTree 调用。
// 服务端复制与跨 context 流式复制各自提供不同实现。
type objectCopyFn func(ctx context.Context, srcKey, dstKey string) (*CopyResult, error)

// copyTree 流式列举源前缀下的对象并逐个执行 copyFn，
// 负责目标路径计算、并发调度、进度条与错误汇总。
// srcLister 必须基于源端 client 构建；concurrent 为 0 时顺序执行。
func copyTree(ctx context.Context, srcLister *Lister,
	srcBucket, srcPrefix, destBucket, destPrefix string,
	concurrent int, desc string, copyFn objectCopyFn) *BatchCopyResult {

	// 确保 srcPrefix 以 / 结尾表示目录
	if srcPrefix != "" && !strings.HasSuffix(srcPrefix, "/") {
		srcPrefix = srcPrefix + "/"
	}

	objectCh := srcLister.ListObjectsStream(ctx, srcBucket, srcPrefix, true)

	batchResult := &BatchCopyResult{
		SrcBucket:  srcBucket,
		DestBucket: destBucket,
		SrcPrefix:  srcPrefix,
		DestPrefix: destPrefix,
	}

	bar := NewProgressBar(0, desc) // 初始总数为 0，流式增加

	// destKeyFor 计算目标对象名，返回 false 表示该对象应跳过
	destKeyFor := func(srcKey string) (string, bool) {
		relPath := strings.TrimPrefix(srcKey, srcPrefix)
		if relPath == srcKey {
			return "", false
		}
		if destPrefix == "" {
			return relPath, true
		}
		return path.Join(destPrefix, relPath), true
	}

	if concurrent > 0 {
		// 并发复制
		var wg sync.WaitGroup
		var mu sync.Mutex
		sem := make(chan struct{}, concurrent)

		for result := range objectCh {
			if result.Err != nil {
				mu.Lock()
				batchResult.Failed++
				batchResult.Errors = append(batchResult.Errors, CopyError{
					SrcKey: srcPrefix,
					Error:  result.Err.Error(),
				})
				mu.Unlock()
				continue
			}
			if result.Object.IsDir {
				continue
			}

			destObject, ok := destKeyFor(result.Object.Key)
			if !ok {
				continue
			}

			bar.AddTotal(1) // 动态增加总数

			wg.Add(1)
			go func(srcKey, dstKey string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				copyResult, err := copyFn(ctx, srcKey, dstKey)

				mu.Lock()
				if err != nil {
					batchResult.Failed++
					batchResult.Errors = append(batchResult.Errors, CopyError{
						SrcKey: srcKey,
						DstKey: dstKey,
						Error:  err.Error(),
					})
				} else {
					batchResult.Success++
					batchResult.Results = append(batchResult.Results, *copyResult)
				}
				bar.Increment()
				mu.Unlock()
			}(result.Object.Key, destObject)
		}

		wg.Wait()
	} else {
		// 顺序复制
		for result := range objectCh {
			if result.Err != nil {
				batchResult.Failed++
				batchResult.Errors = append(batchResult.Errors, CopyError{
					SrcKey: srcPrefix,
					Error:  result.Err.Error(),
				})
				continue
			}
			if result.Object.IsDir {
				continue
			}

			destObject, ok := destKeyFor(result.Object.Key)
			if !ok {
				continue
			}

			bar.AddTotal(1)

			copyResult, err := copyFn(ctx, result.Object.Key, destObject)
			if err != nil {
				batchResult.Failed++
				batchResult.Errors = append(batchResult.Errors, CopyError{
					SrcKey: result.Object.Key,
					DstKey: destObject,
					Error:  err.Error(),
				})
			} else {
				batchResult.Success++
				batchResult.Results = append(batchResult.Results, *copyResult)
			}
			bar.Increment()
		}
	}

	bar.Done()
	return batchResult
}

// printBatchCopyResult 输出批量复制结果（CLI）
func printBatchCopyResult(result *BatchCopyResult, header string, concurrent int) {
	fmt.Println(header)
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

// CopyDirectory 递归复制整个目录，返回结构化数据
// 使用流式方式列出对象，边列出边复制
// concurrent 参数指定并发数，为 0 时逐个复制
func (c *Copier) CopyDirectory(ctx context.Context, srcBucket, srcPrefix, destBucket, destPrefix string, concurrent int) (*BatchCopyResult, error) {
	copyFn := func(ctx context.Context, srcKey, dstKey string) (*CopyResult, error) {
		return c.CopyObject(ctx, srcBucket, srcKey, destBucket, dstKey)
	}
	return copyTree(ctx, NewLister(c.client),
		srcBucket, srcPrefix, destBucket, destPrefix, concurrent, "复制中", copyFn), nil
}

// CopyDir 递归复制整个目录（CLI 直接输出）
// concurrent 参数指定并发数，为 0 时逐个复制
func (c *Copier) CopyDir(ctx context.Context, srcBucket, srcPrefix, destBucket, destPrefix string, concurrent int) {
	result, err := c.CopyDirectory(ctx, srcBucket, srcPrefix, destBucket, destPrefix, concurrent)
	if err != nil {
		log.Fatalf("%v", err)
	}

	header := fmt.Sprintf("目录复制完成: %s/%s -> %s/%s", srcBucket, srcPrefix, destBucket, destPrefix)
	printBatchCopyResult(result, header, concurrent)
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

	bar := NewProgressBar(partCount, "分片复制")

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
			bar.Done()
			return nil, fmt.Errorf("分片 %d 复制失败: %w", partNumber, err)
		}

		parts = append(parts, part)
		bar.Increment()
	}

	bar.Done()

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

// ==================== 跨 context 复制（跨 endpoint） ====================

// CrossCopier 跨 context（跨 endpoint）对象复制。
// S3 服务端复制（x-amz-copy-source）只能在同一 endpoint 内进行，
// 因此跨 endpoint 必须经本机流式中转：源端 GetObject -> 目标端 PutObject。
type CrossCopier struct {
	srcClient *minio.Client
	dstClient *minio.Client
}

// NewCrossCopier 创建 CrossCopier
func NewCrossCopier(srcClient, dstClient *minio.Client) *CrossCopier {
	return &CrossCopier{srcClient: srcClient, dstClient: dstClient}
}

// CrossCopyObject 跨 endpoint 流式复制单个对象，返回结构化数据。
// 数据不落盘：源端 reader 直接作为目标端 PutObject 的输入。
// 由于传入了已知的对象大小，minio-go 会自动选择合适的分片大小，
// 因此大文件无需额外处理，内存占用有界。
// 复制完成后比对大小；跨 S3 实现 ETag 算法不可比，故不校验 ETag。
func (c *CrossCopier) CrossCopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string) (*CopyResult, error) {
	obj, err := c.srcClient.GetObject(ctx, srcBucket, srcObject, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("读取源对象失败: %w", err)
	}
	defer obj.Close()

	srcInfo, err := obj.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取源对象信息失败: %w", err)
	}

	uploadInfo, err := c.dstClient.PutObject(ctx, destBucket, destObject, obj, srcInfo.Size,
		minio.PutObjectOptions{
			ContentType: srcInfo.ContentType,
		})
	if err != nil {
		return nil, fmt.Errorf("写入目标对象失败: %w", err)
	}

	if uploadInfo.Size != srcInfo.Size {
		return nil, fmt.Errorf("大小校验失败: 源 %d 字节，目标 %d 字节", srcInfo.Size, uploadInfo.Size)
	}

	return &CopyResult{
		SrcBucket: srcBucket,
		SrcKey:    srcObject,
		DstBucket: destBucket,
		DstKey:    destObject,
		Size:      srcInfo.Size,
		ETag:      uploadInfo.ETag,
	}, nil
}

// CrossCopy 跨 endpoint 流式复制单个对象（CLI 直接输出）
func (c *CrossCopier) CrossCopy(ctx context.Context, srcBucket, srcObject, destBucket, destObject string) {
	result, err := c.CrossCopyObject(ctx, srcBucket, srcObject, destBucket, destObject)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("跨 context 复制成功: %s/%s -> %s/%s (大小: %d 字节, ETag: %s)\n",
		result.SrcBucket, result.SrcKey, result.DstBucket, result.DstKey, result.Size, result.ETag)
}

// CrossCopyDirectory 跨 endpoint 递归复制整个目录，返回结构化数据。
// 列举使用源端 client，复制逐个走流式中转。
// concurrent 参数指定并发数，为 0 时逐个复制。
func (c *CrossCopier) CrossCopyDirectory(ctx context.Context, srcBucket, srcPrefix, destBucket, destPrefix string, concurrent int) (*BatchCopyResult, error) {
	copyFn := func(ctx context.Context, srcKey, dstKey string) (*CopyResult, error) {
		return c.CrossCopyObject(ctx, srcBucket, srcKey, destBucket, dstKey)
	}
	return copyTree(ctx, NewLister(c.srcClient),
		srcBucket, srcPrefix, destBucket, destPrefix, concurrent, "跨 context 复制中", copyFn), nil
}

// CrossCopyDir 跨 endpoint 递归复制整个目录（CLI 直接输出）
func (c *CrossCopier) CrossCopyDir(ctx context.Context, srcBucket, srcPrefix, destBucket, destPrefix string, concurrent int) {
	result, err := c.CrossCopyDirectory(ctx, srcBucket, srcPrefix, destBucket, destPrefix, concurrent)
	if err != nil {
		log.Fatalf("%v", err)
	}

	header := fmt.Sprintf("跨 context 目录复制完成: %s/%s -> %s/%s", srcBucket, srcPrefix, destBucket, destPrefix)
	printBatchCopyResult(result, header, concurrent)
}
