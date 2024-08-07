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
		t.Log("Error listing files:", err)
	} else {
		t.Log("Files:", files)
	}

	// Get file or directory information
	fileInfo, err := fs.Stat("/somefile")
	if err != nil {
		t.Log("Error getting file info:", err)
	} else {
		t.Log("FileInfo:", fileInfo)
	}

	// Create a new directory with parents
	err = fs.CreateDir("/newdir/parent")
	if err != nil {
		t.Log("Error creating directory:", err)
	}

	// Copy local file or directory to server
	err = fs.Copy("../.github", "/remotedir")
	if err != nil {
		t.Log("Error copying file:", err)
	}

	// Write logs
	logs := []string{"Log entry 1", "Log entry 2"}
	err = fs.WriteLog("/logfile.log", logs)
	if err != nil {
		t.Log("Error writing logs:", err)
	}

	// Add a download task
	taskId, err := fs.AddDownloadTask("/remotedir", "https://fs.luxianghua.top/ips.txt", "")
	if err != nil {
		t.Log("Error adding download task:", err)
	} else {
		t.Log("Download task added, ID:", taskId)
	}

	// Get download task status
	taskInfo, err := fs.GetDownloadTaskStatus(taskId)
	if err != nil {
		t.Log("Error getting task status:", err)
	} else {
		t.Log("Task status:", taskInfo)
	}

	// List download tasks
	tasks, err := fs.ListDownloadTasks(nil, "")
	if err != nil {
		t.Log("Error listing tasks:", err)
	} else {
		t.Log("Tasks:", tasks)
	}
}
