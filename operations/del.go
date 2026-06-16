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

// deleteTask 删除任务
type deleteTask struct {
	key string
}

// DeleteDirectoryObjects 递归删除整个目录，返回删除数量和错误
func (d *Deleter) DeleteDirectoryObjects(ctx context.Context, bucket, prefix string, concurrent int) (int, []DeleteError) {
	// 确保 prefix 以 / 结尾表示目录
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	// 列出目录下所有对象
	lister := NewLister(d.client)
	result, err := lister.ListObjects(ctx, bucket, prefix, true)
	if err != nil {
		return 0, []DeleteError{{Key: prefix, Error: fmt.Sprintf("列出目录失败: %v", err)}}
	}

	if len(result.Objects) == 0 {
		return 0, []DeleteError{{Key: prefix, Error: "目录为空或不存在"}}
	}

	// 构建删除任务列表
	var tasks []deleteTask
	for _, obj := range result.Objects {
		if obj.IsDir {
			continue
		}
		tasks = append(tasks, deleteTask{key: obj.Key})
	}

	if concurrent > 0 {
		return d.deleteConcurrent(ctx, bucket, tasks, concurrent)
	}
	return d.deleteSequential(ctx, bucket, tasks)
}

// deleteSequential 逐个删除
func (d *Deleter) deleteSequential(ctx context.Context, bucket string, tasks []deleteTask) (int, []DeleteError) {
	count := 0
	var errors []DeleteError

	for _, task := range tasks {
		err := d.client.RemoveObject(ctx, bucket, task.key, minio.RemoveObjectOptions{})
		if err != nil {
			errors = append(errors, DeleteError{
				Key:   task.key,
				Error: err.Error(),
			})
			continue
		}
		count++
	}

	return count, errors
}

// deleteConcurrent 并发删除
func (d *Deleter) deleteConcurrent(ctx context.Context, bucket string, tasks []deleteTask, concurrent int) (int, []DeleteError) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	count := 0
	var errors []DeleteError

	// 使用信号量控制并发数
	sem := make(chan struct{}, concurrent)

	for _, task := range tasks {
		wg.Add(1)
		go func(t deleteTask) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			err := d.client.RemoveObject(ctx, bucket, t.key, minio.RemoveObjectOptions{})

			mu.Lock()
			if err != nil {
				errors = append(errors, DeleteError{
					Key:   t.key,
					Error: err.Error(),
				})
			} else {
				count++
			}
			mu.Unlock()
		}(task)
	}

	wg.Wait()
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