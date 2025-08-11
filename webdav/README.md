# WebDAV Package

WebDAV 功能的模块化实现，包含服务端和客户端两个独立的包。

## 目录结构

```
webdav/
├── server/     # WebDAV 服务端实现
│   └── server.go
├── client/     # WebDAV 客户端实现
│   └── client.go
└── README.md
```

## Server 包

WebDAV 服务端实现，提供标准的 WebDAV 协议支持。

### 基础用法

```go
package main

import (
    "net/http"
    "github.com/breezechen/go_file_server/webdav/server"
)

func main() {
    // 创建 WebDAV 处理器
    handler := server.NewHandler("/path/to/files")
    
    // 作为独立服务器运行
    http.Handle("/dav/", http.StripPrefix("/dav", handler))
    http.ListenAndServe(":8080", nil)
}
```

### 高级配置

```go
// 使用自定义文件系统
fs := server.NewCustomFS("/path/to/files")
fs.SetReadOnly(true)  // 只读模式
fs.SetAllowList([]string{"*.txt", "*.md"})  // 只允许特定文件
fs.SetDenyList([]string{".*", "_*"})  // 禁止隐藏文件

handler := server.NewHandlerWithOptions(
    server.WithFileSystem(fs),
    server.WithLogger(func(r *http.Request, err error) {
        // 自定义日志
        log.Printf("WebDAV: %s %s", r.Method, r.URL.Path)
    }),
    server.WithPrefix("/dav"),
)
```

### 作为中间件使用

```go
// Gin 框架
router := gin.New()
davMiddleware := server.Middleware("/path/to/files", "/dav")
router.Use(func(c *gin.Context) {
    davMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        c.Next()
    })).ServeHTTP(c.Writer, c.Request)
})
```

## Client 包

WebDAV 客户端实现，提供完整的 WebDAV 协议操作。

### 基础用法

```go
package main

import (
    "fmt"
    "log"
    "github.com/breezechen/go_file_server/webdav/client"
)

func main() {
    // 创建客户端
    c := client.NewClient("http://localhost:8080/dav")
    
    // 设置认证
    c.SetAuth("username", "password")
    
    // 列出目录
    files, err := c.List("/")
    if err != nil {
        log.Fatal(err)
    }
    
    for _, file := range files {
        if file.IsDir {
            fmt.Printf("[DIR]  %s\n", file.Name)
        } else {
            fmt.Printf("[FILE] %s (%d bytes)\n", file.Name, file.Size)
        }
    }
}
```

### 文件操作

```go
// 创建目录
err := c.Mkcol("/new-folder")

// 上传文件
data := []byte("Hello, WebDAV!")
err = c.PutFile("/hello.txt", data)

// 使用流上传大文件
file, _ := os.Open("large-file.zip")
defer file.Close()
err = c.Put("/large-file.zip", file)

// 下载文件
content, err := c.Get("/hello.txt")

// 获取文件流
stream, err := c.GetStream("/large-file.zip")
defer stream.Close()

// 删除文件
err = c.Delete("/old-file.txt")

// 移动/重命名
err = c.Move("/old-name.txt", "/new-name.txt", false)

// 复制文件
err = c.Copy("/source.txt", "/backup.txt", true)
```

### 高级功能

```go
// 获取文件信息
info, err := c.Stat("/file.txt")
fmt.Printf("File: %s, Size: %d, ModTime: %v\n", 
    info.Name, info.Size, info.ModTime)

// 递归列出所有文件（深度为 -1 表示无限深度）
allFiles, err := c.Propfind("/", -1)

// 锁定文件
err = c.Lock("/important.doc", 30*time.Minute)

// 解锁文件
err = c.Unlock("/important.doc", lockToken)

// 设置自定义请求头
c.SetHeader("X-Custom-Header", "value")

// 设置超时
c.SetTimeout(60 * time.Second)
```

## 特性

### Server 特性
- ✅ 完整的 WebDAV 协议支持
- ✅ 内存锁系统
- ✅ 自定义文件系统
- ✅ 访问控制（允许/禁止列表）
- ✅ 只读模式
- ✅ 中间件支持
- ✅ 日志记录

### Client 特性
- ✅ 完整的 WebDAV 方法支持
- ✅ 基础认证
- ✅ 流式上传/下载
- ✅ XML 属性解析
- ✅ 文件锁定/解锁
- ✅ 自定义请求头
- ✅ 超时控制

## WebDAV 方法支持

| 方法 | Server | Client | 说明 |
|------|--------|--------|------|
| OPTIONS | ✅ | - | 获取支持的方法 |
| GET | ✅ | ✅ | 下载文件 |
| HEAD | ✅ | - | 获取文件头信息 |
| PUT | ✅ | ✅ | 上传文件 |
| DELETE | ✅ | ✅ | 删除文件/目录 |
| MKCOL | ✅ | ✅ | 创建目录 |
| COPY | ✅ | ✅ | 复制文件/目录 |
| MOVE | ✅ | ✅ | 移动/重命名 |
| PROPFIND | ✅ | ✅ | 获取属性 |
| PROPPATCH | ✅ | - | 修改属性 |
| LOCK | ✅ | ✅ | 锁定资源 |
| UNLOCK | ✅ | ✅ | 解锁资源 |

## 兼容性

- 兼容主流 WebDAV 客户端
  - Windows 资源管理器
  - macOS Finder
  - Linux 文件管理器（Nautilus, Dolphin 等）
  - Mobile 应用（Documents, FE File Explorer 等）
  - 命令行工具（cadaver, davfs2 等）

## 安装

```bash
go get github.com/breezechen/go_file_server/webdav/server
go get github.com/breezechen/go_file_server/webdav/client
```

## 依赖

- `golang.org/x/net/webdav` - WebDAV 协议实现

## 许可证

MIT License