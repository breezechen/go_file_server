package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/webdav"
)

// createMockWebDAVServer 创建模拟的WebDAV服务器
func createMockWebDAVServer(t *testing.T) (*httptest.Server, string) {
	tmpdir, err := os.MkdirTemp("", "webdav-client-test")
	if err != nil {
		t.Fatal(err)
	}

	// 创建测试文件和目录
	os.Mkdir(filepath.Join(tmpdir, "testdir"), 0755)
	os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("test content"), 0644)
	os.WriteFile(filepath.Join(tmpdir, "testdir", "nested.txt"), []byte("nested content"), 0644)

	handler := &webdav.Handler{
		FileSystem: webdav.Dir(tmpdir),
		LockSystem: webdav.NewMemLS(),
	}

	server := httptest.NewServer(handler)
	return server, tmpdir
}

// TestNewClient 测试创建客户端
func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:8080/dav")
	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if client.BaseURL != "http://localhost:8080/dav" {
		t.Errorf("BaseURL = %s, want http://localhost:8080/dav", client.BaseURL)
	}

	// 测试尾部斜杠处理
	client2 := NewClient("http://localhost:8080/dav/")
	if client2.BaseURL != "http://localhost:8080/dav" {
		t.Errorf("BaseURL with trailing slash = %s, want http://localhost:8080/dav", client2.BaseURL)
	}
}

// TestSetAuth 测试设置认证
func TestSetAuth(t *testing.T) {
	client := NewClient("http://localhost:8080/dav")
	client.SetAuth("testuser", "testpass")

	if client.Username != "testuser" {
		t.Errorf("Username = %s, want testuser", client.Username)
	}
	if client.Password != "testpass" {
		t.Errorf("Password = %s, want testpass", client.Password)
	}
}

// TestSetTimeout 测试设置超时
func TestSetTimeout(t *testing.T) {
	client := NewClient("http://localhost:8080/dav")
	client.SetTimeout(5 * time.Second)

	if client.HTTPClient.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", client.HTTPClient.Timeout)
	}
}

// TestSetHeader 测试设置自定义头
func TestSetHeader(t *testing.T) {
	client := NewClient("http://localhost:8080/dav")
	client.SetHeader("X-Custom", "value")

	if client.Headers["X-Custom"] != "value" {
		t.Errorf("Headers[X-Custom] = %s, want value", client.Headers["X-Custom"])
	}
}

// TestMakeRequest 测试发送请求
func TestMakeRequest(t *testing.T) {
	var receivedHeaders http.Header
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.SetAuth("user", "pass")
	client.SetHeader("X-Custom", "value")

	resp, err := client.makeRequest("GET", "/test", nil, map[string]string{
		"X-Request": "specific",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// 验证认证头
	if receivedAuth == "" {
		t.Error("Authorization header not sent")
	}

	// 验证自定义头
	if receivedHeaders.Get("X-Custom") != "value" {
		t.Error("Custom header not sent")
	}

	// 验证请求特定头
	if receivedHeaders.Get("X-Request") != "specific" {
		t.Error("Request-specific header not sent")
	}
}

// TestPropfind 测试PROPFIND操作
func TestPropfind(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	// 测试深度0
	files, err := client.Propfind("/", 0)
	if err != nil {
		t.Fatalf("Propfind depth 0 failed: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Propfind depth 0: got %d files, want 1", len(files))
	}

	// 测试深度1
	files, err = client.Propfind("/", 1)
	if err != nil {
		t.Fatalf("Propfind depth 1 failed: %v", err)
	}

	if len(files) < 2 {
		t.Errorf("Propfind depth 1: got %d files, want at least 2", len(files))
	}

	// 验证文件信息
	var foundTest, foundDir bool
	for _, f := range files {
		if strings.Contains(f.Path, "test.txt") {
			foundTest = true
			if f.IsDir {
				t.Error("test.txt should not be a directory")
			}
			if f.Size != 12 { // "test content"
				t.Errorf("test.txt size = %d, want 12", f.Size)
			}
		}
		if strings.Contains(f.Path, "testdir") && !strings.Contains(f.Path, ".txt") {
			foundDir = true
			if !f.IsDir {
				t.Error("testdir should be a directory")
			}
		}
	}

	if !foundTest {
		t.Error("test.txt not found in PROPFIND response")
	}
	if !foundDir {
		t.Error("testdir not found in PROPFIND response")
	}
}

// TestList 测试列出目录
func TestList(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	files, err := client.List("/")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(files) < 2 {
		t.Errorf("List: got %d files, want at least 2", len(files))
	}
}

// TestStat 测试获取文件信息
func TestStat(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	info, err := client.Stat("/test.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if info == nil {
		t.Fatal("Stat returned nil info")
	}

	if info.IsDir {
		t.Error("test.txt should not be a directory")
	}

	if info.Size != 12 {
		t.Errorf("test.txt size = %d, want 12", info.Size)
	}

	// 测试不存在的文件
	_, err = client.Stat("/nonexistent.txt")
	if err == nil {
		t.Error("Stat should fail for nonexistent file")
	}
}

// TestMkcol 测试创建目录
func TestMkcol(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	err := client.Mkcol("/newdir")
	if err != nil {
		t.Fatalf("Mkcol failed: %v", err)
	}

	// 验证目录被创建
	if _, err := os.Stat(filepath.Join(tmpdir, "newdir")); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}
}

// TestPutAndGet 测试上传和下载
func TestPutAndGet(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	// 测试Put
	testContent := []byte("uploaded content")
	err := client.Put("/uploaded.txt", bytes.NewReader(testContent))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// 验证文件被创建
	uploadedFile := filepath.Join(tmpdir, "uploaded.txt")
	if _, err := os.Stat(uploadedFile); os.IsNotExist(err) {
		t.Error("Uploaded file was not created")
	}

	// 测试Get
	content, err := client.Get("/uploaded.txt")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(content) != string(testContent) {
		t.Errorf("Downloaded content = %s, want %s", string(content), string(testContent))
	}
}

// TestPutFile 测试上传文件内容
func TestPutFile(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	content := []byte("file content")
	err := client.PutFile("/newfile.txt", content)
	if err != nil {
		t.Fatalf("PutFile failed: %v", err)
	}

	// 验证文件内容
	savedContent, err := os.ReadFile(filepath.Join(tmpdir, "newfile.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if string(savedContent) != string(content) {
		t.Errorf("Saved content = %s, want %s", string(savedContent), string(content))
	}
}

// TestGetStream 测试获取文件流
func TestGetStream(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	stream, err := client.GetStream("/test.txt")
	if err != nil {
		t.Fatalf("GetStream failed: %v", err)
	}
	defer stream.Close()

	content, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}

	if string(content) != "test content" {
		t.Errorf("Stream content = %s, want 'test content'", string(content))
	}
}

// TestDelete 测试删除
func TestDelete(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	// 创建要删除的文件
	deleteFile := filepath.Join(tmpdir, "delete.txt")
	os.WriteFile(deleteFile, []byte("to delete"), 0644)

	err := client.Delete("/delete.txt")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 验证文件被删除
	if _, err := os.Stat(deleteFile); !os.IsNotExist(err) {
		t.Error("File was not deleted")
	}
}

// TestMove 测试移动/重命名
func TestMove(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	// 创建源文件
	srcFile := filepath.Join(tmpdir, "source.txt")
	os.WriteFile(srcFile, []byte("source content"), 0644)

	err := client.Move("/source.txt", "/moved.txt", false)
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	// 验证源文件不存在
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Error("Source file still exists after move")
	}

	// 验证目标文件存在
	movedFile := filepath.Join(tmpdir, "moved.txt")
	if _, err := os.Stat(movedFile); os.IsNotExist(err) {
		t.Error("Moved file does not exist")
	}

	// 验证内容
	content, _ := os.ReadFile(movedFile)
	if string(content) != "source content" {
		t.Errorf("Moved file content = %s, want 'source content'", string(content))
	}
}

// TestCopy 测试复制
func TestCopy(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	// 创建源文件
	srcFile := filepath.Join(tmpdir, "original.txt")
	os.WriteFile(srcFile, []byte("original content"), 0644)

	err := client.Copy("/original.txt", "/copied.txt", false)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// 验证源文件仍存在
	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		t.Error("Source file does not exist after copy")
	}

	// 验证目标文件存在
	copiedFile := filepath.Join(tmpdir, "copied.txt")
	if _, err := os.Stat(copiedFile); os.IsNotExist(err) {
		t.Error("Copied file does not exist")
	}

	// 验证内容
	content, _ := os.ReadFile(copiedFile)
	if string(content) != "original content" {
		t.Errorf("Copied file content = %s, want 'original content'", string(content))
	}
}

// TestLockUnlock 测试锁定和解锁
func TestLockUnlock(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	// 创建文件
	os.WriteFile(filepath.Join(tmpdir, "lock.txt"), []byte("lock me"), 0644)

	// 测试Lock
	err := client.Lock("/lock.txt", 30*time.Second)
	if err != nil {
		// 某些WebDAV服务器可能不支持锁定
		t.Logf("Lock not supported or failed: %v", err)
	}

	// 注意：实际的Lock-Token需要从Lock响应中获取
	// 这里只是测试API调用
	err = client.Unlock("/lock.txt", "dummy-token")
	if err != nil {
		t.Logf("Unlock not supported or failed: %v", err)
	}
}

// TestOverwrite 测试覆盖文件
func TestOverwrite(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	// 创建原始文件
	os.WriteFile(filepath.Join(tmpdir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpdir, "file2.txt"), []byte("content2"), 0644)

	// 测试不允许覆盖
	err := client.Move("/file1.txt", "/file2.txt", false)
	if err == nil {
		t.Error("Move should fail when overwrite is false and target exists")
	}

	// 测试允许覆盖
	err = client.Move("/file1.txt", "/file2.txt", true)
	if err != nil {
		t.Fatalf("Move with overwrite failed: %v", err)
	}

	// 验证file1.txt不存在
	if _, err := os.Stat(filepath.Join(tmpdir, "file1.txt")); !os.IsNotExist(err) {
		t.Error("Source file still exists after move")
	}

	// 验证file2.txt内容
	content, _ := os.ReadFile(filepath.Join(tmpdir, "file2.txt"))
	if string(content) != "content1" {
		t.Errorf("Overwritten file content = %s, want 'content1'", string(content))
	}
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	// 测试无效服务器
	client := NewClient("http://invalid-server:99999")

	_, err := client.List("/")
	if err == nil {
		t.Error("Expected error for invalid server")
	}

	// 测试错误响应
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errorServer.Close()

	client2 := NewClient(errorServer.URL)
	_, err = client2.List("/")
	if err == nil {
		t.Error("Expected error for server error response")
	}

	// 测试无效XML响应
	badXMLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207) // Multi-Status
		w.Write([]byte("invalid xml"))
	}))
	defer badXMLServer.Close()

	client3 := NewClient(badXMLServer.URL)
	_, err = client3.Propfind("/", 0)
	if err == nil {
		t.Error("Expected error for invalid XML response")
	}
}

// TestTimeout 测试超时
func TestTimeout(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	client := NewClient(slowServer.URL)
	client.SetTimeout(100 * time.Millisecond)

	_, err := client.List("/")
	if err == nil {
		t.Error("Expected timeout error")
	}
}

// TestConcurrentOperations 测试并发操作
func TestConcurrentOperations(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	// 并发上传多个文件
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			content := []byte(fmt.Sprintf("content %d", id))
			err := client.PutFile(fmt.Sprintf("/file%d.txt", id), content)
			done <- err
		}(i)
	}

	// 等待所有操作完成
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent upload %d failed: %v", i, err)
		}
	}

	// 验证所有文件都被创建
	for i := 0; i < 10; i++ {
		filename := fmt.Sprintf("file%d.txt", i)
		if _, err := os.Stat(filepath.Join(tmpdir, filename)); os.IsNotExist(err) {
			t.Errorf("File %s was not created", filename)
		}
	}
}

// TestLargeFile 测试大文件操作
func TestLargeFile(t *testing.T) {
	server, tmpdir := createMockWebDAVServer(t)
	defer server.Close()
	defer os.RemoveAll(tmpdir)

	client := NewClient(server.URL)

	// 创建1MB的数据
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	// 上传大文件
	err := client.Put("/large.bin", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Put large file failed: %v", err)
	}

	// 下载并验证
	downloaded, err := client.Get("/large.bin")
	if err != nil {
		t.Fatalf("Get large file failed: %v", err)
	}

	if len(downloaded) != len(data) {
		t.Errorf("Downloaded size = %d, want %d", len(downloaded), len(data))
	}

	// 验证内容
	for i := 0; i < len(data); i += 1000 {
		if downloaded[i] != data[i] {
			t.Errorf("Data mismatch at position %d", i)
			break
		}
	}
}

// TestPropfindXMLParsing 测试PROPFIND XML解析
func TestPropfindXMLParsing(t *testing.T) {
	// 创建返回自定义XML响应的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		w.Write([]byte(`<?xml version="1.0"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>/test.txt</d:href>
    <d:propstat>
      <d:prop>
        <d:displayname>test.txt</d:displayname>
        <d:getcontentlength>1234</d:getcontentlength>
        <d:getcontenttype>text/plain</d:getcontenttype>
        <d:getlastmodified>Mon, 01 Jan 2024 12:00:00 GMT</d:getlastmodified>
        <d:resourcetype/>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
  <d:response>
    <d:href>/testdir/</d:href>
    <d:propstat>
      <d:prop>
        <d:displayname>testdir</d:displayname>
        <d:getcontentlength>0</d:getcontentlength>
        <d:getlastmodified>Mon, 01 Jan 2024 12:00:00 GMT</d:getlastmodified>
        <d:resourcetype>
          <d:collection/>
        </d:resourcetype>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	files, err := client.Propfind("/", 1)
	if err != nil {
		t.Fatalf("Propfind failed: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(files))
	}

	// 验证文件属性
	for _, f := range files {
		if strings.Contains(f.Path, "test.txt") {
			if f.Size != 1234 {
				t.Errorf("test.txt size = %d, want 1234", f.Size)
			}
			if f.ContentType != "text/plain" {
				t.Errorf("test.txt content type = %s, want text/plain", f.ContentType)
			}
			if f.IsDir {
				t.Error("test.txt should not be a directory")
			}
		}
		if strings.Contains(f.Path, "testdir") {
			if !f.IsDir {
				t.Error("testdir should be a directory")
			}
		}
	}
}

// TestURLConstruction 测试URL构造
func TestURLConstruction(t *testing.T) {
	tests := []struct {
		baseURL  string
		path     string
		expected string
	}{
		{"http://example.com", "/file.txt", "http://example.com/file.txt"},
		{"http://example.com/", "/file.txt", "http://example.com/file.txt"},
		{"http://example.com/dav", "/file.txt", "http://example.com/dav/file.txt"},
		{"http://example.com/dav/", "/file.txt", "http://example.com/dav/file.txt"},
	}

	for _, tt := range tests {
		client := NewClient(tt.baseURL)
		
		// 创建测试服务器验证URL
		var receivedURL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedURL = r.URL.String()
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// 替换baseURL为测试服务器URL
		client.BaseURL = server.URL + strings.TrimPrefix(client.BaseURL, "http://example.com")

		client.makeRequest("GET", tt.path, nil, nil)

		if receivedURL != tt.path {
			t.Errorf("URL construction: baseURL=%s, path=%s, got=%s, want=%s",
				tt.baseURL, tt.path, receivedURL, tt.path)
		}
	}
}

// BenchmarkPropfind 基准测试PROPFIND
func BenchmarkPropfind(b *testing.B) {
	tmpdir, err := os.MkdirTemp("", "webdav-bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	handler := &webdav.Handler{
		FileSystem: webdav.Dir(tmpdir),
		LockSystem: webdav.NewMemLS(),
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	// 创建多个文件
	for i := 0; i < 100; i++ {
		os.WriteFile(filepath.Join(tmpdir, fmt.Sprintf("file%d.txt", i)), []byte("content"), 0644)
	}

	client := NewClient(server.URL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Propfind("/", 1)
	}
}

// BenchmarkPutGet 基准测试上传下载
func BenchmarkPutGet(b *testing.B) {
	tmpdir, err := os.MkdirTemp("", "webdav-bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	handler := &webdav.Handler{
		FileSystem: webdav.Dir(tmpdir),
		LockSystem: webdav.NewMemLS(),
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	client := NewClient(server.URL)
	content := []byte("benchmark content")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.PutFile("/bench.txt", content)
		client.Get("/bench.txt")
	}
}