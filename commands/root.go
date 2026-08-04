package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/spf13/cobra"
	"minio/operations"
	"minio/tui"
)

// parseBucketPath 解析 bucket/path 格式的参数
// 返回 bucket 和 path（path 可能为空）
func parseBucketPath(bucketPath string) (bucket, path string, err error) {
	parts := strings.SplitN(bucketPath, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", "", fmt.Errorf("无效的路径格式: %s，应为 bucket/path 或 bucket", bucketPath)
	}
	bucket = parts[0]
	if len(parts) > 1 {
		path = parts[1]
	}
	return bucket, path, nil
}

// requirePath 验证 path 必须非空，用于需要 objectKey 的命令
func requirePath(path, cmdName string) error {
	if path == "" {
		return fmt.Errorf("%s 命令需要指定对象路径，格式: bucket/object", cmdName)
	}
	return nil
}

var (
	configPath string
	debug      bool
	client     *minio.Client
	core       *minio.Core
)

// ClientWrapper 包装 minio.Client 以便在 main 包中初始化
type ClientWrapper struct {
	Client *minio.Client
	Core   *minio.Core
}

// InitClient 由 main 包设置的初始化客户端回调函数
var InitClient func(configPath string, debug bool) (*ClientWrapper, error)

// NewRootCmd 创建根命令
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "minio",
		Short: "MinIO/S3 对象存储管理工具",
		Long: `用于管理 MinIO/S3 对象存储的命令行工具，支持列出、查询、下载对象等操作

配置文件:
  默认查找路径（按优先级）:
    1. 当前目录 ./minio.conf
    2. ~/.config/minio/minio.conf
    3. /etc/minio/minio.conf

  配置文件格式:
    endpoint=<s3-endpoint>
    accesskey=<access-key>
    secretkey=<secret-key>`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			wrapper, err := InitClient(configPath, debug)
			if err != nil {
				return err
			}
			client = wrapper.Client
			core = wrapper.Core
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			// 无子命令时进入 TUI 交互模式
			if err := tui.Run(client); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "配置文件路径（默认: ./minio.conf, ~/.config/minio/minio.conf, /etc/minio/minio.conf）")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "启用 HTTP 请求调试输出")

	// 添加子命令
	rootCmd.AddCommand(
		NewConfigCmd(),
		NewPutCmd(),
		NewListCmd(),
		NewStatCmd(),
		NewGetCmd(),
		NewCatCmd(),
		NewSignCmd(),
		NewCopyCmd(),
		NewDelCmd(),
	)

	return rootCmd
}

// NewListCmd 创建 list 子命令
func NewListCmd() *cobra.Command {
	var recursive bool
	cmd := &cobra.Command{
		Use:     "list bucket/prefix",
		Aliases: []string{"ls"},
		Short:   "列出存储桶中的对象",
		Long: `列出指定存储桶中的所有对象，可选择按前缀过滤和递归列出

参数:
  bucket/prefix  存储桶名称和可选的对象前缀（bucket/ 或 bucket/path/）

示例:
  minio list my-bucket/                # 列出根级对象
  minio list my-bucket/photos/         # 列出 photos/ 前缀下的对象
  minio list my-bucket/ -r             # 递归列出所有对象
  minio list my-bucket/photos/ -r      # 递归列出 photos/ 下所有对象`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, prefix, err := parseBucketPath(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			lister := operations.NewLister(client)
			lister.List(cmd.Context(), bucket, prefix, recursive)
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "递归列出对象")
	return cmd
}

// NewStatCmd 创建 stat 子命令
func NewStatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stat bucket/object",
		Short: "查询对象元数据",
		Long: `获取指定对象的详细信息，包括大小、ETag、最后修改时间、Content-Type 等

参数:
  bucket/object  存储桶名称和对象路径

示例:
  minio stat my-bucket/file.txt
  minio stat my-bucket/photos/2024/image.jpg

输出格式:
  对象名: <key>
  大小: <MB> (<字节>)
  ETag: <etag>
  最后修改时间: <timestamp>
  Content-Type: <type>`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, objectName, err := parseBucketPath(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			if err := requirePath(objectName, "stat"); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			stater := operations.NewStater(client)
			stater.Stat(cmd.Context(), bucket, objectName)
		},
	}
}

// NewGetCmd 创建 get 子命令
func NewGetCmd() *cobra.Command {
	var output string
	var recursive bool
	var concurrent int
	cmd := &cobra.Command{
		Use:   "get bucket/object [local-dir]",
		Short: "下载对象或目录到本地",
		Long: `从存储桶下载指定对象或目录到本地

参数:
  bucket/object  存储桶名称和对象路径或目录前缀
  local-dir      本地保存目录（仅递归下载时使用，默认: 当前目录）

选项:
  -o, --output     本地保存路径（下载单个对象时使用）
  -r, --recursive  递归下载整个目录
  -c, --concurrent 并发下载数量（仅与 -r 一起使用，默认 0 表示逐个下载）

示例:
  # 下载单个对象
  minio get my-bucket/file.txt                             # 保存为 file.txt
  minio get my-bucket/photos/image.jpg -o /tmp/photo.jpg   # 指定路径

  # 递归下载目录
  minio get my-bucket/photos/ ./local-photos/ -r           # 逐个下载
  minio get my-bucket/docs/ ./docs/ -r -c 5                # 5个并发下载`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, objectName, err := parseBucketPath(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			if err := requirePath(objectName, "get"); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			getter := operations.NewGetter(client)

			if recursive {
				// 递归下载目录
				localDir := "."
				if len(args) > 1 {
					localDir = args[1]
				}
				getter.GetDir(cmd.Context(), bucket, objectName, localDir, concurrent)
			} else {
				// 下载单个对象
				getter.Get(cmd.Context(), bucket, objectName, output)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "本地保存路径（下载单个对象时使用）")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "递归下载整个目录")
	cmd.Flags().IntVarP(&concurrent, "concurrent", "c", 0, "并发下载数量（仅与 -r 一起使用）")
	return cmd
}

// NewCatCmd 创建 cat 子命令
func NewCatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cat bucket/object",
		Short: "输出对象内容到控制台",
		Long: `将指定对象的内容直接输出到标准输出

参数:
  bucket/object  存储桶名称和对象路径

示例:
  minio cat my-bucket/file.txt
  minio cat my-bucket/logs/app.log`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, objectName, err := parseBucketPath(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			if err := requirePath(objectName, "cat"); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			catter := operations.NewCatter(client)
			catter.Cat(cmd.Context(), bucket, objectName)
		},
	}
}

// NewPutCmd 创建 put 子命令
func NewPutCmd() *cobra.Command {
	var contentType string
	var recursive bool
	var concurrent int
	cmd := &cobra.Command{
		Use:   "put bucket/object local-file",
		Short: "上传本地文件或目录到存储桶",
		Long: `将本地文件或目录上传到指定存储桶

参数:
  bucket/object  存储桶名称和对象存储路径（或目录前缀，与 -r 一起使用）
  local-file     本地文件路径或目录路径（与 -r 一起使用）

选项:
  -t, --type       Content-Type（默认: 自动检测，仅单文件上传时有效）
  -r, --recursive  递归上传整个目录
  -c, --concurrent 并发上传数量（仅与 -r 一起使用，默认 0 表示逐个上传）

示例:
  minio put my-bucket/file.txt ./local.txt
  minio put my-bucket/photos/image.jpg ./photo.jpg
  minio put my-bucket/data.json ./data.json -t application/json
  minio put my-bucket/photos/ ./local-photos/ -r           # 递归上传目录
  minio put my-bucket/docs/ ./docs/ -r -c 5                # 5个并发上传`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, objectName, err := parseBucketPath(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			if err := requirePath(objectName, "put"); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			localPath := args[1]
			putter := operations.NewPutter(client)

			if recursive {
				putter.PutDir(cmd.Context(), bucket, objectName, localPath, concurrent)
			} else {
				putter.Put(cmd.Context(), bucket, objectName, localPath, contentType)
			}
		},
	}
	cmd.Flags().StringVarP(&contentType, "type", "t", "", "Content-Type（默认: 自动检测）")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "递归上传整个目录")
	cmd.Flags().IntVarP(&concurrent, "concurrent", "c", 0, "并发上传数量（仅与 -r 一起使用）")
	return cmd
}

// NewSignCmd 创建 sign 子命令
func NewSignCmd() *cobra.Command {
	var expire time.Duration
	cmd := &cobra.Command{
		Use:   "sign bucket/object",
		Short: "生成带签名的下载链接",
		Long: `为指定对象生成带签名的 HTTP 下载链接

参数:
  bucket/object  存储桶名称和对象路径

选项:
  -e, --expire  链接有效期（默认: 168h 即 7 天）

示例:
  minio sign my-bucket/file.txt
  minio sign my-bucket/photos/image.jpg -e 24h
  minio sign my-bucket/data.zip -e 1h`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, objectName, err := parseBucketPath(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			if err := requirePath(objectName, "sign"); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			signer := operations.NewSigner(client)
			signer.Sign(cmd.Context(), bucket, objectName, expire)
		},
	}
	cmd.Flags().DurationVarP(&expire, "expire", "e", 7*24*time.Hour, "链接有效期（如: 1h, 24h, 168h）")
	return cmd
}

// NewCopyCmd 创建 copy 子命令
func NewCopyCmd() *cobra.Command {
	var recursive bool
	var concurrent int
	var bigFile bool
	cmd := &cobra.Command{
		Use:   "copy src-bucket/src-object dest-bucket/dest-object",
		Short: "复制对象或目录",
		Long: `将对象或目录从一个位置复制到另一个位置，支持跨存储桶复制

参数:
  src-bucket/src-object   源存储桶和对象名称或目录前缀
  dest-bucket/dest-object 目标存储桶和对象名称或目录前缀

选项:
  -r, --recursive     递归复制整个目录
  -c, --concurrent    并发复制数量（仅与 -r 一起使用，默认 0 表示逐个复制）
  -b, --big           大文件分片复制（用于超过 5GB 的文件）

示例:
  minio copy my-bucket/file.txt my-bucket/copy.txt              # 复制单个对象
  minio copy bucket1/photos/ bucket2/backup/photos/ -r          # 递归复制目录（逐个）
  minio copy bucket1/photos/ bucket2/backup/photos/ -r -c 5     # 递归复制目录（5个并发）
  minio copy bucket1/large.dat bucket2/large-copy.dat -b        # 大文件分片复制`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			srcBucket, srcObject, err := parseBucketPath(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			destBucket, destObject, err := parseBucketPath(args[1])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			if err := requirePath(srcObject, "copy (源)"); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			if err := requirePath(destObject, "copy (目标)"); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			copier := operations.NewCopier(client, core)

			if recursive {
				copier.CopyDir(cmd.Context(), srcBucket, srcObject, destBucket, destObject, concurrent)
			} else if bigFile {
				copier.MultipartCopy(cmd.Context(), srcBucket, srcObject, destBucket, destObject)
			} else {
				copier.Copy(cmd.Context(), srcBucket, srcObject, destBucket, destObject)
			}
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "递归复制整个目录")
	cmd.Flags().IntVarP(&concurrent, "concurrent", "c", 0, "并发复制数量（仅与 -r 一起使用）")
	cmd.Flags().BoolVarP(&bigFile, "big", "b", false, "大文件分片复制")
	return cmd
}

// NewDelCmd 创建 del 子命令
func NewDelCmd() *cobra.Command {
	var recursive bool
	var force bool
	var concurrent int
	cmd := &cobra.Command{
		Use:   "del bucket/object",
		Short: "删除对象或目录",
		Long: `删除指定存储桶中的对象或目录

参数:
  bucket/object  存储桶名称和对象名称或目录前缀

选项:
  -r, --recursive    递归删除整个目录
  -c, --concurrent   并发删除数量（仅与 -r 一起使用，默认 0 表示逐个删除）
  --force            强制删除，不进行确认提示

示例:
  minio del my-bucket/file.txt                  # 删除单个对象（需确认）
  minio del my-bucket/file.txt --force          # 删除单个对象（无需确认）
  minio del my-bucket/photos/ -r                # 递归删除目录（逐个，需确认）
  minio del my-bucket/photos/ -r -c 5           # 递归删除目录（5个并发，需确认）
  minio del my-bucket/photos/ -r --force        # 递归删除目录（逐个，无需确认）
  minio del my-bucket/photos/ -r -c 5 --force   # 递归删除目录（5个并发，无需确认）

确认提示:
  执行删除前会提示 "确定要删除 xxx 吗？(y/N)"
  输入 y 或 Y 确认删除，输入其他则取消操作`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, object, err := parseBucketPath(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			if err := requirePath(object, "del"); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			deleter := operations.NewDeleter(client)

			if recursive {
				deleter.DeleteDir(cmd.Context(), bucket, object, force, concurrent)
			} else {
				deleter.Delete(cmd.Context(), bucket, object, force)
			}
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "递归删除整个目录")
	cmd.Flags().IntVarP(&concurrent, "concurrent", "c", 0, "并发删除数量（仅与 -r 一起使用）")
	cmd.Flags().BoolVar(&force, "force", false, "强制删除，不进行确认提示")
	return cmd
}
