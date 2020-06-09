package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Error messages
var (
	ErrSomethingWentWrong = errors.New("something went wrong")
)

// Payload ...
type Payload struct {
	Type string `json:"type" binding:"required"`
	Code string `json:"code" binding:"required"`
}

var languages = map[string]map[string]string{
	"python": {
		"image":    "python-0.1",
		"filename": "main.py",
	},
	"node": {
		"image":    "node-0.1",
		"filename": "main.js",
	},
	"c": {
		"image":    "c-0.1",
		"filename": "main.c",
	},
	"cpp": {
		"image":    "cpp-0.1",
		"filename": "main.cpp",
	},
	"go": {
		"image":    "go-0.1",
		"filename": "main.go",
	},
}

func main() {
	router := gin.Default()
	router.Use(cors.Default())
	router.POST("/execute", func(c *gin.Context) {
		// Parse request payload to Payload struct
		var payload Payload
		if err := c.ShouldBindJSON(&payload); err != nil {
			log.Println("ShouldBindJSON:", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": ErrSomethingWentWrong.Error(),
			})
			return
		}

		langauge, ok := languages[payload.Type]
		if !ok {
			log.Println("languages[payload.Type]:", "Could not find payload.Type in languages")
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": ErrSomethingWentWrong.Error(),
			})
			return
		}

		// Create a temp dir
		content := []byte(payload.Code)
		dir, err := ioutil.TempDir("./", "source")
		if err != nil {
			log.Println("TempDir:", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": ErrSomethingWentWrong.Error(),
			})
			return
		}
		defer os.RemoveAll(dir)

		tmpfn := filepath.Join(dir, langauge["filename"])
		if err := ioutil.WriteFile(tmpfn, content, 0777); err != nil {
			log.Println("WriteFile:", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": ErrSomethingWentWrong.Error(),
			})
			return
		}

		// Get dir absolute path
		fullpath, err := filepath.Abs(dir)
		if err != nil {
			log.Println("Abs:", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": ErrSomethingWentWrong.Error(),
			})
			return
		}

		// Using request context
		ctx := c.Request.Context()

		// Init docker client
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			log.Println("NewClientWithOpts:", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": ErrSomethingWentWrong.Error(),
			})
			return
		}
		defer cli.Close()

		containerConfig := container.Config{
			Image:        langauge["image"],
			Env:          []string{fmt.Sprintf("FILE_NAME=%s", langauge["filename"])},
			Tty:          false,
			AttachStdout: true,
			AttachStderr: true,
		}

		hostConfig := container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: fullpath,
					Target: "/app",
				},
			},
			Resources: container.Resources{
				Memory: 1e+9,
			},
		}

		resp, err := cli.ContainerCreate(ctx, &containerConfig, &hostConfig, nil, nil, "")
		if err != nil {
			log.Println("ContainerCreate: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": ErrSomethingWentWrong.Error(),
			})
			return
		}

		// Start docker container
		if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
			log.Println("ContainerStart: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": ErrSomethingWentWrong.Error(),
			})
			return
		}

		// Wait for container to start
		statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
		select {
		case err := <-errCh:
			if err != nil {
				log.Println("ContainerWait: ", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  http.StatusInternalServerError,
					"message": ErrSomethingWentWrong.Error(),
				})
				return
			}
		case status := <-statusCh:
			if status.StatusCode == 137 {
				// Prune container ignore error
				cli.ContainersPrune(ctx, filters.Args{})

				log.Println("StatusCode:", "Out of Memory")
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  http.StatusInternalServerError,
					"message": ErrSomethingWentWrong.Error(),
				})
				return
			}

			// Get docker container logs
			out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
			if err != nil {
				log.Println("ContainerLogs:", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  http.StatusInternalServerError,
					"message": ErrSomethingWentWrong.Error(),
				})
				return
			}
			defer out.Close()

			// convert io.ReadCloser to []byte
			logs, err := ioutil.ReadAll(out)
			if err != nil {
				log.Println("ReadAll:", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  http.StatusInternalServerError,
					"message": ErrSomethingWentWrong.Error(),
				})
				return
			}

			message := string(logs)
			// Remove container ignore error
			cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})

			c.JSON(http.StatusOK, gin.H{
				"status":   http.StatusOK,
				"response": message,
			})
			return
		case <-time.After(10 * time.Second):
			// Remove container ignore error
			cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})

			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": ErrSomethingWentWrong.Error(),
			})
			return
		}
	})

	router.Run(":8080")

}
