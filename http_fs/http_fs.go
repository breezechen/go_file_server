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
