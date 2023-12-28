package main

import (
	_ "embed"
	"fmt"
	"log"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/flytam/filenamify"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/melbahja/got"
	"github.com/urfave/cli/v2"
)

var (
	//go:embed favicon.svg
	favicon []byte

	//go:embed index.html
	indexHtml string

	manager = NewDownloadManager()

	rootDir string
)

type PostRequest struct {
	Url    string `json:"url"`
	Method string `json:"method"`
	Name   string `json:"name"`
}

type DownloadResponse struct {
	TaskId   string `json:"taskId"`
	Filename string `json:"filename"`
}

type CreateDirResponse struct {
	Name string `json:"name"`
	Url  string `json:"url"`
}

type ListTaskRequestItem struct {
	TaskIds []string `json:"taskIds"`
	Status  string   `json:"status"`
}

type ListTaskRequest struct {
	OrItems []ListTaskRequestItem `json:"or"`
}

type ListTaskResponse struct {
	Tasks []*DownloadTaskInfo `json:"tasks"`
}

type DownloadStatus struct {
	Status     string `json:"status"`
	Totalsize  uint64 `json:"totalsize"`
	Downloaded uint64 `json:"downloaded"`
	Speed      string `json:"speed"`
	ErrMsg     string `json:"errMsg"`
}

type DownloadTaskInfo struct {
	TaskId    string          `json:"taskId"`
	Url       string          `json:"url"`
	Filename  string          `json:"filename"`
	Filepath  string          `json:"filepath"`
	Status    *DownloadStatus `json:"status"`
	StartedAt *time.Time      `json:"startedAt"`
	EndAt     *time.Time      `json:"endAt"`
}

type DownloadManager struct {
	Tasks             map[string]*DownloadTaskInfo
	taskToDownlaodMap map[string]*got.Download
	downloadToTaskMap map[*got.Download]string
}

func NewDownloadManager() *DownloadManager {
	return &DownloadManager{
		Tasks:             make(map[string]*DownloadTaskInfo),
		taskToDownlaodMap: make(map[string]*got.Download),
		downloadToTaskMap: make(map[*got.Download]string),
	}
}

func (dm *DownloadManager) GetTaskStatus(taskId string) *DownloadTaskInfo {
	return dm.Tasks[taskId]
}

func (dm *DownloadManager) List(taskIds []string, status string) []*DownloadTaskInfo {
	tasks := make([]*DownloadTaskInfo, 0, len(dm.Tasks))
	if len(taskIds) == 0 {
		taskIds = make([]string, 0, len(dm.Tasks))
		for taskId := range dm.Tasks {
			taskIds = append(taskIds, taskId)
		}
	}

	for _, taskId := range taskIds {
		task := dm.Tasks[taskId]
		if task != nil && (status == "" || task.Status.Status == status) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (dm *DownloadManager) ProgressFunc(d *got.Download) {
	taskId := dm.downloadToTaskMap[d]
	downloaded := d.Size()

	if downloaded > dm.Tasks[taskId].Status.Downloaded {
		dm.Tasks[taskId].Status.Status = "downloading"
	}
	dm.Tasks[taskId].Status.Downloaded = downloaded
	dm.Tasks[taskId].Status.Totalsize = d.TotalSize()
	dm.Tasks[taskId].Status.Speed = humanReadableSize(int64(d.AvgSpeed())) + "/s"
}

func (dm *DownloadManager) CompleteTask(taskId string) {
	download := dm.taskToDownlaodMap[taskId]
	download.StopProgress = true
	dm.Tasks[taskId].Status.Status = "finished"
	var timeNow = time.Now()
	dm.Tasks[taskId].EndAt = &timeNow
}

func (dm *DownloadManager) FailTask(taskId string, errMsg string) {
	download := dm.taskToDownlaodMap[taskId]
	download.StopProgress = true
	dm.Tasks[taskId].Status.Status = "failed"
	dm.Tasks[taskId].Status.ErrMsg = errMsg
	var timeNow = time.Now()
	dm.Tasks[taskId].EndAt = &timeNow
}

func (dm *DownloadManager) AddTask(url, dir string) (string, error) {
	download := &got.Download{
		URL: url,
		Dir: dir,
	}
	if err := download.Init(); err != nil {
		return "", err
	}

	taskId := uuid.New().String()
	path := download.Path()
	relPath, err := filepath.Rel(rootDir, path)
	if err == nil {
		path = relPath
	}
	timeNow := time.Now()
	dm.Tasks[taskId] = &DownloadTaskInfo{
		TaskId:   taskId,
		Url:      url,
		Filepath: path,
		Filename: filepath.Base(path),
		Status: &DownloadStatus{
			Status: "pending",
		},
		StartedAt: &timeNow,
	}

	dm.downloadToTaskMap[download] = taskId
	dm.taskToDownlaodMap[taskId] = download

	go func() {
		if err := download.Start(); err != nil {
			dm.FailTask(taskId, err.Error())
		} else {
			dm.CompleteTask(taskId)
		}
	}()

	go func() {
		download.RunProgress(dm.ProgressFunc)
	}()

	return taskId, nil
}

func (dm *DownloadManager) ClearEndedTasks(days int) {
	for taskId, task := range dm.Tasks {
		if task.EndAt != nil && time.Since(*task.EndAt).Hours() > float64(days*24) {
			delete(dm.Tasks, taskId)
		}
	}
}

func humanReadableSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(size)/1024/1024)
	}
	return fmt.Sprintf("%.1fGB", float64(size)/1024/1024/1024)
}

func genIndexHtml(rootDir string, uri string) string {
	items, err := os.ReadDir(path.Join(rootDir, uri))
	if err != nil {
		log.Fatal(err)
	}

	html := indexHtml
	html += fmt.Sprintf("<script>start('%s');</script>", uri)
	if uri != "/" {
		html += "<script>onHasParentDirectory();</script>"
	}

	for _, item := range items {
		if item.IsDir() {
			info, _ := item.Info()
			html += fmt.Sprintf("<script>addRow('%s', '%s', 1, 0, '', %d, '%s');</script>\n",
				strings.ReplaceAll(item.Name(), "'", "\\'"),
				url.PathEscape(item.Name()),
				info.ModTime().Unix(),
				info.ModTime().Format("2006-01-02 15:04:05"),
			)
		}
	}

	for _, item := range items {
		if !item.IsDir() {
			info, _ := item.Info()
			html += fmt.Sprintf("<script>addRow('%s', '%s', 0, %d, '%s', %d, '%s');</script>\n",
				strings.ReplaceAll(item.Name(), "'", "\\'"),
				url.PathEscape(item.Name()),
				info.Size(),
				humanReadableSize(info.Size()),
				info.ModTime().Unix(),
				info.ModTime().Format("2006-01-02 15:04:05"),
			)
		}
	}
	return html
}

func handleListTask(c *gin.Context) {
	req := &ListTaskRequest{}
	err := c.BindJSON(req)
	if err != nil {
		c.String(400, "400 bad request")
		return
	}

	ret := make([]*DownloadTaskInfo, 0)
	taskIdMap := make(map[string]bool)
	for _, item := range req.OrItems {
		tasks := manager.List(item.TaskIds, item.Status)
		for _, task := range tasks {
			if _, ok := taskIdMap[task.TaskId]; !ok {
				ret = append(ret, task)
				taskIdMap[task.TaskId] = true
			}
		}
	}

	c.JSON(200, ListTaskResponse{
		Tasks: ret,
	})
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func isSubDir(parent string, child string) bool {
	parent, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	child, err = filepath.Abs(child)
	if err != nil {
		return false
	}
	return child == parent || strings.HasPrefix(child, parent+string(os.PathSeparator))
}

func start_server(c *cli.Context) error {
	port := c.String("port")
	dir := c.String("dir")
	rootDir = dir
	r := gin.Default()
	mime.AddExtensionType(".apk", "application/vnd.android.package-archive")
	mime.AddExtensionType(".ipa", "application/vnd.iphone")
	mime.AddExtensionType(".txt", "text/plain")

	r.GET("/*uri", func(c *gin.Context) {
		uri := c.Param("uri")
		if uri == "/favicon.ico" {
			c.Data(200, "image/svg+xml", favicon)
			return
		}

		filePath := path.Join(dir, uri)
		stat, err := os.Stat(filePath)
		if err != nil {
			c.String(404, "404 not found")
			return
		}
		if stat.IsDir() {
			c.Data(200, "text/html", []byte(genIndexHtml(dir, uri)))
			return
		}

		c.File(path.Join(dir, uri))
	})
	r.POST("/*uri", func(c *gin.Context) {
		uri := c.Param("uri")

		if uri == "/:tasks" {
			handleListTask(c)
			return
		}

		filePath := path.Join(dir, uri)
		stat, err := os.Stat(filePath)
		if err != nil {
			c.String(404, "404 not found")
			return
		}
		if !stat.IsDir() {
			c.String(400, "400 bad request")
			return
		}
		form, err := c.MultipartForm()
		if err == nil {
			files := form.File["files"]
			for _, file := range files {
				c.SaveUploadedFile(file, path.Join(filePath, file.Filename))
			}
			c.String(200, "200 ok")
			return
		}

		req := PostRequest{}
		err = c.BindJSON(&req)
		if err == nil {
			if req.Method == "download" {
				taskId, err := manager.AddTask(req.Url, filePath)
				if err != nil {
					c.String(500, err.Error())
				} else {
					c.JSON(200, DownloadResponse{
						TaskId: taskId,
					})
				}
				return
			} else if req.Method == "createDir" {
				safeName, err := filenamify.Filenamify(req.Name, filenamify.Options{})
				if err != nil {
					c.String(500, err.Error())
				}

				createdDirPath := path.Join(filePath, safeName)
				if !isSubDir(dir, createdDirPath) {
					c.String(400, "400 bad request")
					return
				}

				err = os.Mkdir(createdDirPath, 0755)
				if err != nil {
					c.String(500, err.Error())
				} else {
					c.JSON(200, CreateDirResponse{
						Name: safeName,
						Url:  path.Join(uri, url.PathEscape(safeName)),
					})
				}
				return
			} else if req.Method == "deleteFile" {
				deletedFilePath := path.Join(filePath, req.Name)
				if ok, _ := exists(deletedFilePath); !ok {
					c.String(404, "file not found")
					return
				}

				if !isSubDir(dir, deletedFilePath) {
					c.String(400, "400 bad request")
					return
				}

				err := os.RemoveAll(deletedFilePath)
				if err != nil {
					c.String(500, err.Error())
				} else {
					c.String(200, "200 ok")
				}
				return
			}
		}

		c.String(400, "400 bad request")
	})
	r.Run(":" + port)
	return nil
}

func main() {
	app := &cli.App{
		Name:  "fileserver",
		Usage: "fileserver",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   "9008",
				Usage:   "http listen port",
			},
			&cli.StringFlag{
				Name:    "dir",
				Aliases: []string{"d"},
				Value:   ".",
				Usage:   "root dir",
			},
		},
		Action: start_server,
	}
	app.Run(os.Args)
}
