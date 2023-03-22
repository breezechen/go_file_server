package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/urfave/cli/v2"
)

var (
	//go:embed favicon.svg
	favicon []byte

	//go:embed index.html
	indexHtml string
)

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
				item.Name(),
				item.Name(),
				info.ModTime().Unix(),
				info.ModTime().Format("2006-01-02 15:04:05"),
			)
		}
	}

	for _, item := range items {
		if !item.IsDir() {
			info, _ := item.Info()
			html += fmt.Sprintf("<script>addRow('%s', '%s', 0, %d, '%s', %d, '%s');</script>\n",
				item.Name(),
				item.Name(),
				info.Size(),
				humanReadableSize(info.Size()),
				info.ModTime().Unix(),
				info.ModTime().Format("2006-01-02 15:04:05"),
			)
		}
	}
	return html
}

func start_server(c *cli.Context) error {
	port := c.String("port")
	dir := c.String("dir")
	r := gin.Default()

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
		form, _ := c.MultipartForm()
		files := form.File["files"]
		for _, file := range files {
			c.SaveUploadedFile(file, path.Join(filePath, file.Filename))
		}
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
