package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config MinIO 客户端配置
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// 配置文件搜索路径（按优先级排序）
var configPaths = []string{
	"./minio.conf",
	"~/.config/minio/minio.conf",
	"/etc/minio/minio.conf",
}

// LoadConfig 从配置文件加载配置
// 如果指定了 configPath，则只从该路径读取
// 否则按优先级依次查找配置文件
func LoadConfig(configPath string) (*Config, error) {
	var paths []string
	if configPath != "" {
		paths = []string{configPath}
	} else {
		paths = configPaths
	}

	for _, path := range paths {
		cfg, err := parseConfigFile(expandPath(path))
		if err == nil {
			return cfg, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
		}
	}

	return nil, errors.New("未找到配置文件，请创建 minio.conf 或使用 --config 指定配置文件路径")
}

// expandPath 展开 ~ 为用户主目录
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// parseConfigFile 解析配置文件
func parseConfigFile(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg := &Config{UseSSL: true}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
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
			cfg.Endpoint = value
		case "accesskey":
			cfg.AccessKey = value
		case "secretkey":
			cfg.SecretKey = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate 验证配置是否完整
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("配置文件缺少 endpoint")
	}
	if c.AccessKey == "" {
		return errors.New("配置文件缺少 accesskey")
	}
	if c.SecretKey == "" {
		return errors.New("配置文件缺少 secretkey")
	}
	return nil
}
