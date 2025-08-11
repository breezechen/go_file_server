package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/webdav"
)

// Handler 创建一个 WebDAV 处理器
func NewHandler(rootDir string) http.Handler {
	return &webdav.Handler{
		FileSystem: webdav.Dir(rootDir),
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil {
				fmt.Printf("WebDAV: %s %s - Error: %v\n", r.Method, r.URL.Path, err)
			} else {
				fmt.Printf("WebDAV: %s %s\n", r.Method, r.URL.Path)
			}
		},
	}
}

// HandlerWithOptions 创建一个带选项的 WebDAV 处理器
func NewHandlerWithOptions(opts ...Option) http.Handler {
	h := &webdav.Handler{
		LockSystem: webdav.NewMemLS(),
	}
	
	for _, opt := range opts {
		opt(h)
	}
	
	return h
}

// Option 定义配置选项
type Option func(*webdav.Handler)

// WithFileSystem 设置文件系统
func WithFileSystem(fs webdav.FileSystem) Option {
	return func(h *webdav.Handler) {
		h.FileSystem = fs
	}
}

// WithLockSystem 设置锁系统
func WithLockSystem(ls webdav.LockSystem) Option {
	return func(h *webdav.Handler) {
		h.LockSystem = ls
	}
}

// WithLogger 设置日志记录器
func WithLogger(logger func(*http.Request, error)) Option {
	return func(h *webdav.Handler) {
		h.Logger = logger
	}
}

// WithPrefix 设置路径前缀
func WithPrefix(prefix string) Option {
	return func(h *webdav.Handler) {
		h.Prefix = prefix
	}
}

// CustomFS 实现了一个自定义的 WebDAV 文件系统
type CustomFS struct {
	root      string
	readOnly  bool
	allowList []string // 允许访问的路径列表
	denyList  []string // 禁止访问的路径列表
}

// NewCustomFS 创建一个新的自定义 WebDAV 文件系统
func NewCustomFS(root string) *CustomFS {
	return &CustomFS{
		root:      root,
		readOnly:  false,
		allowList: []string{},
		denyList:  []string{},
	}
}

// SetReadOnly 设置只读模式
func (fs *CustomFS) SetReadOnly(readOnly bool) {
	fs.readOnly = readOnly
}

// SetAllowList 设置允许访问的路径列表
func (fs *CustomFS) SetAllowList(paths []string) {
	fs.allowList = paths
}

// SetDenyList 设置禁止访问的路径列表  
func (fs *CustomFS) SetDenyList(paths []string) {
	fs.denyList = paths
}

// resolvePath 解析并验证路径
func (fs *CustomFS) resolvePath(name string) (string, error) {
	// 清理路径
	name = filepath.Clean("/" + name)
	
	// 转换为系统路径
	fullPath := filepath.Join(fs.root, filepath.FromSlash(name))
	
	// 确保路径在根目录内
	if !strings.HasPrefix(fullPath, fs.root) {
		return "", os.ErrPermission
	}
	
	// 检查访问控制列表
	if len(fs.allowList) > 0 {
		allowed := false
		for _, pattern := range fs.allowList {
			if matched, _ := filepath.Match(pattern, name); matched {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", os.ErrPermission
		}
	}
	
	for _, pattern := range fs.denyList {
		if matched, _ := filepath.Match(pattern, name); matched {
			return "", os.ErrPermission
		}
	}
	
	return fullPath, nil
}

// Mkdir 创建目录
func (fs *CustomFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	if fs.readOnly {
		return os.ErrPermission
	}
	
	path, err := fs.resolvePath(name)
	if err != nil {
		return err
	}
	return os.Mkdir(path, perm)
}

// OpenFile 打开文件
func (fs *CustomFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	if fs.readOnly && flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0 {
		return nil, os.ErrPermission
	}
	
	path, err := fs.resolvePath(name)
	if err != nil {
		return nil, err
	}
	
	f, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}
	
	return &customFile{File: f, name: name}, nil
}

// RemoveAll 删除文件或目录
func (fs *CustomFS) RemoveAll(ctx context.Context, name string) error {
	if fs.readOnly {
		return os.ErrPermission
	}
	
	path, err := fs.resolvePath(name)
	if err != nil {
		return err
	}
	return os.RemoveAll(path)
}

// Rename 重命名文件或目录
func (fs *CustomFS) Rename(ctx context.Context, oldName, newName string) error {
	if fs.readOnly {
		return os.ErrPermission
	}
	
	oldPath, err := fs.resolvePath(oldName)
	if err != nil {
		return err
	}
	
	newPath, err := fs.resolvePath(newName)
	if err != nil {
		return err
	}
	
	return os.Rename(oldPath, newPath)
}

// Stat 获取文件信息
func (fs *CustomFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	path, err := fs.resolvePath(name)
	if err != nil {
		return nil, err
	}
	return os.Stat(path)
}

// customFile 包装了 os.File 以实现 webdav.File 接口
type customFile struct {
	*os.File
	name string
}

// Readdir 读取目录内容
func (f *customFile) Readdir(count int) ([]os.FileInfo, error) {
	return f.File.Readdir(count)
}

// Stat 获取文件信息
func (f *customFile) Stat() (os.FileInfo, error) {
	return f.File.Stat()
}

// Middleware 创建一个 WebDAV 中间件
func Middleware(rootDir string, pathPrefix string) func(http.Handler) http.Handler {
	davHandler := NewHandler(rootDir)
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, pathPrefix) {
				// 调整路径
				r.URL.Path = strings.TrimPrefix(r.URL.Path, pathPrefix)
				if r.URL.Path == "" {
					r.URL.Path = "/"
				}
				davHandler.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}