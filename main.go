package main

import (
	"fmt"
	"os"

	"s3m/commands"
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
	ops := &contextOps{}
	commands.CtxOps = commands.ContextOps{
		ConfigPathFn:     ops.ConfigPath,
		ReadOnlyFn:       ops.ReadOnly,
		ListFn:           ops.List,
		CurrentFn:        ops.CurrentName,
		SetCurrentFn:     ops.SetCurrent,
		ShowFn:           ops.Show,
		UpsertFn:         ops.Upsert,
		RenameFn:         ops.Rename,
		DeleteFn:         ops.Delete,
		ImportFromFileFn: ops.ImportFromFile,
	}

	commands.SetExternalConfigPath = func(path string) {
		ops.configPath = path
		ops.readOnly = path != ""
	}

	commands.InitClient = func(configPath, contextName string, debug bool) (*commands.ClientWrapper, string, error) {
		if configPath != "" {
			ops.configPath = configPath
			ops.readOnly = true
		}
		cfg, used, err := LoadContextStore(configPath, contextName)
		if err != nil {
			return nil, "", err
		}

		client, err := NewClient(cfg, debug)
		if err != nil {
			return nil, "", fmt.Errorf("初始化客户端失败: %w", err)
		}

		core, err := NewCoreClient(cfg, debug)
		if err != nil {
			return nil, "", fmt.Errorf("初始化 Core 客户端失败: %w", err)
		}

		return &commands.ClientWrapper{Client: client, Core: core}, used, nil
	}
}
