package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"minio/crypto"
)

// NewConfigCmd 创建 config 子命令
func NewConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "配置管理",
		Long: `管理 MinIO 客户端配置，敏感信息加密存储

子命令:
  set     设置配置（加密存储，绑定当前机器）
  show    查看配置（解密显示）`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	configCmd.AddCommand(
		NewConfigSetCmd(),
		NewConfigShowCmd(),
	)

	return configCmd
}

// NewConfigSetCmd 创建 config set 子命令
func NewConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set",
		Short: "设置配置",
		Long: `设置 MinIO 客户端配置，敏感信息加密存储到 ~/.config/minio/minio.conf

加密绑定当前机器，配置文件无法在其他机器解密。`,
		Run: func(cmd *cobra.Command, args []string) {
			setConfig()
		},
	}
}

// NewConfigShowCmd 创建 config show 子命令
func NewConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "查看配置",
		Long: `查看当前配置，解密后显示敏感信息`,
		Run: func(cmd *cobra.Command, args []string) {
			showConfig()
		},
	}
}

func readInput(prompt string) string {
	fmt.Print(prompt)
	input, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(input)
}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.config/minio/minio.conf"
}

func ensureConfigDir() error {
	home, _ := os.UserHomeDir()
	dir := home + "/.config/minio"
	return os.MkdirAll(dir, 0700)
}

func setConfig() {
	fmt.Println("请输入配置信息:")
	endpoint := readInput("Endpoint: ")
	accesskey := readInput("AccessKey: ")
	secretkey := readInput("SecretKey: ")

	if endpoint == "" || accesskey == "" || secretkey == "" {
		fmt.Println("错误: 所有字段都必须填写")
		return
	}

	encryptedAccessKey, err := crypto.Encrypt(accesskey)
	if err != nil {
		fmt.Printf("错误: 加密 AccessKey 失败: %v\n", err)
		return
	}

	encryptedSecretKey, err := crypto.Encrypt(secretkey)
	if err != nil {
		fmt.Printf("错误: 加密 SecretKey 失败: %v\n", err)
		return
	}

	if err := ensureConfigDir(); err != nil {
		fmt.Printf("错误: 创建配置目录失败: %v\n", err)
		return
	}

	configPath := getConfigPath()
	content := fmt.Sprintf("endpoint=%s\naccesskey=enc:aes:%s\nsecretkey=enc:aes:%s\n",
		endpoint, encryptedAccessKey, encryptedSecretKey)

	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		fmt.Printf("错误: 写入配置文件失败: %v\n", err)
		return
	}

	fmt.Printf("配置已保存到 %s（加密存储，绑定当前机器）\n", configPath)
}

func showConfig() {
	configPath := getConfigPath()

	content, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("错误: 读取配置文件失败: %v\n", err)
		return
	}

	lines := strings.Split(string(content), "\n")
	var endpoint, accesskeyEnc, secretkeyEnc string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])
		switch key {
		case "endpoint":
			endpoint = value
		case "accesskey":
			accesskeyEnc = value
		case "secretkey":
			secretkeyEnc = value
		}
	}

	if endpoint == "" || accesskeyEnc == "" || secretkeyEnc == "" {
		fmt.Println("错误: 配置文件格式不正确")
		return
	}

	if !crypto.IsEncrypted(accesskeyEnc) || !crypto.IsEncrypted(secretkeyEnc) {
		fmt.Println("警告: 配置未加密")
		fmt.Printf("Endpoint: %s\n", endpoint)
		fmt.Printf("AccessKey: %s\n", accesskeyEnc)
		fmt.Printf("SecretKey: %s\n", secretkeyEnc)
		return
	}

	accesskeyCipher := accesskeyEnc[8:]
	secretkeyCipher := secretkeyEnc[8:]

	accesskey, err := crypto.Decrypt(accesskeyCipher)
	if err != nil {
		fmt.Printf("错误: 解密失败: %v\n", err)
		return
	}

	secretkey, err := crypto.Decrypt(secretkeyCipher)
	if err != nil {
		fmt.Printf("错误: 解密失败: %v\n", err)
		return
	}

	fmt.Println("配置信息:")
	fmt.Printf("  Endpoint:  %s\n", endpoint)
	fmt.Printf("  AccessKey: %s\n", accesskey)
	fmt.Printf("  SecretKey: %s\n", secretkey)
}
