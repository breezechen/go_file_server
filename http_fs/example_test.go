package http_fs_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/breezechen/go_file_server/http_fs"
)

func ExampleHttpFs_basic() {
	// 创建基础客户端
	fs := http_fs.NewHttpFs("http://localhost:9008")
	
	// 列出文件
	files, err := fs.ListFiles("/")
	if err != nil {
		log.Fatal(err)
	}
	
	for _, file := range files {
		fmt.Printf("%s - %d bytes\n", file.Name, file.Size)
	}
}

func ExampleHttpFs_withOptions() {
	// 使用选项创建客户端
	fs := http_fs.NewHttpFsWithOptions("http://localhost:9008",
		http_fs.WithTimeout(30*time.Second),
		http_fs.WithAuth("username", "password"),
		http_fs.WithHeaders(map[string]string{
			"X-Custom-Header": "value",
		}),
	)
	
	// 检查文件是否存在
	exists, err := fs.Exists("/path/to/file")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("File exists: %v\n", exists)
}

func ExampleHttpFs_batchOperations() {
	fs := http_fs.NewHttpFs("http://localhost:9008")
	
	// 批量操作
	ctx := context.Background()
	operations := []http_fs.BatchOperation{
		{Type: "upload", Source: "/local/file1.txt", Dest: "/remote/file1.txt"},
		{Type: "upload", Source: "/local/file2.txt", Dest: "/remote/file2.txt"},
		{Type: "download", Source: "/remote/file3.txt", Dest: "/local/file3.txt"},
		{Type: "delete", Source: "/remote/old-file.txt"},
	}
	
	errs := fs.BatchExecute(ctx, operations)
	for i, err := range errs {
		if err != nil {
			fmt.Printf("Operation %d failed: %v\n", i, err)
		}
	}
}

func ExampleHttpFs_walk() {
	fs := http_fs.NewHttpFs("http://localhost:9008")
	
	// 遍历远程目录
	err := fs.Walk("/", func(path string, info *http_fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir {
			fmt.Printf("Directory: %s\n", path)
		} else {
			fmt.Printf("File: %s (%d bytes)\n", path, info.Size)
		}
		return nil
	})
	
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleHttpFs_advancedFileOperations() {
	fs := http_fs.NewHttpFs("http://localhost:9008")
	
	// 获取文件内容
	content, err := fs.GetFileContent("/path/to/file.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("File content: %s\n", content)
	
	// 从内存上传
	data := []byte("file content from memory")
	err = fs.CreateFileFromBytes("/remote/memory-file.txt", data)
	if err != nil {
		log.Fatal(err)
	}
	
	// 递归列出所有文件
	allFiles, err := fs.ListFilesRecursive("/")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Total files: %d\n", len(allFiles))
	
	// 创建多级目录
	err = fs.CreateDirAll("/deep/nested/directory/structure")
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleWebDAVClient() {
	// WebDAV 客户端示例 - 现在使用独立的 webdav/client 包
	// import "github.com/breezechen/go_file_server/webdav/client"
	// 
	// client := client.NewClient("http://localhost:9008/$.dav$/")
	// client.SetAuth("username", "password")
	// 
	// // 列出目录
	// files, err := client.List("/")
	// 
	// // 上传文件
	// err = client.PutFile("/file.txt", []byte("content"))
	// 
	// // 下载文件
	// content, err := client.Get("/file.txt")
}