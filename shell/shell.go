package shell

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"minio/operations"
)

// Session 管理交互会话状态
type Session struct {
	client   *minio.Client
	bucket   string // 当前 bucket
	prefix   string // 当前 prefix（远程路径）
	localDir string // 本地工作目录
}

// Shell 交互式命令行
type Shell struct {
	session  *Session
	reader   *bufio.Reader
	commands map[string]Command
}

// Command 命令接口
type Command interface {
	Name() string
	Execute(session *Session, args []string) error
	Help() string
}

// NewShell 创建 Shell
func NewShell(client *minio.Client) *Shell {
	s := &Shell{
		session: &Session{
			client:   client,
			localDir: ".",
		},
		reader: bufio.NewReader(os.Stdin),
	}

	// 注册命令
	s.commands = map[string]Command{
		"use":   &UseCommand{},
		"cd":    &CdCommand{},
		"pwd":   &PwdCommand{},
		"ls":    &LsCommand{},
		"list":  &LsCommand{}, // list 作为 ls 的别名
		"stat":  &StatCommand{},
		"cat":   &CatCommand{},
		"get":   &GetCommand{},
		"put":   &PutCommand{},
		"sign":  &SignCommand{},
		"lls":   &LlsCommand{},
		"lcd":   &LcdCommand{},
		"lpwd":  &LpwdCommand{},
		"clear": &ClearCommand{},
		"help":  &HelpCommand{},
	}

	return s
}

// Run 运行交互模式
func (s *Shell) Run() error {
	fmt.Println("MinIO 交互模式")
	fmt.Println("输入 'help' 查看帮助, 'exit' 退出")

	for {
		// 显示提示符
		prompt := s.prompt()
		fmt.Print(prompt + "> ")

		// 读取输入
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 解析命令
		parts := strings.Fields(line)
		cmdName := parts[0]
		args := parts[1:]

		// 处理退出
		if cmdName == "exit" || cmdName == "quit" {
			fmt.Println("再见!")
			return nil
		}

		// 查找并执行命令
		cmd, ok := s.commands[cmdName]
		if !ok {
			fmt.Printf("未知命令: %s\n输入 'help' 查看可用命令\n", cmdName)
			continue
		}

		if err := cmd.Execute(s.session, args); err != nil {
			fmt.Printf("错误: %v\n", err)
		}
	}
}

func (s *Shell) prompt() string {
	if s.session.bucket == "" {
		return "minio"
	}
	return fmt.Sprintf("%s/%s", s.session.bucket, s.session.prefix)
}

// UseCommand 切换 bucket
type UseCommand struct{}

func (c *UseCommand) Name() string { return "use" }
func (c *UseCommand) Help() string {
	return "use <bucket> - 切换 bucket"
}
func (c *UseCommand) Execute(session *Session, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("用法: use <bucket>")
	}
	bucket := args[0]

	// 验证 bucket 是否存在
	exists, err := session.client.BucketExists(context.Background(), bucket)
	if err != nil {
		return fmt.Errorf("检查 bucket 失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("bucket '%s' 不存在", bucket)
	}

	session.bucket = bucket
	session.prefix = ""
	fmt.Printf("已切换到 bucket: %s\n", bucket)
	return nil
}

// CdCommand 切换 prefix
type CdCommand struct{}

func (c *CdCommand) Name() string { return "cd" }
func (c *CdCommand) Help() string {
	return "cd <prefix> - 切换远程目录 (支持 .. 和 /)"
}
func (c *CdCommand) Execute(session *Session, args []string) error {
	if session.bucket == "" {
		return fmt.Errorf("请先使用 'use' 命令选择 bucket")
	}
	if len(args) != 1 {
		return fmt.Errorf("用法: cd <prefix>")
	}

	target := args[0]
	newPrefix := ""

	switch {
	case target == "/":
		// 回到根目录
		newPrefix = ""
	case target == "..":
		// 上级目录
		parts := strings.Split(strings.TrimSuffix(session.prefix, "/"), "/")
		if len(parts) > 1 {
			newPrefix = strings.Join(parts[:len(parts)-1], "/") + "/"
		}
	case strings.HasPrefix(target, "/"):
		// 绝对路径
		newPrefix = strings.TrimPrefix(target, "/")
		if !strings.HasSuffix(newPrefix, "/") && newPrefix != "" {
			newPrefix += "/"
		}
	default:
		// 相对路径
		newPrefix = session.prefix + target
		if !strings.HasSuffix(newPrefix, "/") && newPrefix != "" {
			newPrefix += "/"
		}
	}

	session.prefix = newPrefix
	fmt.Printf("当前路径: %s/%s\n", session.bucket, newPrefix)
	return nil
}

// PwdCommand 显示当前路径
type PwdCommand struct{}

func (c *PwdCommand) Name() string { return "pwd" }
func (c *PwdCommand) Help() string {
	return "pwd - 显示当前远程路径"
}
func (c *PwdCommand) Execute(session *Session, args []string) error {
	if session.bucket == "" {
		fmt.Println("未选择 bucket")
		return nil
	}
	fmt.Printf("%s/%s\n", session.bucket, session.prefix)
	return nil
}

// LsCommand 列出对象
type LsCommand struct{}

func (c *LsCommand) Name() string { return "ls" }
func (c *LsCommand) Help() string {
	return "ls [-r] - 列出对象 (-r 递归)"
}
func (c *LsCommand) Execute(session *Session, args []string) error {
	if session.bucket == "" {
		return fmt.Errorf("请先使用 'use' 命令选择 bucket")
	}

	recursive := false
	for _, arg := range args {
		if arg == "-r" {
			recursive = true
		}
	}

	lister := operations.NewLister(session.client)
	lister.List(context.Background(), session.bucket, session.prefix, recursive)
	return nil
}

// StatCommand 查询对象元数据
type StatCommand struct{}

func (c *StatCommand) Name() string { return "stat" }
func (c *StatCommand) Help() string {
	return "stat <object> - 查询对象元数据"
}
func (c *StatCommand) Execute(session *Session, args []string) error {
	if session.bucket == "" {
		return fmt.Errorf("请先使用 'use' 命令选择 bucket")
	}
	if len(args) != 1 {
		return fmt.Errorf("用法: stat <object>")
	}

	objectName := args[0]
	// 如果不是完整路径，拼接当前 prefix
	if !strings.Contains(objectName, "/") && session.prefix != "" {
		objectName = session.prefix + objectName
	}

	stater := operations.NewStater(session.client)
	stater.Stat(context.Background(), session.bucket, objectName)
	return nil
}

// CatCommand 输出对象内容
type CatCommand struct{}

func (c *CatCommand) Name() string { return "cat" }
func (c *CatCommand) Help() string {
	return "cat <object> - 输出对象内容到标准输出"
}
func (c *CatCommand) Execute(session *Session, args []string) error {
	if session.bucket == "" {
		return fmt.Errorf("请先使用 'use' 命令选择 bucket")
	}
	if len(args) != 1 {
		return fmt.Errorf("用法: cat <object>")
	}

	objectName := args[0]
	// 如果不是完整路径，拼接当前 prefix
	if !strings.Contains(objectName, "/") && session.prefix != "" {
		objectName = session.prefix + objectName
	}

	catter := operations.NewCatter(session.client)
	catter.Cat(context.Background(), session.bucket, objectName)
	return nil
}

// GetCommand 下载对象
type GetCommand struct{}

func (c *GetCommand) Name() string { return "get" }
func (c *GetCommand) Help() string {
	return "get <object> [local-path] - 下载对象"
}
func (c *GetCommand) Execute(session *Session, args []string) error {
	if session.bucket == "" {
		return fmt.Errorf("请先使用 'use' 命令选择 bucket")
	}
	if len(args) < 1 {
		return fmt.Errorf("用法: get <object> [local-path]")
	}

	objectName := args[0]
	// 如果不是完整路径，拼接当前 prefix
	if !strings.Contains(objectName, "/") && session.prefix != "" {
		objectName = session.prefix + objectName
	}

	outputPath := ""
	if len(args) > 1 {
		outputPath = args[1]
	} else {
		// 默认保存到本地当前目录
		parts := strings.Split(objectName, "/")
		outputPath = filepath.Join(session.localDir, parts[len(parts)-1])
	}

	getter := operations.NewGetter(session.client)
	getter.Get(context.Background(), session.bucket, objectName, outputPath)
	return nil
}

// PutCommand 上传文件
type PutCommand struct{}

func (c *PutCommand) Name() string { return "put" }
func (c *PutCommand) Help() string {
	return "put <local-file> [object-name] - 上传文件"
}
func (c *PutCommand) Execute(session *Session, args []string) error {
	if session.bucket == "" {
		return fmt.Errorf("请先使用 'use' 命令选择 bucket")
	}
	if len(args) < 1 {
		return fmt.Errorf("用法: put <local-file> [object-name]")
	}

	localPath := args[0]
	// 如果是相对路径，基于本地工作目录
	if !filepath.IsAbs(localPath) {
		localPath = filepath.Join(session.localDir, localPath)
	}

	objectName := ""
	if len(args) > 1 {
		objectName = args[1]
	} else {
		objectName = filepath.Base(localPath)
	}
	// 拼接当前 prefix
	if session.prefix != "" {
		objectName = session.prefix + objectName
	}

	putter := operations.NewPutter(session.client)
	putter.Put(context.Background(), session.bucket, objectName, localPath, "")
	return nil
}

// LlsCommand 列出本地目录
type LlsCommand struct{}

func (c *LlsCommand) Name() string { return "lls" }
func (c *LlsCommand) Help() string {
	return "lls [path] - 列出本地目录"
}
func (c *LlsCommand) Execute(session *Session, args []string) error {
	dir := session.localDir
	if len(args) > 0 {
		dir = args[0]
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(session.localDir, dir)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("读取目录失败: %w", err)
	}

	for _, entry := range entries {
		info, _ := entry.Info()
		prefix := " "
		if entry.IsDir() {
			prefix = "d"
		}
		fmt.Printf("%s %10d %s\n", prefix, info.Size(), entry.Name())
	}
	return nil
}

// LcdCommand 切换本地目录
type LcdCommand struct{}

func (c *LcdCommand) Name() string { return "lcd" }
func (c *LcdCommand) Help() string {
	return "lcd <path> - 切换本地目录"
}
func (c *LcdCommand) Execute(session *Session, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("用法: lcd <path>")
	}

	newPath := args[0]
	if !filepath.IsAbs(newPath) {
		newPath = filepath.Join(session.localDir, newPath)
	}

	absPath, err := filepath.Abs(newPath)
	if err != nil {
		return fmt.Errorf("解析路径失败: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("目录不存在: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("不是目录")
	}

	session.localDir = absPath
	fmt.Printf("本地目录: %s\n", absPath)
	return nil
}

// LpwdCommand 显示本地工作目录
type LpwdCommand struct{}

func (c *LpwdCommand) Name() string { return "lpwd" }
func (c *LpwdCommand) Help() string {
	return "lpwd - 显示本地工作目录"
}
func (c *LpwdCommand) Execute(session *Session, args []string) error {
	fmt.Println(session.localDir)
	return nil
}

// ClearCommand 清屏
type ClearCommand struct{}

func (c *ClearCommand) Name() string { return "clear" }
func (c *ClearCommand) Help() string {
	return "clear - 清屏"
}
func (c *ClearCommand) Execute(session *Session, args []string) error {
	fmt.Print("\033[2J\033[H")
	return nil
}

// HelpCommand 显示帮助
type HelpCommand struct{}

func (c *HelpCommand) Name() string { return "help" }
func (c *HelpCommand) Help() string {
	return "help - 显示帮助信息"
}
func (c *HelpCommand) Execute(session *Session, args []string) error {
	fmt.Println("可用命令:")
	fmt.Println("  use <bucket>      切换 bucket")
	fmt.Println("  cd <prefix>       切换远程目录 (支持 .. 和 /)")
	fmt.Println("  pwd               显示当前远程路径")
	fmt.Println("  ls, list [-r]     列出对象 (-r 递归)")
	fmt.Println("  stat <object>     查询对象元数据")
	fmt.Println("  cat <object>      输出对象内容")
	fmt.Println("  get <object>      下载对象")
	fmt.Println("  put <file> [name] 上传文件")
	fmt.Println("  sign <object>     生成签名下载链接")
	fmt.Println("  lls [path]        列出本地目录")
	fmt.Println("  lcd <path>        切换本地目录")
	fmt.Println("  lpwd              显示本地工作目录")
	fmt.Println("  clear             清屏")
	fmt.Println("  help              显示帮助")
	fmt.Println("  exit              退出")
	return nil
}

// SignCommand 生成签名 URL
type SignCommand struct{}

func (c *SignCommand) Name() string { return "sign" }
func (c *SignCommand) Help() string {
	return "sign <object> [expire] - 生成签名下载链接 (expire 如: 1h, 24h, 7d)"
}
func (c *SignCommand) Execute(session *Session, args []string) error {
	if session.bucket == "" {
		return fmt.Errorf("请先使用 'use' 命令选择 bucket")
	}
	if len(args) < 1 {
		return fmt.Errorf("用法: sign <object> [expire]")
	}

	objectName := args[0]
	// 如果不是完整路径，拼接当前 prefix
	if !strings.Contains(objectName, "/") && session.prefix != "" {
		objectName = session.prefix + objectName
	}

	expire := 7 * 24 * time.Hour // 默认 7 天
	if len(args) > 1 {
		d, err := time.ParseDuration(args[1])
		if err != nil {
			return fmt.Errorf("无效的时间格式: %v (支持: 1h, 24h, 7d 等)", err)
		}
		expire = d
	}

	signer := operations.NewSigner(session.client)
	signer.Sign(context.Background(), session.bucket, objectName, expire)
	return nil
}
