package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"s3m/crypto"
)

// Config S3M 客户端配置（从 context 派生出的运行期配置）
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// Context 一个 context 的配置
type Context struct {
	Endpoint  string
	UseSSL    bool
	AccessKey string
	SecretKey string
}

// ContextStore 配置文件内存模型
type ContextStore struct {
	Current  string
	Contexts map[string]Context
}

// contextKeyPrefix 扁平 key=value 中 context 字段的前缀，例如 "ctx.prod.endpoint"
const (
	contextKeyPrefix     = "ctx."
	currentContextKey    = "current-context"
	legacyKeyEndpoint    = "endpoint"
	legacyKeyAccessKey   = "accesskey"
	legacyKeySecretKey   = "secretkey"
	legacyKeyUseSSL      = "usessl"
	legacyKeySSL         = "ssl"
	defaultContextName   = "default"
	migratedBackupSuffix = ".bak"
)

// 配置文件搜索路径（按优先级排序）
var configPaths = []string{
	"./s3m.conf",
	"~/.config/s3m/s3m.conf",
	"/etc/s3m/s3m.conf",
}

// LoadConfig 从配置文件加载当前 context 的运行期 Config。
// 保留旧接口；推荐使用 LoadContextStore。
func LoadConfig(configPath string) (*Config, error) {
	cfg, _, err := LoadContextStore(configPath, "")
	return cfg, err
}

// LoadContextStore 加载配置文件，按以下顺序解析请求的 context：
//  1. requestedName 不为空时，优先按 requestedName 查找
//  2. 否则使用 current-context
//  3. 都为空时报错
//
// 返回最终的运行期 Config、实际使用的 context 名称、错误。
func LoadContextStore(configPath, requestedName string) (*Config, string, error) {
	paths := configPaths
	if configPath != "" {
		paths = []string{configPath}
	}

	for _, p := range paths {
		store, err := parseContextStore(expandPath(p))
		if err == nil {
			return resolveContext(store, requestedName, p)
		}
		if !os.IsNotExist(err) {
			return nil, "", fmt.Errorf("读取配置文件 %s 失败: %w", p, err)
		}
	}

	return nil, "", fmt.Errorf("未找到配置文件，请使用 's3m context set <name>' 创建 context")
}

// resolveContext 从 store 中选择目标 context，构造运行期 Config
func resolveContext(store *ContextStore, requestedName, sourcePath string) (*Config, string, error) {
	if len(store.Contexts) == 0 {
		return nil, "", errors.New("配置文件中未找到任何 context")
	}

	name := requestedName
	if name == "" {
		name = store.Current
	}
	if name == "" {
		return nil, "", fmt.Errorf("未指定 context，也未设置 current-context。可用 context: %s", joinNames(store))
	}

	ctx, ok := store.Contexts[name]
	if !ok {
		return nil, "", fmt.Errorf("context %q 不存在。可用 context: %s", name, joinNames(store))
	}

	cfg := &Config{
		Endpoint:  ctx.Endpoint,
		UseSSL:    ctx.UseSSL,
		AccessKey: ctx.AccessKey,
		SecretKey: ctx.SecretKey,
	}
	if err := cfg.Validate(); err != nil {
		return nil, "", fmt.Errorf("context %q 配置无效: %w", name, err)
	}
	return cfg, name, nil
}

func joinNames(store *ContextStore) string {
	names := make([]string, 0, len(store.Contexts))
	for n := range store.Contexts {
		names = append(names, n)
	}
	return strings.Join(names, ", ")
}

// parseContextStore 解析配置文件。
// 触发条件：旧格式（无 ctx.* 键、无 current-context，但存在 endpoint/accesskey/secretkey）→ 自动迁移。
func parseContextStore(path string) (*ContextStore, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	store := &ContextStore{Contexts: map[string]Context{}}
	hasNewFormat := false
	legacy := map[string]string{}
	var legacyUseSSL *bool

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
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}

		// current-context 不带前缀
		if key == currentContextKey {
			store.Current = value
			hasNewFormat = true
			continue
		}

		// 旧格式字段
		if !strings.HasPrefix(key, contextKeyPrefix) {
			switch strings.ToLower(key) {
			case legacyKeyEndpoint, legacyKeyAccessKey, legacyKeySecretKey, legacyKeyUseSSL, legacyKeySSL:
				legacy[strings.ToLower(key)] = value
				if strings.ToLower(key) == legacyKeyUseSSL || strings.ToLower(key) == legacyKeySSL {
					parsed, perr := strconv.ParseBool(value)
					if perr != nil {
						return nil, fmt.Errorf("配置项 %s 值无效: %s（应为 true/false）", key, value)
					}
					legacyUseSSL = &parsed
				}
			}
			continue
		}

		// 新格式：ctx.<name>.<field>
		hasNewFormat = true
		rest := strings.TrimPrefix(key, contextKeyPrefix)
		dot := strings.Index(rest, ".")
		if dot <= 0 {
			continue
		}
		name := rest[:dot]
		field := strings.ToLower(rest[dot+1:])

		ctx := store.Contexts[name]
		switch field {
		case "endpoint":
			ctx.Endpoint = value
		case "usessl", "ssl":
			parsed, perr := strconv.ParseBool(value)
			if perr != nil {
				return nil, fmt.Errorf("配置项 %s 值无效: %s（应为 true/false）", key, value)
			}
			ctx.UseSSL = parsed
		case "auth":
			ak, sk, derr := crypto.DecryptCredentials(value)
			if derr != nil {
				return nil, fmt.Errorf("context %q 解密失败: %w", name, derr)
			}
			ctx.AccessKey = ak
			ctx.SecretKey = sk
		default:
			// 未知字段忽略
		}
		store.Contexts[name] = ctx
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 触发旧格式迁移
	if !hasNewFormat && len(legacy) > 0 {
		migrated, merr := migrateLegacyConfig(path, legacy, legacyUseSSL)
		if merr != nil {
			return nil, merr
		}
		// 迁移后使用新 store
		store = migrated
	}

	return store, nil
}

// migrateLegacyConfig 将旧 s3m.conf 迁移到新格式，备份原文件到 path.bak
func migrateLegacyConfig(path string, legacy map[string]string, useSSL *bool) (*ContextStore, error) {
	ep := legacy[legacyKeyEndpoint]
	akEnc := legacy[legacyKeyAccessKey]
	skEnc := legacy[legacyKeySecretKey]

	if ep == "" || akEnc == "" || skEnc == "" {
		return nil, errors.New("旧配置格式不完整（缺 endpoint/accesskey/secretkey）")
	}

	var ak, sk string
	if crypto.IsEncrypted(akEnc) && crypto.IsEncrypted(skEnc) {
		var err error
		ak, err = crypto.Decrypt(akEnc[crypto.EncryptedPrefixLen:])
		if err != nil {
			return nil, fmt.Errorf("迁移时解密 AccessKey 失败: %w", err)
		}
		sk, err = crypto.Decrypt(skEnc[crypto.EncryptedPrefixLen:])
		if err != nil {
			return nil, fmt.Errorf("迁移时解密 SecretKey 失败: %w", err)
		}
	} else {
		ak = akEnc
		sk = skEnc
	}

	ssl := true
	if useSSL != nil {
		ssl = *useSSL
	}

	store := &ContextStore{
		Current: defaultContextName,
		Contexts: map[string]Context{
			defaultContextName: {
				Endpoint:  ep,
				UseSSL:    ssl,
				AccessKey: ak,
				SecretKey: sk,
			},
		},
	}

	// 备份原文件
	if data, rerr := os.ReadFile(path); rerr == nil {
		_ = os.WriteFile(path+migratedBackupSuffix, data, 0600)
	}

	if err := SaveContextStore(path, store); err != nil {
		return nil, fmt.Errorf("写入新格式配置失败: %w", err)
	}

	fmt.Fprintf(os.Stderr, "检测到旧格式配置，已自动迁移为 context %q（原文件备份为 %s%s）\n",
		defaultContextName, filepath.Base(path), migratedBackupSuffix)

	return store, nil
}

// SaveContextStore 把 store 写到文件（扁平 key=value 格式）
func SaveContextStore(path string, store *ContextStore) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	names := make([]string, 0, len(store.Contexts))
	for n := range store.Contexts {
		names = append(names, n)
	}
	// 稳定顺序：current 在前
	sortStrings(names)
	ordered := []string{}
	if store.Current != "" {
		ordered = append(ordered, store.Current)
	}
	for _, n := range names {
		if n != store.Current {
			ordered = append(ordered, n)
		}
	}

	var b strings.Builder
	b.WriteString("# S3M contexts\n")
	if store.Current != "" {
		fmt.Fprintf(&b, "%s=%s\n", currentContextKey, store.Current)
	}
	for _, name := range ordered {
		ctx, ok := store.Contexts[name]
		if !ok {
			continue
		}
		auth, err := crypto.EncryptCredentials(ctx.AccessKey, ctx.SecretKey)
		if err != nil {
			return fmt.Errorf("context %q 加密失败: %w", name, err)
		}
		fmt.Fprintf(&b, "\n[%s]\n", name)
		fmt.Fprintf(&b, "%s%s.endpoint=%s\n", contextKeyPrefix, name, ctx.Endpoint)
		fmt.Fprintf(&b, "%s%s.usessl=%t\n", contextKeyPrefix, name, ctx.UseSSL)
		fmt.Fprintf(&b, "%s%s.auth=%s%s\n", contextKeyPrefix, name, crypto.EncryptedPrefix, auth)
	}

	return os.WriteFile(path, []byte(b.String()), 0600)
}

// sortStrings 简单字符串排序，避免引入 sort 包
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// expandPath 展开 ~ 为用户主目录
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// GetConfigPath 返回当前用户主目录下的配置路径
func GetConfigPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.config/s3m/s3m.conf"
}

// Validate 验证配置是否完整
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("缺少 endpoint")
	}
	if c.AccessKey == "" {
		return errors.New("缺少 accesskey")
	}
	if c.SecretKey == "" {
		return errors.New("缺少 secretkey")
	}
	return nil
}
