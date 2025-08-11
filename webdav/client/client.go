package client

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// Client 是一个 WebDAV 客户端
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Username   string
	Password   string
	Headers    map[string]string
}

// NewClient 创建一个新的 WebDAV 客户端
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    strings.TrimSuffix(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Headers:    make(map[string]string),
	}
}

// SetAuth 设置基础认证
func (c *Client) SetAuth(username, password string) {
	c.Username = username
	c.Password = password
}

// SetTimeout 设置超时
func (c *Client) SetTimeout(timeout time.Duration) {
	c.HTTPClient.Timeout = timeout
}

// SetHeader 设置自定义请求头
func (c *Client) SetHeader(key, value string) {
	c.Headers[key] = value
}

// makeRequest 发送 WebDAV 请求
func (c *Client) makeRequest(method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	reqURL := c.BaseURL + path
	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, err
	}
	
	// 设置基础认证
	if c.Username != "" && c.Password != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
	
	// 设置自定义头
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	
	// 设置请求特定的头
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	
	return c.HTTPClient.Do(req)
}

// PropfindResponse PROPFIND 响应结构
type PropfindResponse struct {
	XMLName  xml.Name   `xml:"multistatus"`
	Response []Response `xml:"response"`
}

// Response WebDAV 响应
type Response struct {
	Href     string   `xml:"href"`
	Propstat Propstat `xml:"propstat"`
}

// Propstat 属性状态
type Propstat struct {
	Prop   Prop   `xml:"prop"`
	Status string `xml:"status"`
}

// Prop 属性
type Prop struct {
	DisplayName      string    `xml:"displayname"`
	GetContentLength int64     `xml:"getcontentlength"`
	GetContentType   string    `xml:"getcontenttype"`
	GetLastModified  string    `xml:"getlastmodified"`
	ResourceType     *Resource `xml:"resourcetype"`
}

// Resource 资源类型
type Resource struct {
	Collection *struct{} `xml:"collection"`
}

// FileInfo 文件信息
type FileInfo struct {
	Name         string
	Size         int64
	ModTime      time.Time
	IsDir        bool
	ContentType  string
	Path         string
}

// Propfind 执行 PROPFIND 请求
func (c *Client) Propfind(path string, depth int) ([]FileInfo, error) {
	headers := map[string]string{
		"Depth":        fmt.Sprintf("%d", depth),
		"Content-Type": "application/xml",
	}
	
	body := `<?xml version="1.0"?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:displayname/>
    <d:getcontentlength/>
    <d:getcontenttype/>
    <d:getlastmodified/>
    <d:resourcetype/>
  </d:prop>
</d:propfind>`
	
	resp, err := c.makeRequest("PROPFIND", path, strings.NewReader(body), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusMultiStatus {
		return nil, fmt.Errorf("PROPFIND failed with status: %s", resp.Status)
	}
	
	var propfindResp PropfindResponse
	if err := xml.NewDecoder(resp.Body).Decode(&propfindResp); err != nil {
		return nil, err
	}
	
	files := make([]FileInfo, 0, len(propfindResp.Response))
	for _, r := range propfindResp.Response {
		fi := FileInfo{
			Name:        filepath.Base(r.Href),
			Path:        r.Href,
			Size:        r.Propstat.Prop.GetContentLength,
			ContentType: r.Propstat.Prop.GetContentType,
			IsDir:       r.Propstat.Prop.ResourceType != nil && r.Propstat.Prop.ResourceType.Collection != nil,
		}
		
		if r.Propstat.Prop.GetLastModified != "" {
			if t, err := time.Parse(time.RFC1123, r.Propstat.Prop.GetLastModified); err == nil {
				fi.ModTime = t
			}
		}
		
		files = append(files, fi)
	}
	
	return files, nil
}

// List 列出目录内容
func (c *Client) List(path string) ([]FileInfo, error) {
	return c.Propfind(path, 1)
}

// Stat 获取文件信息
func (c *Client) Stat(path string) (*FileInfo, error) {
	files, err := c.Propfind(path, 0)
	if err != nil {
		return nil, err
	}
	
	if len(files) == 0 {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	
	return &files[0], nil
}

// Mkcol 创建集合（目录）
func (c *Client) Mkcol(path string) error {
	resp, err := c.makeRequest("MKCOL", path, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MKCOL failed with status: %s", resp.Status)
	}
	
	return nil
}

// Put 上传文件
func (c *Client) Put(path string, data io.Reader) error {
	resp, err := c.makeRequest("PUT", path, data, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("PUT failed with status: %s", resp.Status)
	}
	
	return nil
}

// PutFile 上传文件内容
func (c *Client) PutFile(path string, content []byte) error {
	return c.Put(path, bytes.NewReader(content))
}

// Get 下载文件
func (c *Client) Get(path string) ([]byte, error) {
	resp, err := c.makeRequest("GET", path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET failed with status: %s", resp.Status)
	}
	
	return io.ReadAll(resp.Body)
}

// GetStream 获取文件流
func (c *Client) GetStream(path string) (io.ReadCloser, error) {
	resp, err := c.makeRequest("GET", path, nil, nil)
	if err != nil {
		return nil, err
	}
	
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET failed with status: %s", resp.Status)
	}
	
	return resp.Body, nil
}

// Delete 删除资源
func (c *Client) Delete(path string) error {
	resp, err := c.makeRequest("DELETE", path, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DELETE failed with status: %s", resp.Status)
	}
	
	return nil
}

// Move 移动或重命名资源
func (c *Client) Move(oldPath, newPath string, overwrite bool) error {
	destURL, err := url.Parse(c.BaseURL + newPath)
	if err != nil {
		return err
	}
	
	headers := map[string]string{
		"Destination": destURL.String(),
		"Overwrite":   "F",
	}
	
	if overwrite {
		headers["Overwrite"] = "T"
	}
	
	resp, err := c.makeRequest("MOVE", oldPath, nil, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("MOVE failed with status: %s", resp.Status)
	}
	
	return nil
}

// Copy 复制资源
func (c *Client) Copy(srcPath, destPath string, overwrite bool) error {
	destURL, err := url.Parse(c.BaseURL + destPath)
	if err != nil {
		return err
	}
	
	headers := map[string]string{
		"Destination": destURL.String(),
		"Overwrite":   "F",
	}
	
	if overwrite {
		headers["Overwrite"] = "T"
	}
	
	resp, err := c.makeRequest("COPY", srcPath, nil, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("COPY failed with status: %s", resp.Status)
	}
	
	return nil
}

// Lock 锁定资源
func (c *Client) Lock(path string, timeout time.Duration) error {
	headers := map[string]string{
		"Content-Type": "application/xml",
		"Timeout":      fmt.Sprintf("Second-%d", int(timeout.Seconds())),
	}
	
	body := `<?xml version="1.0"?>
<d:lockinfo xmlns:d="DAV:">
  <d:lockscope><d:exclusive/></d:lockscope>
  <d:locktype><d:write/></d:locktype>
</d:lockinfo>`
	
	resp, err := c.makeRequest("LOCK", path, strings.NewReader(body), headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("LOCK failed with status: %s", resp.Status)
	}
	
	return nil
}

// Unlock 解锁资源
func (c *Client) Unlock(path string, lockToken string) error {
	headers := map[string]string{
		"Lock-Token": fmt.Sprintf("<%s>", lockToken),
	}
	
	resp, err := c.makeRequest("UNLOCK", path, nil, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("UNLOCK failed with status: %s", resp.Status)
	}
	
	return nil
}