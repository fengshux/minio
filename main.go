package main

import (
	"fmt"
	"os"

	"minio/commands"
)

func main() {
	rootCmd := commands.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// init 初始化包，设置命令包的回调函数
func init() {
	commands.InitClient = func(configPath string) (*commands.ClientWrapper, error) {
		cfg, err := LoadConfig(configPath)
		if err != nil {
			return nil, err
		}

		if err := cfg.Validate(); err != nil {
			return nil, err
		}

		client, err := NewClient(cfg)
		if err != nil {
			return nil, fmt.Errorf("初始化客户端失败: %w", err)
		}

		return &commands.ClientWrapper{Client: client}, nil
	}
}
