package client_test

import (
	"fmt"
	"log"
	"time"

	"github.com/breezechen/go_file_server/webdav/client"
)

func ExampleClient_basic() {
	// 创建 WebDAV 客户端
	c := client.NewClient("http://localhost:8080/dav")
	
	// 设置认证
	c.SetAuth("username", "password")
	
	// 列出目录内容
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

func ExampleClient_fileOperations() {
	c := client.NewClient("http://localhost:8080/dav")
	
	// 创建目录
	if err := c.Mkcol("/documents"); err != nil {
		log.Fatal(err)
	}
	
	// 上传文件
	content := []byte("Hello, WebDAV!")
	if err := c.PutFile("/documents/hello.txt", content); err != nil {
		log.Fatal(err)
	}
	
	// 下载文件
	data, err := c.Get("/documents/hello.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Downloaded: %s\n", string(data))
	
	// 移动文件
	if err := c.Move("/documents/hello.txt", "/documents/greeting.txt", false); err != nil {
		log.Fatal(err)
	}
	
	// 复制文件
	if err := c.Copy("/documents/greeting.txt", "/documents/backup.txt", true); err != nil {
		log.Fatal(err)
	}
	
	// 删除文件
	if err := c.Delete("/documents/backup.txt"); err != nil {
		log.Fatal(err)
	}
}

func ExampleClient_advancedFeatures() {
	c := client.NewClient("http://localhost:8080/dav")
	c.SetAuth("admin", "password")
	c.SetTimeout(30 * time.Second)
	
	// 获取文件信息
	info, err := c.Stat("/important.doc")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("File: %s\nSize: %d bytes\nModified: %v\n", 
		info.Name, info.Size, info.ModTime)
	
	// 递归列出所有文件
	allFiles, err := c.Propfind("/", -1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Total files and directories: %d\n", len(allFiles))
	
	// 锁定文件
	if err := c.Lock("/important.doc", 30*time.Minute); err != nil {
		log.Fatal(err)
	}
	
	// 解锁文件（需要提供锁令牌）
	// lockToken := "opaquelocktoken:xxxx"
	// if err := c.Unlock("/important.doc", lockToken); err != nil {
	//     log.Fatal(err)
	// }
}