# HttpFs - HTTP File System Client Library

HttpFs 是一个 Go 语言的 HTTP 文件系统客户端库，提供了通过 HTTP 协议操作远程文件系统的完整功能。

## 特性

- ✅ 完整的文件操作：列表、创建、删除、上传、下载
- ✅ 目录操作：创建、删除、递归操作
- ✅ 批量操作支持
- ✅ 进度回调
- ✅ 异步下载任务管理
- ✅ 基础认证和自定义请求头
- ✅ WebDAV 协议支持

## 安装

```bash
go get github.com/breezechen/go_file_server/http_fs
```

## 快速开始

### 基础用法

```go
package main

import (
    "fmt"
    "log"
    "github.com/breezechen/go_file_server/http_fs"
)

func main() {
    // 创建客户端
    fs := http_fs.NewHttpFs("http://localhost:9008")
    
    // 列出文件
    files, err := fs.ListFiles("/")
    if err != nil {
        log.Fatal(err)
    }
    
    for _, file := range files {
        fmt.Printf("%s - %d bytes\n", file.Name, file.Size)
    }
    
    // 上传文件
    err = fs.CreateFile("/remote/path/file.txt", "/local/path/file.txt")
    if err != nil {
        log.Fatal(err)
    }
    
    // 下载文件
    err = fs.DownloadFile("/remote/path/file.txt", "/local/download/file.txt")
    if err != nil {
        log.Fatal(err)
    }
}
```

### 高级用法

```go
// 使用选项创建客户端
fs := http_fs.NewHttpFsWithOptions("http://localhost:9008",
    http_fs.WithTimeout(30 * time.Second),
    http_fs.WithAuth("username", "password"),
    http_fs.WithHeaders(map[string]string{
        "X-Custom-Header": "value",
    }),
)

// 检查文件是否存在
exists, err := fs.Exists("/path/to/file")

// 获取文件内容
content, err := fs.GetFileContent("/path/to/file.txt")

// 从内存上传
data := []byte("file content")
err = fs.CreateFileFromBytes("/remote/file.txt", data)

// 递归列出所有文件
allFiles, err := fs.ListFilesRecursive("/")

// 批量操作
ctx := context.Background()
operations := []http_fs.BatchOperation{
    {Type: "upload", Source: "/local/file1.txt", Dest: "/remote/file1.txt"},
    {Type: "upload", Source: "/local/file2.txt", Dest: "/remote/file2.txt"},
    {Type: "download", Source: "/remote/file3.txt", Dest: "/local/file3.txt"},
}
errs := fs.BatchExecute(ctx, operations)

// 遍历远程目录
err = fs.Walk("/", func(path string, info *http_fs.FileInfo, err error) error {
    if err != nil {
        return err
    }
    if info.IsDir {
        fmt.Printf("Directory: %s\n", path)
    } else {
        fmt.Printf("File: %s (%d bytes)\n", path, info.Size)
    }
    return nil
})
```

### WebDAV 支持

```go
// 创建 WebDAV 客户端
client := http_fs.NewWebDAVClient("http://localhost:9008/$.dav$/")

// WebDAV 操作
err := client.Mkcol("/new-folder")
err = client.Delete("/old-file.txt")
err = client.Move("/old-name.txt", "/new-name.txt")
err = client.Copy("/source.txt", "/copy.txt")
```

## API 参考

### 文件操作

- `ListFiles(path string) ([]FileInfo, error)` - 列出目录内容
- `Stat(path string) (*FileInfo, error)` - 获取文件信息
- `Exists(path string) (bool, error)` - 检查文件是否存在
- `CreateFile(destPath, srcFilePath string) error` - 上传文件
- `CreateFileFromBytes(destPath string, data []byte) error` - 从内存上传
- `CreateFileFromUrl(destPath, url string) error` - 从URL下载到服务器
- `DownloadFile(srcPath, destPath string) error` - 下载文件
- `GetFileContent(path string) ([]byte, error)` - 获取文件内容
- `GetFileReader(path string) (io.ReadCloser, error)` - 获取文件流
- `DeleteFile(path string) error` - 删除文件

### 目录操作

- `CreateDir(path string) error` - 创建目录
- `CreateDirAll(path string) error` - 创建目录（包括父目录）
- `DownloadDir(srcPath, destPath string) error` - 下载整个目录
- `CopyFrom(srcPath, destPath string) error` - 上传本地目录
- `CopyTo(srcPath, destPath string) error` - 下载远程目录
- `ListFilesRecursive(path string) ([]FileInfo, error)` - 递归列出文件
- `Walk(root string, walkFn WalkFunc) error` - 遍历目录树

### 异步下载任务

- `AddDownloadTask(path, url, name string) (string, error)` - 添加下载任务
- `GetDownloadTaskStatus(taskId string) (*DownloadTaskInfo, error)` - 获取任务状态
- `ListDownloadTasks(taskIds []string, status string) ([]DownloadTaskInfo, error)` - 列出任务

### 特殊功能

- `WriteLog(path string, logs []string) error` - 写入日志
- `BatchExecute(ctx context.Context, operations []BatchOperation) []error` - 批量执行

## 错误处理

所有方法都返回详细的错误信息，包括：
- 网络错误
- HTTP 状态码错误
- 文件不存在错误
- 权限错误

```go
_, err := fs.Stat("/nonexistent")
if err != nil {
    if strings.Contains(err.Error(), "not found") {
        // 文件不存在
    } else {
        // 其他错误
    }
}
```

## 性能优化

- 使用连接池和 keep-alive
- 支持超时设置
- 批量操作减少网络往返
- 流式传输大文件

## 许可证

MIT License