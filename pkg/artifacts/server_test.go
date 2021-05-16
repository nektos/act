package artifacts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

func TestNewArtifactUploadPrepare(t *testing.T) {
	assert := assert.New(t)

	router := httprouter.New()
	uploads(router, "/tmp")

	req, _ := http.NewRequest("POST", "http://localhost/_apis/pipelines/workflows/1/artifacts", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		assert.Fail("Wrong status")
	}

	response := FileContainerResourceURL{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		panic(err)
	}

	assert.Equal("http://localhost/upload/1", response.FileContainerResourceURL)
}

func TestFinalizeArtifactUpload(t *testing.T) {
	assert := assert.New(t)

	router := httprouter.New()
	uploads(router, "/tmp")

	req, _ := http.NewRequest("PATCH", "http://localhost/_apis/pipelines/workflows/1/artifacts", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		assert.Fail("Wrong status")
	}

	response := ResponseMessage{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		panic(err)
	}

	assert.Equal("success", response.Message)
}
