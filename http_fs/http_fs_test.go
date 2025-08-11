package http_fs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// MockServer 创建一个模拟的 HTTP 文件服务器
func createMockServer(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()

	// 模拟文件列表 API
	mux.HandleFunc("/test-dir", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("json") != "" {
			files := []FileInfo{
				{
					Name:       "file1.txt",
					URL:        "file1.txt",
					Size:       100,
					SizeStr:    "100B",
					ModTime:    time.Now().Unix(),
					ModTimeStr: time.Now().Format("2006-01-02 15:04:05"),
					IsDir:      false,
				},
				{
					Name:       "subdir",
					URL:        "subdir",
					Size:       0,
					SizeStr:    "",
					ModTime:    time.Now().Unix(),
					ModTimeStr: time.Now().Format("2006-01-02 15:04:05"),
					IsDir:      true,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(files)
			return
		}

		// 文件下载
		if r.Method == "GET" {
			w.Write([]byte("file content"))
			return
		}

		// 处理 POST 请求
		if r.Method == "POST" {
			var req map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				method := req["method"].(string)
				switch method {
				case "createDir":
					w.WriteHeader(http.StatusOK)
				case "deleteFile":
					w.WriteHeader(http.StatusOK)
				case "logging":
					w.WriteHeader(http.StatusOK)
				case "download":
					resp := DownloadResponse{TaskId: "task-123"}
					json.NewEncoder(w).Encode(resp)
				default:
					w.WriteHeader(http.StatusBadRequest)
				}
				return
			}

			// 处理文件上传
			if r.ParseMultipartForm(10<<20) == nil {
				w.WriteHeader(http.StatusOK)
				return
			}

			w.WriteHeader(http.StatusBadRequest)
		}
	})

	// 根目录
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("json") != "" {
			files := []FileInfo{
				{
					Name:  "test-dir",
					URL:   "test-dir",
					IsDir: true,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(files)
			return
		}

		if r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Write([]byte("root content"))
	})

	// 下载任务列表
	mux.HandleFunc("/:tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			resp := struct {
				Tasks []DownloadTaskInfo `json:"tasks"`
			}{
				Tasks: []DownloadTaskInfo{
					{
						TaskId:   "task-123",
						Url:      "http://example.com/file.zip",
						Filename: "file.zip",
						Status: &DownloadStatus{
							Status:     "downloading",
							TotalSize:  1000,
							Downloaded: 500,
							Speed:      "1MB/s",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	})

	// 文件内容
	mux.HandleFunc("/file.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test file content"))
	})

	return httptest.NewServer(mux)
}

// TestNewHttpFs 测试创建 HttpFs 实例
func TestNewHttpFs(t *testing.T) {
	server := createMockServer(t)
	defer server.Close()

	fs := NewHttpFs(server.URL)
	if fs == nil {
		t.Fatal("NewHttpFs returned nil")
	}
	if fs.BaseURL != server.URL {
		t.Errorf("BaseURL = %s, want %s", fs.BaseURL, server.URL)
	}
}

// TestNewHttpFsWithOptions 测试带选项创建 HttpFs 实例
func TestNewHttpFsWithOptions(t *testing.T) {
	server := createMockServer(t)
	defer server.Close()

	fs := NewHttpFsWithOptions(server.URL,
		WithTimeout(10*time.Second),
		WithAuth("testuser", "testpass"),
		WithHeaders(map[string]string{"X-Test": "value"}),
	)

	if fs == nil {
		t.Fatal("NewHttpFsWithOptions returned nil")
	}
	if fs.username != "testuser" {
		t.Errorf("username = %s, want testuser", fs.username)
	}
	if fs.password != "testpass" {
		t.Errorf("password = %s, want testpass", fs.password)
	}
	if fs.headers["X-Test"] != "value" {
		t.Errorf("headers[X-Test] = %s, want value", fs.headers["X-Test"])
	}
}

// TestListFiles 测试列出文件
func TestListFiles(t *testing.T) {
	server := createMockServer(t)
	defer server.Close()

	fs := NewHttpFs(server.URL)
	files, err := fs.ListFiles("/test-dir")
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("got %d files, want 2", len(files))
	}

	// 验证文件信息
	var foundFile, foundDir bool
	for _, f := range files {
		if f.Name == "file1.txt" {
			foundFile = true
			if f.IsDir {
				t.Error("file1.txt should not be a directory")
			}
			if f.Size != 100 {
				t.Errorf("file1.txt size = %d, want 100", f.Size)
			}
		}
		if f.Name == "subdir" {
			foundDir = true
			if !f.IsDir {
				t.Error("subdir should be a directory")
			}
		}
	}

	if !foundFile {
		t.Error("file1.txt not found")
	}
	if !foundDir {
		t.Error("subdir not found")
	}
}
