package http_fs

import (
	"testing"
)

func TestHttpFs(t *testing.T) {
	// Example usage
	fs := NewHttpFs("http://localhost:9008")

	// List files in the root directory
	files, err := fs.ListFiles("/")
	if err != nil {
		t.Fatalf("Error listing files: %v", err)
	} else {
		t.Log("Files:", files)
	}

	// Get file or directory information
	fileInfo, err := fs.Stat("/main.go")
	if err != nil {
		t.Fatalf("Error getting file info: %v", err)
	} else {
		t.Log("FileInfo:", fileInfo)
	}

	// Create a new directory with parents
	err = fs.CreateDir("/newdir/parent")
	if err != nil {
		t.Fatalf("Error creating directory: %v", err)
	}

	// Copy local file or directory to server
	err = fs.CopyFrom("../.github", "/remotedir")
	if err != nil {
		t.Fatalf("Error copying from local: %v", err)
	}

	// Write logs
	logs := []string{"Log entry 1", "Log entry 2"}
	err = fs.WriteLog("/logfile.log", logs)
	if err != nil {
		t.Fatalf("Error writing logs: %v", err)
	}

	// Add a download task
	taskId, err := fs.AddDownloadTask("/remotedir", "https://fs.luxianghua.top/ips.txt", "")
	if err != nil {
		t.Fatal("Error adding download task:", err)
	} else {
		t.Log("Download task added, ID:", taskId)
	}

	// Get download task status
	taskInfo, err := fs.GetDownloadTaskStatus(taskId)
	if err != nil {
		t.Fatal("Error getting task status:", err)
	} else {
		t.Log("Task status:", taskInfo)
	}

	// List download tasks
	tasks, err := fs.ListDownloadTasks(nil, "")
	if err != nil {
		t.Fatal("Error listing tasks:", err)
	} else {
		t.Log("Tasks:", tasks)
	}
}
