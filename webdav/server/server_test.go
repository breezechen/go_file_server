package server

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewHandler 测试创建处理器
func TestNewHandler(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "webdav-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	handler := NewHandler(tmpdir)
	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}
}

// TestNewHandlerWithOptions 测试带选项创建处理器
func TestNewHandlerWithOptions(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "webdav-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	var logCalled bool
	handler := NewHandlerWithOptions(
		WithFileSystem(NewCustomFS(tmpdir)),
		WithLogger(func(r *http.Request, err error) {
			logCalled = true
		}),
		WithPrefix("/dav"),
	)

	if handler == nil {
		t.Fatal("NewHandlerWithOptions returned nil")
	}

	// 测试日志回调
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !logCalled {
		t.Error("Logger callback was not called")
	}
}

// TestWebDAVMethods 测试WebDAV方法
func TestWebDAVMethods(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "webdav-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	// 创建测试文件
	testFile := filepath.Join(tmpdir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(tmpdir)
	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		headers    map[string]string
		wantStatus int
	}{
		{
			name:       "OPTIONS",
			method:     "OPTIONS",
			path:       "/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "GET file",
			method:     "GET",
			path:       "/test.txt",
			wantStatus: http.StatusOK,
		},
		{
			name:       "HEAD file",
			method:     "HEAD",
			path:       "/test.txt",
			wantStatus: http.StatusOK,
		},
		{
			name:       "PUT new file",
			method:     "PUT",
			path:       "/new.txt",
			body:       "new content",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "MKCOL",
			method:     "MKCOL",
			path:       "/newdir",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "DELETE file",
			method:     "DELETE",
			path:       "/test.txt",
			wantStatus: http.StatusNoContent,
		},
		{
			name:   "PROPFIND",
			method: "PROPFIND",
			path:   "/",
			body: `<?xml version="1.0"?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:displayname/>
    <d:getcontentlength/>
  </d:prop>
</d:propfind>`,
			headers:    map[string]string{"Depth": "1"},
			wantStatus: http.StatusMultiStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}

			req, err := http.NewRequest(tt.method, server.URL+tt.path, body)
			if err != nil {
				t.Fatal(err)
			}

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Got status %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

// TestMoveAndCopy 测试MOVE和COPY操作
func TestMoveAndCopy(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "webdav-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	// 创建测试文件
	srcFile := filepath.Join(tmpdir, "source.txt")
	err = os.WriteFile(srcFile, []byte("source content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(tmpdir)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 测试COPY
	req, err := http.NewRequest("COPY", server.URL+"/source.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Destination", server.URL+"/copied.txt")
	req.Header.Set("Overwrite", "F")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		t.Errorf("COPY: Got status %d, want 201 or 204", resp.StatusCode)
	}

	// 验证复制的文件存在
	copiedFile := filepath.Join(tmpdir, "copied.txt")
	if _, err := os.Stat(copiedFile); os.IsNotExist(err) {
		t.Error("Copied file does not exist")
	}

	// 测试MOVE
	req, err = http.NewRequest("MOVE", server.URL+"/source.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Destination", server.URL+"/moved.txt")
	req.Header.Set("Overwrite", "T")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		t.Errorf("MOVE: Got status %d, want 201 or 204", resp.StatusCode)
	}

	// 验证源文件不存在
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Error("Source file still exists after MOVE")
	}

	// 验证移动后的文件存在
	movedFile := filepath.Join(tmpdir, "moved.txt")
	if _, err := os.Stat(movedFile); os.IsNotExist(err) {
		t.Error("Moved file does not exist")
	}
}

// TestLockUnlock 测试LOCK和UNLOCK操作
func TestLockUnlock(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "webdav-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	handler := NewHandler(tmpdir)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 测试LOCK
	lockBody := `<?xml version="1.0"?>
<d:lockinfo xmlns:d="DAV:">
  <d:lockscope><d:exclusive/></d:lockscope>
  <d:locktype><d:write/></d:locktype>
</d:lockinfo>`

	req, err := http.NewRequest("LOCK", server.URL+"/test.txt", strings.NewReader(lockBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Timeout", "Second-3600")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("LOCK: Got status %d, want 200 or 201", resp.StatusCode)
	}

	// 获取Lock-Token
	lockToken := resp.Header.Get("Lock-Token")

	// 测试UNLOCK
	req, err = http.NewRequest("UNLOCK", server.URL+"/test.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	if lockToken != "" {
		req.Header.Set("Lock-Token", lockToken)
	}

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Errorf("UNLOCK: Got status %d, want 204 or 200", resp.StatusCode)
	}
}

// TestCustomFS 测试自定义文件系统
func TestCustomFS(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "webdav-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	// 创建测试文件
	testFile := filepath.Join(tmpdir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	hiddenFile := filepath.Join(tmpdir, ".hidden")
	err = os.WriteFile(hiddenFile, []byte("hidden"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 测试只读模式
	fs := NewCustomFS(tmpdir)
	fs.SetReadOnly(true)

	handler := NewHandlerWithOptions(WithFileSystem(fs))
	server := httptest.NewServer(handler)
	defer server.Close()

	// 尝试PUT（应该失败）
	req, err := http.NewRequest("PUT", server.URL+"/new.txt", strings.NewReader("content"))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("PUT in read-only mode: Got status %d, want 403 or 405", resp.StatusCode)
	}

	// 测试DenyList
	fs.SetReadOnly(false)
	fs.SetDenyList([]string{".*"})

	// 尝试访问隐藏文件（应该失败）
	req, err = http.NewRequest("GET", server.URL+"/.hidden", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden {
		t.Errorf("GET denied file: Got status %d, want 404 or 403", resp.StatusCode)
	}

	// 测试AllowList
	fs.SetAllowList([]string{"*.txt"})
	fs.SetDenyList(nil)

	// 访问.txt文件（应该成功）
	req, err = http.NewRequest("GET", server.URL+"/test.txt", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET allowed file: Got status %d, want 200", resp.StatusCode)
	}
}

// TestMiddleware 测试中间件功能
func TestMiddleware(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "webdav-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	// 使用中间件
	middleware := Middleware(tmpdir, "/dav")

	// 创建测试服务器
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("regular handler"))
	})

	handler := middleware(mux)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 测试WebDAV路径
	req, err := http.NewRequest("PROPFIND", server.URL+"/dav/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Depth", "0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("WebDAV path: Got status %d, want 207", resp.StatusCode)
	}

	// 测试非WebDAV路径
	req, err = http.NewRequest("GET", server.URL+"/other", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "regular handler" {
		t.Errorf("Non-WebDAV path: Got %s, want 'regular handler'", string(body))
	}
}

// TestPROPFINDParsing 测试PROPFIND响应解析
func TestPROPFINDParsing(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "webdav-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	// 创建测试结构
	os.Mkdir(filepath.Join(tmpdir, "dir1"), 0755)
	os.WriteFile(filepath.Join(tmpdir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpdir, "dir1", "file2.txt"), []byte("content2"), 0644)

	handler := NewHandler(tmpdir)
	server := httptest.NewServer(handler)
	defer server.Close()

	propfindBody := `<?xml version="1.0"?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:displayname/>
    <d:getcontentlength/>
    <d:getcontenttype/>
    <d:getlastmodified/>
    <d:resourcetype/>
  </d:prop>
</d:propfind>`

	req, err := http.NewRequest("PROPFIND", server.URL+"/", strings.NewReader(propfindBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("Got status %d, want 207", resp.StatusCode)
	}

	// 解析响应
	var multistatus struct {
		XMLName  xml.Name `xml:"multistatus"`
		Response []struct {
			Href string `xml:"href"`
			Propstat struct {
				Prop struct {
					DisplayName    string `xml:"displayname"`
					ContentLength  int64  `xml:"getcontentlength"`
					ContentType    string `xml:"getcontenttype"`
					LastModified   string `xml:"getlastmodified"`
					ResourceType   struct {
						Collection *struct{} `xml:"collection"`
					} `xml:"resourcetype"`
				} `xml:"prop"`
				Status string `xml:"status"`
			} `xml:"propstat"`
		} `xml:"response"`
	}

	err = xml.NewDecoder(resp.Body).Decode(&multistatus)
	if err != nil {
		t.Fatalf("Failed to parse PROPFIND response: %v", err)
	}

	// 验证响应包含预期的资源
	foundRoot := false
	foundDir1 := false
	foundFile1 := false

	for _, r := range multistatus.Response {
		if strings.HasSuffix(r.Href, "/") && !strings.HasSuffix(r.Href, "dir1/") {
			foundRoot = true
		}
		if strings.HasSuffix(r.Href, "dir1/") {
			foundDir1 = true
			if r.Propstat.Prop.ResourceType.Collection == nil {
				t.Error("dir1 should be a collection")
			}
		}
		if strings.HasSuffix(r.Href, "file1.txt") {
			foundFile1 = true
			if r.Propstat.Prop.ContentLength != 8 {
				t.Errorf("file1.txt size = %d, want 8", r.Propstat.Prop.ContentLength)
			}
		}
	}

	if !foundRoot {
		t.Error("Root directory not found in PROPFIND response")
	}
	if !foundDir1 {
		t.Error("dir1 not found in PROPFIND response")
	}
	if !foundFile1 {
		t.Error("file1.txt not found in PROPFIND response")
	}
}

// TestPUTLargeFile 测试上传大文件
func TestPUTLargeFile(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "webdav-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	handler := NewHandler(tmpdir)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 创建5MB的数据
	data := make([]byte, 5*1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	req, err := http.NewRequest("PUT", server.URL+"/large.bin", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		t.Errorf("PUT large file: Got status %d, want 201 or 204", resp.StatusCode)
	}

	// 验证文件大小
	fileInfo, err := os.Stat(filepath.Join(tmpdir, "large.bin"))
	if err != nil {
		t.Fatal(err)
	}

	if fileInfo.Size() != int64(len(data)) {
		t.Errorf("Large file size = %d, want %d", fileInfo.Size(), len(data))
	}
}

// TestConcurrentRequests 测试并发请求
func TestConcurrentRequests(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "webdav-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	handler := NewHandler(tmpdir)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 并发创建多个文件
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			content := []byte(strings.Repeat("x", id*100))
			req, err := http.NewRequest("PUT", 
				server.URL+"/file"+string(rune('0'+id))+".txt", 
				bytes.NewReader(content))
			if err != nil {
				done <- err
				return
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				done <- err
				return
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
				done <- err
				return
			}
			done <- nil
		}(i)
	}

	// 等待所有请求完成
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent request %d failed: %v", i, err)
		}
	}

	// 验证所有文件都被创建
	files, err := os.ReadDir(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 10 {
		t.Errorf("Created %d files, want 10", len(files))
	}
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "webdav-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	handler := NewHandler(tmpdir)
	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "GET nonexistent file",
			method:     "GET",
			path:       "/nonexistent.txt",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "DELETE nonexistent file",
			method:     "DELETE",
			path:       "/nonexistent.txt",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "MKCOL on existing path",
			method:     "MKCOL",
			path:       "/",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, server.URL+tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Got status %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

// BenchmarkPUT 基准测试PUT操作
func BenchmarkPUT(b *testing.B) {
	tmpdir, err := os.MkdirTemp("", "webdav-bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	handler := NewHandler(tmpdir)
	server := httptest.NewServer(handler)
	defer server.Close()

	content := []byte("benchmark content")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("PUT",
			server.URL+"/bench.txt",
			bytes.NewReader(content))
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
	}
}

// BenchmarkPROPFIND 基准测试PROPFIND操作
func BenchmarkPROPFIND(b *testing.B) {
	tmpdir, err := os.MkdirTemp("", "webdav-bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	// 创建一些测试文件
	for i := 0; i < 10; i++ {
		os.WriteFile(filepath.Join(tmpdir, "file"+string(rune('0'+i))+".txt"), []byte("content"), 0644)
	}

	handler := NewHandler(tmpdir)
	server := httptest.NewServer(handler)
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("PROPFIND", server.URL+"/", nil)
		req.Header.Set("Depth", "1")
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
	}
}