package http_fs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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

type HttpFs struct {
	BaseURL string
	Client  *http.Client
}

func NewHttpFs(baseURL string) *HttpFs {
	return &HttpFs{
		BaseURL: baseURL,
		Client:  &http.Client{},
	}
}

// doRequest sends an HTTP request and decodes the response into the result interface
func (fs *HttpFs) doRequest(method, url string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
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
	url := fs.BaseURL + path + "?json"
	var result []FileInfo
	if err := fs.doRequest("GET", url, nil, &result); err != nil {
		return nil, err
	}

	for i := range result {
		fullPath := filepath.Join(path, result[i].URL)
		fullPath = filepath.Clean(fullPath)
		fullPath = filepath.ToSlash(fullPath)
		result[i].FullUrl = fs.BaseURL + fullPath
	}
	return result, nil
}

// Stat returns the FileInfo for a given path
func (fs *HttpFs) Stat(path string) (*FileInfo, error) {
	dir := filepath.Dir(path)
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
	url := fs.BaseURL + filepath.ToSlash(filepath.Dir(path))
	reqBody := map[string]string{
		"method": "deleteFile",
		"name":   filepath.Base(path),
	}
	return fs.doRequest("POST", url, reqBody, nil)
}

// WriteLog writes logs to a specified file
func (fs *HttpFs) WriteLog(path string, logs []string) error {
	url := fs.BaseURL + filepath.ToSlash(filepath.Dir(path))
	reqBody := map[string]interface{}{
		"method": "logging",
		"name":   filepath.Base(path),
		"logs":   logs,
	}
	return fs.doRequest("POST", url, reqBody, nil)
}

// Copy copies a local file or directory to the server, preserving the directory structure
func (fs *HttpFs) Copy(srcPath, destPath string) error {
	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}

		destFilePath := filepath.Join(destPath, relPath)
		// windows path separator fix
		destFilePath = filepath.ToSlash(destFilePath)
		if info.IsDir() {
			return fs.CreateDir(destFilePath)
		}
		return fs.CreateFile(destFilePath, path)
	})
}

// CreateFile uploads a file to the specified directory
func (fs *HttpFs) CreateFile(destPath, srcFilePath string) error {
	url := fs.BaseURL + filepath.ToSlash(filepath.Dir(destPath))
	file, err := os.Open(srcFilePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Create a form file field
	formFile, err := writer.CreateFormFile("files", filepath.Base(srcFilePath))
	if err != nil {
		return fmt.Errorf("failed to create form file field: %w", err)
	}

	// Copy the file content to the form file field
	if _, err := io.Copy(formFile, file); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Close the multipart writer to finalize the form data
	writer.Close()

	// Create a new POST request
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create POST request: %w", err)
	}

	// Set the Content-Type header to multipart/form-data
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// Send the request
	resp, err := fs.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for server-side errors
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed with status: %s", resp.Status)
	}

	return nil
}

// AddDownloadTask adds a new download task
func (fs *HttpFs) AddDownloadTask(path, url, name string) (string, error) {
	destPath := filepath.ToSlash(filepath.Dir(path))
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
