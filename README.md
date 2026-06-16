# MinIO CLI

一个用于管理 MinIO/S3 对象存储的命令行工具，支持命令行模式和 TUI 交互模式。

## 功能特性

- **命令行模式**: 单次执行操作，适合脚本和自动化
- **TUI 交互模式**: 基于 Bubble Tea 的终端用户界面，支持分屏布局、进度显示、历史记录
- **加密存储**: 敏感信息（accesskey、secretkey）加密存储，更安全
- **多配置支持**: 支持多个配置文件路径优先级查找

## 安装

```bash
go build -o minio
```

## 配置

### 推荐方式：使用 config 命令（加密存储）

```bash
minio config set
```

交互式输入：
- Endpoint: S3 服务端点
- AccessKey: 访问密钥（加密存储）
- SecretKey: 密钥（加密存储）

配置文件保存到 `~/.config/minio/minio.conf`，格式如下：

```ini
endpoint=s3-internal.cn-north-1-bjps.jdcloud-oss.com
accesskey=enc:aes:加密后的密文
secretkey=enc:aes:加密后的密文
```

### 配置管理命令

```bash
minio config set    # 设置配置（加密存储）
minio config show   # 查看配置（解密显示）
```


### 传统方式：手动创建配置文件（明文）

```ini
endpoint=s3-internal.cn-north-1-bjps.jdcloud-oss.com
accesskey=your-access-key
secretkey=your-secret-key
```

**注意**: 明文配置不推荐，建议使用 `minio config set` 加密存储。

### 配置文件查找优先级

1. `--config` 参数指定的路径
2. 当前目录 `./minio.conf`
3. 用户目录 `~/.config/minio/minio.conf`
4. 系统目录 `/etc/minio/minio.conf`

## 使用方式

### TUI 交互模式

直接运行 `minio` 进入 TUI 界面:

```bash
./minio
```

**界面布局:**

```
┌──────────────────────────────────────────────────────────────────┐
│ MinIO Explorer - bucket-name/prefix/                     F1:帮助 │
├────────────────────────────────────────┬─────────────────────────┤
│  Name                 Size    Modified │ 对象详情: file.txt      │
│  📁 photos/                        -   │ 大小: 1.2 MB            │
│  📄 file.txt          1.2 MB   01-15  │ 修改: 2024-01-15        │
│  📄 data.json         15 KB    01-13  │ ETag: abc123            │
├────────────────────────────────────────┴─────────────────────────┤
│ > get file.txt                                                   │
├──────────────────────────────────────────────────────────────────┤
│ [↑↓]导航 [Enter]选择 [Backspace]返回 [:]命令 [r]刷新 [q]退出     │
└──────────────────────────────────────────────────────────────────┘
```

**快捷键:**

| 快捷键 | 说明 |
|--------|------|
| `↑` `↓` | 导航选择 |
| `Enter` | 进入目录/查看详情 |
| `Backspace` | 返回上级 |
| `:` | 进入命令模式 |
| `r` | 刷新列表 |
| `h` | 查看历史 |
| `Tab` | 切换左右面板 |
| `q` | 退出 |

**命令模式命令:**

| 命令 | 说明 | 示例 |
|------|------|------|
| `use <bucket>` | 切换 bucket | `use my-bucket` |
| `cd <prefix>` | 切换目录 | `cd photos/` |
| `ls [-r]` | 列出对象 | `ls`, `ls -r` |
| `get <object>` | 下载对象 | `get file.txt` |
| `put <file>` | 上传文件 | `put ./local.txt` |
| `sign <object>` | 生成签名链接 | `sign file.txt` |
| `history` | 查看操作历史 | `history` |
| `exit` | 退出 | `exit` |

### 命令行模式

#### 列出对象

```bash
minio list bucket/prefix              # 列出根级对象
minio list bucket/photos/             # 列出指定前缀下的对象
minio list bucket/ -r                 # 递归列出所有对象
minio list bucket/photos/ -r          # 递归列出指定前缀下所有对象
```

#### 查询对象元数据

```bash
minio stat bucket/object              # 获取对象详细信息
minio stat my-bucket/photos/2024/image.jpg
```

#### 下载对象

```bash
minio get bucket/object               # 下载对象（默认保存为对象名）
minio get bucket/object -o /tmp/file  # 指定保存路径
```

#### 输出对象内容

```bash
minio cat bucket/object               # 输出对象内容到 stdout
minio cat my-bucket/logs/app.log
```

#### 上传文件

```bash
minio put bucket/object local-file    # 上传文件
minio put bucket/data.json ./data.json -t application/json  # 指定 Content-Type
```

#### 生成签名下载链接

```bash
minio sign bucket/object              # 生成签名链接（默认 7 天有效）
minio sign bucket/object -e 24h       # 指定 24 小时有效
minio sign bucket/object -e 1h        # 指定 1 小时有效
```

#### 复制对象

```bash
minio copy src-bucket/src-object dest-bucket/dest-object  # 复制单个对象
minio copy bucket1/file.txt bucket2/backup/file.txt       # 示例

# 递归复制目录
minio copy bucket1/photos/ bucket2/backup/photos/ -r      # 逐个复制
minio copy bucket1/photos/ bucket2/backup/photos/ -r -c 5 # 5个并发复制

# 大文件分片复制（用于超过 5GB 的文件）
minio copy bucket1/large.dat bucket2/large-copy.dat -b
minio copy bucket1/large.dat bucket2/large-copy.dat --big
```

#### 删除对象

```bash
minio del bucket/object               # 删除单个对象（需确认）
minio del bucket/object --force       # 删除单个对象（无需确认）

# 递归删除目录
minio del bucket/photos/ -r                # 逐个删除（需确认）
minio del bucket/photos/ -r -c 5           # 5个并发删除（需确认）
minio del bucket/photos/ -r --force        # 逐个删除（无需确认）
minio del bucket/photos/ -r -c 5 --force   # 5个并发删除（无需确认）
```

**删除确认提示：**
- 执行删除前会提示 `确定要删除 xxx 吗？(y/N)`
- 输入 `y` 或 `Y` 确认删除
- 输入其他任何内容取消操作
- 使用 `--force` 选项跳过确认直接删除

### 配置管理

```bash
minio config set                      # 设置配置（加密存储）
minio config show                     # 查看配置（解密显示）
```

## 项目结构

```
minio/
├── main.go           # 程序入口
├── config.go         # 配置文件解析（支持解密）
├── client.go         # MinIO 客户端创建
├── crypto/
│   └── crypto.go     # 加解密（PBKDF2 + AES-256-GCM）
├── commands/
│   ├── root.go       # Cobra 命令定义
│   └── config.go     # config 子命令
├── operations/
│   ├── types.go      # 数据结构定义
│   ├── list.go       # 列出对象
│   ├── stat.go       # 查询元数据
│   ├── get.go        # 下载对象
│   ├── cat.go        # 输出内容
│   ├── put.go        # 上传文件
│   ├── copy.go       # 复制对象（含分片复制）
│   ├── del.go        # 删除对象
│   └── presign.go    # 生成签名 URL
├── tui/
│   ├── model.go      # TUI 状态模型
│   ├── app.go        # Bubble Tea 主程序
│   └── styles.go     # 样式定义
└── README.md         # 说明文档
```

## 依赖

- [Cobra](https://github.com/spf13/cobra) - 命令行解析
- [minio-go](https://github.com/minio/minio-go) - MinIO/S3 SDK
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI 框架
- [golang.org/x/crypto](https://golang.org/x/crypto) - PBKDF2 密钥派生

## 许可证

MIT