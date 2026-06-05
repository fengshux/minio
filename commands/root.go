package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/spf13/cobra"
	"minio/operations"
	"minio/shell"
)

var (
	configPath string
	debug      bool
	client     *minio.Client
)

// ClientWrapper 包装 minio.Client 以便在 main 包中初始化
type ClientWrapper struct {
	Client *minio.Client
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
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			// 无子命令时进入交互模式
			sh := shell.NewShell(client)
			if err := sh.Run(); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "配置文件路径（默认: ./minio.conf, ~/.config/minio/minio.conf, /etc/minio/minio.conf）")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "启用 HTTP 请求调试输出")

	// 添加子命令
	rootCmd.AddCommand(
		NewPutCmd(),
		NewListCmd(),
		NewStatCmd(),
		NewGetCmd(),
		NewCatCmd(),
		NewSignCmd(),
		NewCopyCmd(),
	)

	return rootCmd
}

// NewListCmd 创建 list 子命令
func NewListCmd() *cobra.Command {
	var recursive bool
	cmd := &cobra.Command{
		Use:   "list bucket [prefix]",
		Short: "列出存储桶中的对象",
		Long: `列出指定存储桶中的所有对象，可选择按前缀过滤和递归列出

参数:
  bucket  存储桶名称（必需）
  prefix  对象前缀过滤（可选，默认列出所有对象）

示例:
  minio list my-bucket                # 列出根级对象
  minio list my-bucket photos/        # 列出 photos/ 前缀下的对象
  minio list my-bucket -r             # 递归列出所有对象
  minio list my-bucket photos/ -r     # 递归列出 photos/ 下所有对象`,
		Args: cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			bucket := args[0]
			prefix := ""
			if len(args) > 1 {
				prefix = args[1]
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
		Use:   "stat bucket object",
		Short: "查询对象元数据",
		Long: `获取指定对象的详细信息，包括大小、ETag、最后修改时间、Content-Type 等

参数:
  bucket  存储桶名称
  object  对象名称（完整路径）

示例:
  minio stat my-bucket file.txt
  minio stat my-bucket photos/2024/image.jpg

输出格式:
  对象名: <key>
  大小: <MB> (<字节>)
  ETag: <etag>
  最后修改时间: <timestamp>
  Content-Type: <type>`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			bucket := args[0]
			objectName := args[1]
			stater := operations.NewStater(client)
			stater.Stat(cmd.Context(), bucket, objectName)
		},
	}
}

// NewGetCmd 创建 get 子命令
func NewGetCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "get bucket object",
		Short: "下载对象到本地",
		Long: `从存储桶下载指定对象到本地文件

参数:
  bucket  存储桶名称
  object  对象名称（完整路径）

选项:
  -o, --output  本地保存路径（默认: 对象名称的最后一部分）

示例:
  minio get my-bucket file.txt                          # 保存为 file.txt
  minio get my-bucket photos/image.jpg -o /tmp/photo.jpg  # 指定路径`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			bucket := args[0]
			objectName := args[1]
			getter := operations.NewGetter(client)
			getter.Get(cmd.Context(), bucket, objectName, output)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "本地保存路径（默认: 对象名称的最后一部分）")
	return cmd
}

// NewCatCmd 创建 cat 子命令
func NewCatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cat bucket object",
		Short: "输出对象内容到控制台",
		Long: `将指定对象的内容直接输出到标准输出

参数:
  bucket  存储桶名称
  object  对象名称（完整路径）

示例:
  minio cat my-bucket file.txt
  minio cat my-bucket logs/app.log`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			bucket := args[0]
			objectName := args[1]
			catter := operations.NewCatter(client)
			catter.Cat(cmd.Context(), bucket, objectName)
		},
	}
}

// NewPutCmd 创建 put 子命令
func NewPutCmd() *cobra.Command {
	var contentType string
	cmd := &cobra.Command{
		Use:   "put bucket object local-file",
		Short: "上传本地文件到存储桶",
		Long: `将本地文件上传到指定存储桶

参数:
  bucket      存储桶名称
  object      对象名称（存储路径）
  local-file  本地文件路径

选项:
  -t, --type  Content-Type（默认: 自动检测）

示例:
  minio put my-bucket file.txt ./local.txt
  minio put my-bucket photos/image.jpg ./photo.jpg
  minio put my-bucket data.json ./data.json -t application/json`,
		Args: cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			bucket := args[0]
			objectName := args[1]
			localPath := args[2]
			putter := operations.NewPutter(client)
			putter.Put(cmd.Context(), bucket, objectName, localPath, contentType)
		},
	}
	cmd.Flags().StringVarP(&contentType, "type", "t", "", "Content-Type（默认: 自动检测）")
	return cmd
}

// NewSignCmd 创建 sign 子命令
func NewSignCmd() *cobra.Command {
	var expire time.Duration
	cmd := &cobra.Command{
		Use:   "sign bucket object",
		Short: "生成带签名的下载链接",
		Long: `为指定对象生成带签名的 HTTP 下载链接

参数:
  bucket  存储桶名称
  object  对象名称（完整路径）

选项:
  -e, --expire  链接有效期（默认: 168h 即 7 天）

示例:
  minio sign my-bucket file.txt
  minio sign my-bucket photos/image.jpg -e 24h
  minio sign my-bucket data.zip -e 1h`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			bucket := args[0]
			objectName := args[1]
			signer := operations.NewSigner(client)
			signer.Sign(cmd.Context(), bucket, objectName, expire)
		},
	}
	cmd.Flags().DurationVarP(&expire, "expire", "e", 7*24*time.Hour, "链接有效期（如: 1h, 24h, 168h）")
	return cmd
}

// NewCopyCmd 创建 copy 子命令
func NewCopyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "copy src-bucket src-object dest-bucket dest-object",
		Short: "复制对象",
		Long: `将对象从一个位置复制到另一个位置，支持跨存储桶复制

参数:
  src-bucket   源存储桶名称
  src-object   源对象名称（完整路径）
  dest-bucket  目标存储桶名称
  dest-object  目标对象名称（完整路径）

示例:
  minio copy my-bucket file.txt my-bucket copy.txt
  minio copy bucket1 photos/image.jpg bucket2 backup/image.jpg`,
		Args: cobra.ExactArgs(4),
		Run: func(cmd *cobra.Command, args []string) {
			srcBucket := args[0]
			srcObject := args[1]
			destBucket := args[2]
			destObject := args[3]
			copier := operations.NewCopier(client)
			copier.Copy(cmd.Context(), srcBucket, srcObject, destBucket, destObject)
		},
	}
}
