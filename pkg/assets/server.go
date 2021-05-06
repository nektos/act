package assets

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/julienschmidt/httprouter"
)

type FileContainerResourceURL struct {
	FileContainerResourceURL string `json:"fileContainerResourceUrl"`
}

type ResponseMessage struct {
	Message string `json:"message"`
}

func uploads(router *httprouter.Router) {
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

		filePath := fmt.Sprintf("act-assets/%s/%s", runID, itemPath)

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

func AssetServer() {
	router := httprouter.New()

	uploads(router)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(http.ListenAndServe(fmt.Sprintf("localhost:%s", port), router))
}
