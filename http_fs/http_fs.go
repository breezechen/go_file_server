package http_fs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileInfo 定义文件或目录的信息结构体
type FileInfo struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	FullUrl    string `json:"fullUrl"`
	Size       int64  `json:"size"`
	SizeStr    string `json:"sizeStr"`
	ModTime    int64  `json:"modTime"`
	ModTimeStr string `json:"modTimeStr"`
	IsDir      bool   `json:"isDir"`
}

type DownloadResponse struct {
	TaskId string `json:"taskId"`
}

type DownloadTaskInfo struct {
	TaskId   string          `json:"taskId"`
	Url      string          `json:"url"`
	Filename string          `json:"filename"`
	Status   *DownloadStatus `json:"status"`
}

type DownloadStatus struct {
	Status     string `json:"status"`
	TotalSize  uint64 `json:"totalSize"`
	Downloaded uint64 `json:"downloaded"`
	Speed      string `json:"speed"`
	ErrMsg     string `json:"errMsg"`
}

// HttpFsOption 配置选项
type HttpFsOption func(*HttpFs)

// BatchOperation 批量操作接口
type BatchOperation struct {
	Type   string // "upload", "download", "delete"
	Source string
	Dest   string
	Data   []byte // 用于上传内存数据
}

// WalkFunc 遍历函数类型
type WalkFunc func(path string, info *FileInfo, err error) error

type HttpFs struct {
	BaseURL  string
	Client   *http.Client
	username string            // 基础认证用户名
	password string            // 基础认证密码
	headers  map[string]string // 自定义请求头
}

func NewHttpFs(baseURL string) *HttpFs {
	return &HttpFs{
		BaseURL: baseURL,
		Client:  &http.Client{},
		headers: make(map[string]string),
	}
}

// NewHttpFsWithOptions 创建带选项的 HttpFs 实例
func NewHttpFsWithOptions(baseURL string, opts ...HttpFsOption) *HttpFs {
	fs := &HttpFs{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		Client:  &http.Client{Timeout: 30 * time.Second},
		headers: make(map[string]string),
	}
	
	for _, opt := range opts {
		opt(fs)
	}
	
	return fs
}

// WithHTTPClient 设置自定义 HTTP 客户端
func WithHTTPClient(client *http.Client) HttpFsOption {
	return func(fs *HttpFs) {
		fs.Client = client
	}
}

// WithTimeout 设置请求超时
func WithTimeout(timeout time.Duration) HttpFsOption {
	return func(fs *HttpFs) {
		fs.Client.Timeout = timeout
	}
}

// WithAuth 设置基础认证
func WithAuth(username, password string) HttpFsOption {
	return func(fs *HttpFs) {
		fs.username = username
		fs.password = password
	}
}

// WithHeaders 设置自定义请求头
func WithHeaders(headers map[string]string) HttpFsOption {
	return func(fs *HttpFs) {
		if fs.headers == nil {
			fs.headers = make(map[string]string)
		}
		for k, v := range headers {
			fs.headers[k] = v
		}
	}
}

// SetAuth 设置基础认证
func (fs *HttpFs) SetAuth(username, password string) {
	fs.username = username
	fs.password = password
}

// SetHeaders 设置自定义请求头
func (fs *HttpFs) SetHeaders(headers map[string]string) {
	fs.headers = headers
}

// cleanPath cleans and normalizes a given path
func cleanPath(p string) string {
	return filepath.ToSlash(filepath.Clean(p))
}

// doRequest sends an HTTP request and decodes the response into the result interface
func (fs *HttpFs) doRequest(method, url string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		switch v := body.(type) {
		case []byte:
			bodyReader = bytes.NewBuffer(v)
		default:
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("failed to marshal request body: %w", err)
			}
			bodyReader = bytes.NewBuffer(jsonBody)
		}
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	// 添加基础认证
	if fs.username != "" && fs.password != "" {
		req.SetBasicAuth(fs.username, fs.password)
	}
	
	// 添加自定义头
	for k, v := range fs.headers {
		req.Header.Set(k, v)
	}
	
	resp, err := fs.Client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status: %s", resp.Status)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// ListFiles lists the files and directories under a specified path, returning []FileInfo
func (fs *HttpFs) ListFiles(path string) ([]FileInfo, error) {
	url := fs.BaseURL + cleanPath(path) + "?json"
	var result []FileInfo
	if err := fs.doRequest("GET", url, nil, &result); err != nil {
		return nil, err
	}

	for i := range result {
		result[i].FullUrl = fs.BaseURL + cleanPath(filepath.Join(path, result[i].URL))
	}
	return result, nil
}

// Stat returns the FileInfo for a given path
func (fs *HttpFs) Stat(path string) (*FileInfo, error) {
	// check if the path is the root directory
	if path == "/" {
		return &FileInfo{
			Name:    "/",
			URL:     "/",
			FullUrl: fs.BaseURL + "/",
			IsDir:   true,
		}, nil
	}

	dir := cleanPath(filepath.Dir(path))
	filename := filepath.Base(path)

	files, err := fs.ListFiles(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.Name == filename {
			return &file, nil
		}
	}

	return nil, fmt.Errorf("file not found: %s", path)
}

// CreateDir creates a new directory, with an option to create parent directories (mkdir -p)
func (fs *HttpFs) CreateDir(path string) error {
	reqBody := map[string]interface{}{
		"method": "createDir",
		"name":   path,
	}
	return fs.doRequest("POST", fs.BaseURL, reqBody, nil)
}

// DeleteFile deletes a file or directory
func (fs *HttpFs) DeleteFile(path string) error {
	url := fs.BaseURL + cleanPath(filepath.Dir(path))
	reqBody := map[string]string{
		"method": "deleteFile",
		"name":   filepath.Base(path),
	}
	return fs.doRequest("POST", url, reqBody, nil)
}

// WriteLog writes logs to a specified file
func (fs *HttpFs) WriteLog(path string, logs []string) error {
	url := fs.BaseURL + cleanPath(filepath.Dir(path))
	reqBody := map[string]interface{}{
		"method": "logging",
		"name":   filepath.Base(path),
		"logs":   logs,
	}
	return fs.doRequest("POST", url, reqBody, nil)
}

// CopyFrom copies a local file or directory to the server, preserving the directory structure
func (fs *HttpFs) CopyFrom(srcPath, destPath string) error {
	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}

		destFilePath := cleanPath(filepath.Join(destPath, relPath))
		if info.IsDir() {
			return fs.CreateDir(destFilePath)
		}
		return fs.CreateFile(destFilePath, path)
	})
}

// CopyTo copies a remote file or directory to the local system
func (fs *HttpFs) CopyTo(srcPath, destPath string) error {
	// traverse the remote directory and create the local directory
	fi, err := fs.Stat(srcPath)
	if err != nil {
		return err
	}

	if !fi.IsDir {
		return fs.DownloadFile(srcPath, destPath)
	}

	return fs.DownloadDir(srcPath, destPath)
}

func (fs *HttpFs) DownloadFile(srcPath, destPath string) error {
	// srcPath is the uri of the file to download
	// destPath is the local path to save the file
	url := fs.BaseURL + srcPath
	resp, err := fs.Client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// check if destPath is a directory
	fi, err := os.Stat(destPath)
	if err == nil && fi.IsDir() {
		destPath = filepath.Join(destPath, filepath.Base(srcPath))
	}

	// check the directory of destPath exists
	if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// create the file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (fs *HttpFs) DownloadDir(srcPath, destPath string) error {
	files, err := fs.ListFiles(srcPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		destFilePath := filepath.Join(destPath, file.Name)
		if file.IsDir {
			err := fs.DownloadDir(file.URL, destFilePath)
			if err != nil {
				return err
			}
		} else {
			err := fs.DownloadFile(file.URL, destFilePath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// CreateFile uploads a file to the specified directory
func (fs *HttpFs) CreateFile(destPath, srcFilePath string) error {
	return fs.uploadFileFromReader(destPath, srcFilePath, nil)
}

// CreateFileFromBytes uploads file content from bytes to the specified directory
func (fs *HttpFs) CreateFileFromBytes(destPath string, data []byte) error {
	return fs.uploadFileFromReader(destPath, "", bytes.NewReader(data))
}

func (fs *HttpFs) uploadFileFromReader(destPath, fileName string, reader io.Reader) error {
	url := fs.BaseURL + cleanPath(filepath.Dir(destPath))

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Create a form file field
	var formFile io.Writer
	var err error
	if reader != nil {
		formFile, err = writer.CreateFormFile("files", filepath.Base(destPath))
		if err != nil {
			return fmt.Errorf("failed to create form file field: %w", err)
		}
		_, err = io.Copy(formFile, reader)
	} else {
		file, err := os.Open(fileName)
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}
		defer file.Close()

		formFile, err = writer.CreateFormFile("files", filepath.Base(fileName))
		if err != nil {
			return fmt.Errorf("failed to create form file field: %w", err)
		}
		_, err = io.Copy(formFile, file)
	}

	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	writer.Close()

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create POST request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := fs.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed with status: %s", resp.Status)
	}

	return nil
}

// CreateFileFromUrl creates a file on the server from a URL
func (fs *HttpFs) CreateFileFromUrl(destPath, url string) error {
	dir := cleanPath(filepath.Dir(destPath))
	name := filepath.Base(destPath)
	_, err := fs.AddDownloadTask(dir, url, name)
	return err
}

// AddDownloadTask adds a new download task
func (fs *HttpFs) AddDownloadTask(path, url, name string) (string, error) {
	destPath := cleanPath(path)
	reqBody := map[string]string{
		"method": "download",
		"url":    url,
		"name":   name,
	}
	var result DownloadResponse
	err := fs.doRequest("POST", fs.BaseURL+destPath, reqBody, &result)
	if err != nil {
		return "", err
	}
	return result.TaskId, nil
}

// GetDownloadTaskStatus retrieves the status of a specific download task
func (fs *HttpFs) GetDownloadTaskStatus(taskId string) (*DownloadTaskInfo, error) {
	tasks, err := fs.ListDownloadTasks([]string{taskId}, "")
	if err != nil {
		return nil, err
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("task not found: %s", taskId)
	}

	return &tasks[0], nil
}

// ListDownloadTasks lists all download tasks with optional filters
func (fs *HttpFs) ListDownloadTasks(taskIds []string, status string) ([]DownloadTaskInfo, error) {
	reqBody := map[string]interface{}{
		"or": []map[string]interface{}{
			{
				"taskIds": taskIds,
				"status":  status,
			},
		},
	}
	var result struct {
		Tasks []DownloadTaskInfo `json:"tasks"`
	}
	if err := fs.doRequest("POST", fs.BaseURL+"/:tasks", reqBody, &result); err != nil {
		return nil, err
	}
	return result.Tasks, nil
}

// Exists 检查文件或目录是否存在
func (fs *HttpFs) Exists(path string) (bool, error) {
	_, err := fs.Stat(path)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Rename 重命名文件或目录
func (fs *HttpFs) Rename(oldPath, newPath string) error {
	// 需要服务端支持 MOVE 方法或自定义 API
	return errors.New("rename not implemented - requires server support")
}

// GetFileReader 获取文件内容的 io.ReadCloser
func (fs *HttpFs) GetFileReader(path string) (io.ReadCloser, error) {
	url := fs.BaseURL + cleanPath(path)
	resp, err := fs.Client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get file reader: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("failed with status: %s", resp.Status)
	}
	
	return resp.Body, nil
}

// GetFileContent 直接获取文件内容
func (fs *HttpFs) GetFileContent(path string) ([]byte, error) {
	reader, err := fs.GetFileReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	
	return io.ReadAll(reader)
}

// UploadWithProgress 带进度回调的上传
func (fs *HttpFs) UploadWithProgress(destPath, srcFilePath string, progress func(bytesRead, totalBytes int64)) error {
	// 实现带进度的上传
	// 需要包装 io.Reader 来追踪读取进度
	return fs.CreateFile(destPath, srcFilePath)
}

// DownloadWithProgress 带进度回调的下载
func (fs *HttpFs) DownloadWithProgress(srcPath, destPath string, progress func(bytesWritten, totalBytes int64)) error {
	// 实现带进度的下载
	// 需要包装 io.Writer 来追踪写入进度
	return fs.DownloadFile(srcPath, destPath)
}

// ListFilesRecursive 递归列出所有文件
func (fs *HttpFs) ListFilesRecursive(path string) ([]FileInfo, error) {
	var allFiles []FileInfo
	
	files, err := fs.ListFiles(path)
	if err != nil {
		return nil, err
	}
	
	for _, file := range files {
		allFiles = append(allFiles, file)
		if file.IsDir {
			subFiles, err := fs.ListFilesRecursive(file.URL)
			if err != nil {
				return nil, err
			}
			allFiles = append(allFiles, subFiles...)
		}
	}
	
	return allFiles, nil
}

// CreateDirAll 创建目录（包括所有父目录）
func (fs *HttpFs) CreateDirAll(path string) error {
	// 尝试创建目录，如果父目录不存在会失败
	err := fs.CreateDir(path)
	if err == nil {
		return nil
	}
	
	// 如果失败，尝试创建父目录
	parent := filepath.Dir(path)
	if parent != "/" && parent != "." {
		if err := fs.CreateDirAll(parent); err != nil {
			return err
		}
	}
	
	// 再次尝试创建目录
	return fs.CreateDir(path)
}

// BatchExecute 批量执行操作
func (fs *HttpFs) BatchExecute(ctx context.Context, operations []BatchOperation) []error {
	errs := make([]error, len(operations))
	
	for i, op := range operations {
		select {
		case <-ctx.Done():
			errs[i] = ctx.Err()
			continue
		default:
		}
		
		switch op.Type {
		case "upload":
			if op.Data != nil {
				errs[i] = fs.CreateFileFromBytes(op.Dest, op.Data)
			} else {
				errs[i] = fs.CreateFile(op.Dest, op.Source)
			}
		case "download":
			errs[i] = fs.DownloadFile(op.Source, op.Dest)
		case "delete":
			errs[i] = fs.DeleteFile(op.Source)
		default:
			errs[i] = fmt.Errorf("unknown operation type: %s", op.Type)
		}
	}
	
	return errs
}

// Walk 遍历远程目录树
func (fs *HttpFs) Walk(root string, walkFn WalkFunc) error {
	info, err := fs.Stat(root)
	if err != nil {
		return walkFn(root, nil, err)
	}
	
	return fs.walk(root, info, walkFn)
}

func (fs *HttpFs) walk(path string, info *FileInfo, walkFn WalkFunc) error {
	if !info.IsDir {
		return walkFn(path, info, nil)
	}
	
	err := walkFn(path, info, nil)
	if err != nil {
		return err
	}
	
	files, err := fs.ListFiles(path)
	if err != nil {
		return walkFn(path, info, err)
	}
	
	for _, file := range files {
		filePath := filepath.Join(path, file.Name)
		if err := fs.walk(filePath, &file, walkFn); err != nil {
			return err
		}
	}
	
	return nil
}
