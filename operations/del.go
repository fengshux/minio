package operations

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
)

// Deleter 对象删除操作
type Deleter struct {
	client *minio.Client
}

// NewDeleter 创建 Deleter
func NewDeleter(client *minio.Client) *Deleter {
	return &Deleter{client: client}
}

// confirmDelete 确认删除操作
// 返回 true 表示确认删除，false 表示取消
func confirmDelete(target string) bool {
	fmt.Printf("确定要删除 %s 吗？(y/N): ", target)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y"
}

// DeleteObject 删除单个对象，返回结构化数据
func (d *Deleter) DeleteObject(ctx context.Context, bucket, object string) error {
	err := d.client.RemoveObject(ctx, bucket, object, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("删除对象失败: %w", err)
	}
	return nil
}

// Delete 删除单个对象（CLI 直接输出）
func (d *Deleter) Delete(ctx context.Context, bucket, object string, force bool) {
	target := fmt.Sprintf("%s/%s", bucket, object)

	// 如果不是强制模式，需要确认
	if !force {
		if !confirmDelete(target) {
			fmt.Println("已取消删除")
			return
		}
	}

	err := d.DeleteObject(ctx, bucket, object)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("删除成功: %s\n", target)
}

// DeleteDirectoryObjects 递归删除整个目录，返回删除数量和错误
// 使用流式方式列出对象，边列出边删除
func (d *Deleter) DeleteDirectoryObjects(ctx context.Context, bucket, prefix string, concurrent int) (int, []DeleteError) {
	// 确保 prefix 以 / 结尾表示目录
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	lister := NewLister(d.client)
	objectCh := lister.ListObjectsStream(ctx, bucket, prefix, true)

	count := 0
	var errors []DeleteError
	var mu sync.Mutex

	bar := NewProgressBar(0, "删除中") // 初始总数为 0，流式增加

	if concurrent > 0 {
		// 并发删除
		var wg sync.WaitGroup
		sem := make(chan struct{}, concurrent)

		for result := range objectCh {
			if result.Err != nil {
				errors = append(errors, DeleteError{Key: prefix, Error: result.Err.Error()})
				continue
			}
			if result.Object.IsDir {
				continue
			}

			bar.AddTotal(1) // 动态增加总数

			wg.Add(1)
			go func(key string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				err := d.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})

				mu.Lock()
				if err != nil {
					errors = append(errors, DeleteError{
						Key:   key,
						Error: err.Error(),
					})
				} else {
					count++
				}
				bar.Increment()
				mu.Unlock()
			}(result.Object.Key)
		}

		wg.Wait()
	} else {
		// 顺序删除
		for result := range objectCh {
			if result.Err != nil {
				errors = append(errors, DeleteError{Key: prefix, Error: result.Err.Error()})
				continue
			}
			if result.Object.IsDir {
				continue
			}

			bar.AddTotal(1)

			err := d.client.RemoveObject(ctx, bucket, result.Object.Key, minio.RemoveObjectOptions{})
			if err != nil {
				errors = append(errors, DeleteError{
					Key:   result.Object.Key,
					Error: err.Error(),
				})
			} else {
				count++
			}
			bar.Increment()
		}
	}

	bar.Done()
	return count, errors
}

// DeleteDir 递归删除整个目录（CLI 直接输出）
func (d *Deleter) DeleteDir(ctx context.Context, bucket, prefix string, force bool, concurrent int) {
	target := fmt.Sprintf("%s/%s", bucket, prefix)

	// 如果不是强制模式，需要确认
	if !force {
		if !confirmDelete(target) {
			fmt.Println("已取消删除")
			return
		}
	}

	count, errors := d.DeleteDirectoryObjects(ctx, bucket, prefix, concurrent)

	fmt.Printf("目录删除完成: %s\n", target)
	if concurrent > 0 {
		fmt.Printf("并发数: %d, ", concurrent)
	}
	fmt.Printf("成功: %d, 失败: %d\n", count, len(errors))

	if len(errors) > 0 {
		fmt.Println("\n失败的对象:")
		for _, e := range errors {
			fmt.Printf("  %s: %s\n", e.Key, e.Error)
		}
	}
}