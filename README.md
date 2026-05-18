# MinIO CLI

一个用于管理 MinIO/S3 对象存储的命令行工具，支持命令行模式和交互式 Shell 模式。

## 功能特性

- **命令行模式**: 单次执行操作，适合脚本和自动化
- **交互模式**: 类似 FTP/SFTP 的交互式 Shell，适合日常操作
- **多配置支持**: 支持多个配置文件路径优先级查找

## 安装

```bash
go build -o minio
```

## 配置

配置文件格式 (`minio.conf`):

```ini
endpoint=s3-internal.cn-north-1-bjps.jdcloud-oss.com
accesskey=your-access-key
secretkey=your-secret-key
```

配置文件查找优先级:

1. `--config` 参数指定的路径
2. 当前目录 `./minio.conf`
3. 用户目录 `~/.config/minio/minio.conf`
4. 系统目录 `/etc/minio/minio.conf`

## 使用方式

### 交互模式

直接运行 `minio` 进入交互式 Shell:

```bash
./minio
```

交互模式命令:

| 命令 | 说明 | 示例 |
|------|------|------|
| `use <bucket>` | 切换当前 bucket | `use my-bucket` |
| `cd <prefix>` | 切换当前 prefix，支持 `..` 和 `/` | `cd photos/`, `cd ..`, `cd /` |
| `pwd` | 显示当前远程路径 | `pwd` |
| `ls`, `list [-r]` | 列出当前 prefix 下的对象 | `ls`, `list`, `ls -r`, `list -r` |
| `stat <object>` | 查询对象元数据 | `stat file.txt` |
| `cat <object>` | 输出对象内容到标准输出 | `cat logs/app.log` |
| `get <object>` | 下载对象到本地当前目录 | `get file.txt` |
| `put <file> [name]` | 上传本地文件到当前 prefix | `put ./local.txt`, `put ./local.txt remote.txt` |
| `sign <object> [expire]` | 生成签名下载链接 | `sign file.txt`, `sign file.txt 24h` |
| `lls [path]` | 列出本地目录内容 | `lls`, `lls /tmp` |
| `lcd <path>` | 切换本地工作目录 | `lcd /tmp` |
| `lpwd` | 显示当前本地工作目录 | `lpwd` |
| `clear` | 清屏 | `clear` |
| `help` | 显示帮助信息 | `help` |
| `exit` / `quit` | 退出交互模式 | `exit` |

交互模式示例:

```
minio> use test-bucket
已切换到 bucket: test-bucket
test-bucket/> ls
        1024 file.txt
d        0 photos/
test-bucket/> cd photos/
当前路径: test-bucket/photos/
test-bucket/photos/> ls -r
        2048 2024/image1.jpg
        3072 2024/image2.jpg
test-bucket/photos/> get 2024/image1.jpg
下载成功: image1.jpg (2.00 KB)
test-bucket/photos/> lcd /tmp
本地目录: /tmp
test-bucket/photos/> lls
d        0 Downloads
       1024 test.txt
test-bucket/photos/> exit
再见!
```

### 命令行模式

#### 列出对象

```bash
minio list bucket [prefix]           # 列出根级对象
minio list bucket photos/            # 列出指定前缀下的对象
minio list bucket -r                 # 递归列出所有对象
minio list bucket photos/ -r         # 递归列出指定前缀下所有对象
```

#### 查询对象元数据

```bash
minio stat bucket object             # 获取对象详细信息
minio stat my-bucket photos/2024/image.jpg
```

#### 下载对象

```bash
minio get bucket object              # 下载对象（默认保存为对象名）
minio get bucket object -o /tmp/file # 指定保存路径
```

#### 输出对象内容

```bash
minio cat bucket object              # 输出对象内容到 stdout
minio cat my-bucket logs/app.log
```

#### 上传文件

```bash
minio put bucket object local-file   # 上传文件
minio put bucket data.json ./data.json -t application/json  # 指定 Content-Type
```

#### 生成签名下载链接

```bash
minio sign bucket object              # 生成签名链接（默认 7 天有效）
minio sign bucket object -e 24h       # 指定 24 小时有效
minio sign bucket object -e 1h        # 指定 1 小时有效
```

## 项目结构

```
minio/
├── main.go           # 程序入口
├── config.go         # 配置文件解析
├── client.go         # MinIO 客户端创建
├── commands/
│   └── root.go       # Cobra 命令定义
├── operations/
│   ├── list.go       # 列出对象
│   ├── stat.go       # 查询元数据
│   ├── get.go        # 下载对象
│   ├── cat.go        # 输出内容
│   ├── put.go        # 上传文件
│   └── presign.go    # 生成签名 URL
├── shell/
│   └── shell.go      # 交互式 Shell 实现
└── spec.md           # 交互模式设计文档
```

## 依赖

- [Cobra](https://github.com/spf13/cobra) - 命令行解析
- [minio-go](https://github.com/minio/minio-go) - MinIO/S3 SDK

## 许可证

MIT