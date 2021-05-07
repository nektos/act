package assets

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/julienschmidt/httprouter"
)

type FileContainerResourceURL struct {
	FileContainerResourceURL string `json:"fileContainerResourceUrl"`
}

type NamedFileContainerResourceURL struct {
	Name                     string `json:"name"`
	FileContainerResourceURL string `json:"fileContainerResourceUrl"`
}

type NamedFileContainerResourceURLResponse struct {
	Count int                             `json:"count"`
	Value []NamedFileContainerResourceURL `json:"value"`
}

type ContainerItem struct {
	Path            string `json:"path"`
	ItemType        string `json:"itemType"`
	ContentLocation string `json:"contentLocation"`
}

type ContainerItemResponse struct {
	Value []ContainerItem `json:"value"`
}

type ResponseMessage struct {
	Message string `json:"message"`
}

func uploads(router *httprouter.Router, assetPath string) {
	router.POST("/_apis/pipelines/workflows/:runId/artifacts", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		runID := params.ByName("runId")

		json, err := json.Marshal(FileContainerResourceURL{
			FileContainerResourceURL: fmt.Sprintf("http://%s/upload/%s", req.Host, runID),
		})
		if err != nil {
			panic(err)
		}

		// get runId from path
		_, err = w.Write(json)
		if err != nil {
			panic(err)
		}
	})

	router.PUT("/upload/:runId", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		itemPath := req.URL.Query().Get("itemPath")
		runID := params.ByName("runId")

		filePath := fmt.Sprintf("%s/%s/%s", assetPath, runID, itemPath)

		err := os.MkdirAll(path.Dir(filePath), os.ModePerm)
		if err != nil {
			panic(err)
		}

		file, err := os.Create(filePath)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		_, err = io.Copy(file, req.Body)
		if err != nil {
			panic(err)
		}

		json, err := json.Marshal(ResponseMessage{
			Message: "success",
		})
		if err != nil {
			panic(err)
		}

		// get runId from path
		_, err = w.Write(json)
		if err != nil {
			panic(err)
		}
	})

	router.PATCH("/_apis/pipelines/workflows/:runId/artifacts", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		json, err := json.Marshal(ResponseMessage{
			Message: "success",
		})
		if err != nil {
			panic(err)
		}

		// get runId from path
		_, err = w.Write(json)
		if err != nil {
			panic(err)
		}
	})
}

func downloads(router *httprouter.Router, assetPath string) {
	router.GET("/_apis/pipelines/workflows/:runId/artifacts", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		runID := params.ByName("runId")
		dirPath := fmt.Sprintf("%s/%s", assetPath, runID)

		files, err := ioutil.ReadDir(dirPath)
		if err != nil {
			panic(err)
		}

		var list []NamedFileContainerResourceURL
		for _, file := range files {
			list = append(list, NamedFileContainerResourceURL{
				Name:                     file.Name(),
				FileContainerResourceURL: fmt.Sprintf("http://%s/download/%s", req.Host, runID),
			})
		}

		json, err := json.Marshal(NamedFileContainerResourceURLResponse{
			Count: len(list),
			Value: list,
		})
		if err != nil {
			panic(err)
		}

		// get runId from path
		_, err = w.Write(json)
		if err != nil {
			panic(err)
		}
	})

	router.GET("/download/:container", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		container := params.ByName("container")
		itemPath := req.URL.Query().Get("itemPath")
		dirPath := fmt.Sprintf("%s/%s/%s", assetPath, container, itemPath)

		var files []ContainerItem
		err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				rel, err := filepath.Rel(dirPath, path)
				if err != nil {
					panic(err)
				}

				files = append(files, ContainerItem{
					Path:            fmt.Sprintf("%s/%s", itemPath, rel),
					ItemType:        "file",
					ContentLocation: fmt.Sprintf("http://%s/asset/%s/%s/%s", req.Host, container, itemPath, rel),
				})
			}
			return nil
		})
		if err != nil {
			panic(err)
		}

		json, err := json.Marshal(ContainerItemResponse{
			Value: files,
		})
		if err != nil {
			panic(err)
		}

		// get runId from path
		_, err = w.Write(json)
		if err != nil {
			panic(err)
		}
	})

	router.GET("/asset/*path", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		path := params.ByName("path")
		dirPath := fmt.Sprintf("%s/%s", assetPath, path)

		file, err := os.Open(dirPath)
		if err != nil {
			panic(err)
		}

		_, err = io.Copy(w, file)
		if err != nil {
			panic(err)
		}
	})
}

func ServeAssets() {
	router := httprouter.New()

	// todo: how to name this env var?
	assetPath := os.Getenv("ASSET_PATH")
	if assetPath == "" {
		absPath, err := filepath.Abs("./.act")
		if err != nil {
			panic(err)
		}

		assetPath = absPath
	}

	uploads(router, assetPath)
	downloads(router, assetPath)

	// todo: how to name this env var?
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(http.ListenAndServe(fmt.Sprintf("localhost:%s", port), router))
}
