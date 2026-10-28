// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package artifacts

// GitHub Actions Artifacts V4 API Simple Description
//
// 1. Upload artifact
// 1.1. CreateArtifact
// Post: /twirp/github.actions.results.api.v1.ArtifactService/CreateArtifact
// Request:
// {
//     "workflow_run_backend_id": "21",
//     "workflow_job_run_backend_id": "49",
//     "name": "test",
//     "version": 4
// }
// Response:
// {
//     "ok": true,
//     "signedUploadUrl": "http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact?sig=mO7y35r4GyjN7fwg0DTv3-Fv1NDXD84KLEgLpoPOtDI=&expires=2024-01-23+21%3A48%3A37.20833956+%2B0100+CET&artifactName=test&taskID=75"
// }
// 1.2. Upload Zip Content to Blobstorage (unauthenticated request)
// PUT: http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact?sig=mO7y35r4GyjN7fwg0DTv3-Fv1NDXD84KLEgLpoPOtDI=&expires=2024-01-23+21%3A48%3A37.20833956+%2B0100+CET&artifactName=test&taskID=75&comp=block
// 1.3. Continue Upload Zip Content to Blobstorage (unauthenticated request), repeat until everything is uploaded
// PUT: http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact?sig=mO7y35r4GyjN7fwg0DTv3-Fv1NDXD84KLEgLpoPOtDI=&expires=2024-01-23+21%3A48%3A37.20833956+%2B0100+CET&artifactName=test&taskID=75&comp=appendBlock
// 1.4. Unknown xml payload to Blobstorage (unauthenticated request), ignored for now
// PUT: http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact?sig=mO7y35r4GyjN7fwg0DTv3-Fv1NDXD84KLEgLpoPOtDI=&expires=2024-01-23+21%3A48%3A37.20833956+%2B0100+CET&artifactName=test&taskID=75&comp=blockList
// 1.5. FinalizeArtifact
// Post: /twirp/github.actions.results.api.v1.ArtifactService/FinalizeArtifact
// Request
// {
//     "workflow_run_backend_id": "21",
//     "workflow_job_run_backend_id": "49",
//     "name": "test",
//     "size": "2097",
//     "hash": "sha256:b6325614d5649338b87215d9536b3c0477729b8638994c74cdefacb020a2cad4"
// }
// Response
// {
//     "ok": true,
//     "artifactId": "4"
// }
// 2. Download artifact
// 2.1. ListArtifacts and optionally filter by artifact exact name or id
// Post: /twirp/github.actions.results.api.v1.ArtifactService/ListArtifacts
// Request
// {
//     "workflow_run_backend_id": "21",
//     "workflow_job_run_backend_id": "49",
//     "name_filter": "test"
// }
// Response
// {
//     "artifacts": [
//         {
//             "workflowRunBackendId": "21",
//             "workflowJobRunBackendId": "49",
//             "databaseId": "4",
//             "name": "test",
//             "size": "2093",
//             "createdAt": "2024-01-23T00:13:28Z"
//         }
//     ]
// }
// 2.2. GetSignedArtifactURL get the URL to download the artifact zip file of a specific artifact
// Post: /twirp/github.actions.results.api.v1.ArtifactService/GetSignedArtifactURL
// Request
// {
//     "workflow_run_backend_id": "21",
//     "workflow_job_run_backend_id": "49",
//     "name": "test"
// }
// Response
// {
//     "signedUrl": "http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/DownloadArtifact?sig=wHzFOwpF-6220-5CA0CIRmAX9VbiTC2Mji89UOqo1E8=&expires=2024-01-23+21%3A51%3A56.872846295+%2B0100+CET&artifactName=test&taskID=76"
// }
// 2.3. Download Zip from Blobstorage (unauthenticated request)
// GET: http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/DownloadArtifact?sig=wHzFOwpF-6220-5CA0CIRmAX9VbiTC2Mji89UOqo1E8=&expires=2024-01-23+21%3A51%3A56.872846295+%2B0100+CET&artifactName=test&taskID=76

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	ArtifactV4RouteBase       = "/twirp/github.actions.results.api.v1.ArtifactService"
	ArtifactV4ContentEncoding = "application/zip"
	artifactV4MetadataFile    = ".metadata.json"
)

type artifactV4Metadata struct {
	ContentType string `json:"contentType"`
}

type artifactV4Routes struct {
	prefix  string
	fs      WriteFS
	rfs     fs.FS
	AppURL  string
	baseDir string
}

type ArtifactContext struct {
	Req  *http.Request
	Resp http.ResponseWriter
}

func artifactNameToID(s string) int64 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int64(h.Sum32())
}

func (c ArtifactContext) Error(status int, _ ...interface{}) {
	c.Resp.WriteHeader(status)
}

func (c ArtifactContext) JSON(status int, _ ...interface{}) {
	c.Resp.WriteHeader(status)
}

func validateRunIDV4(ctx *ArtifactContext, rawRunID string) (interface{}, int64, bool) {
	runID, err := strconv.ParseInt(rawRunID, 10, 64)
	if err != nil /* || task.Job.RunID != runID*/ {
		log.Error("Error runID not match")
		ctx.Error(http.StatusBadRequest, "run-id does not match")
		return nil, 0, false
	}
	return nil, runID, true
}

func RoutesV4(router *httprouter.Router, baseDir string, fsys WriteFS, rfs fs.FS) {
	route := &artifactV4Routes{
		fs:      fsys,
		rfs:     rfs,
		baseDir: baseDir,
		prefix:  ArtifactV4RouteBase,
	}
	router.POST(path.Join(ArtifactV4RouteBase, "CreateArtifact"), func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		route.AppURL = r.Host
		route.createArtifact(&ArtifactContext{
			Req:  r,
			Resp: w,
		})
	})
	router.POST(path.Join(ArtifactV4RouteBase, "FinalizeArtifact"), func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		route.finalizeArtifact(&ArtifactContext{
			Req:  r,
			Resp: w,
		})
	})
	router.POST(path.Join(ArtifactV4RouteBase, "ListArtifacts"), func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		route.listArtifacts(&ArtifactContext{
			Req:  r,
			Resp: w,
		})
	})
	router.POST(path.Join(ArtifactV4RouteBase, "GetSignedArtifactURL"), func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		route.AppURL = r.Host
		route.getSignedArtifactURL(&ArtifactContext{
			Req:  r,
			Resp: w,
		})
	})
	router.POST(path.Join(ArtifactV4RouteBase, "DeleteArtifact"), func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		route.AppURL = r.Host
		route.deleteArtifact(&ArtifactContext{
			Req:  r,
			Resp: w,
		})
	})
	router.PUT(path.Join(ArtifactV4RouteBase, "UploadArtifact"), func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		route.uploadArtifact(&ArtifactContext{
			Req:  r,
			Resp: w,
		})
	})
	router.GET(path.Join(ArtifactV4RouteBase, "DownloadArtifact"), func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		route.downloadArtifact(&ArtifactContext{
			Req:  r,
			Resp: w,
		})
	})
}

func (r artifactV4Routes) buildSignature(endp, expires, artifactName string, taskID int64) []byte {
	mac := hmac.New(sha256.New, []byte{0xba, 0xdb, 0xee, 0xf0})
	mac.Write([]byte(endp))
	mac.Write([]byte(expires))
	mac.Write([]byte(artifactName))
	mac.Write([]byte(fmt.Sprint(taskID)))
	return mac.Sum(nil)
}

func (r artifactV4Routes) buildArtifactURL(endp, artifactName string, taskID int64) string {
	expires := time.Now().Add(60 * time.Minute).Format("2006-01-02 15:04:05.999999999 -0700 MST")
	uploadURL := "http://" + strings.TrimSuffix(r.AppURL, "/") + strings.TrimSuffix(r.prefix, "/") +
		"/" + endp + "?sig=" + base64.URLEncoding.EncodeToString(r.buildSignature(endp, expires, artifactName, taskID)) + "&expires=" + url.QueryEscape(expires) + "&artifactName=" + url.QueryEscape(artifactName) + "&taskID=" + fmt.Sprint(taskID)
	return uploadURL
}

func (r artifactV4Routes) verifySignature(ctx *ArtifactContext, endp string) (int64, string, bool) {
	rawTaskID := ctx.Req.URL.Query().Get("taskID")
	sig := ctx.Req.URL.Query().Get("sig")
	expires := ctx.Req.URL.Query().Get("expires")
	artifactName := ctx.Req.URL.Query().Get("artifactName")
	// Azure blob clients may strip base64 padding when adding block parameters.
	dsig, _ := base64.RawURLEncoding.DecodeString(strings.TrimRight(sig, "="))
	taskID, _ := strconv.ParseInt(rawTaskID, 10, 64)

	expecedsig := r.buildSignature(endp, expires, artifactName, taskID)
	if !hmac.Equal(dsig, expecedsig) {
		log.Error("Error unauthorized")
		ctx.Error(http.StatusUnauthorized, "Error unauthorized")
		return -1, "", false
	}
	t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", expires)
	if err != nil || t.Before(time.Now()) {
		log.Error("Error link expired")
		ctx.Error(http.StatusUnauthorized, "Error link expired")
		return -1, "", false
	}
	return taskID, artifactName, true
}

func (r *artifactV4Routes) parseProtbufBody(ctx *ArtifactContext, req protoreflect.ProtoMessage) ([]byte, bool) {
	body, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		log.Errorf("Error decode request body: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error decode request body")
		return nil, false
	}
	// Artifact clients may add optional fields before act updates its schema.
	err = protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(body, req)
	if err != nil {
		log.Errorf("Error decode request body: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error decode request body")
		return nil, false
	}
	return body, true
}

func artifactV4ContentType(body []byte) string {
	var request struct {
		MimeType string `json:"mime_type"`
	}
	if err := json.Unmarshal(body, &request); err != nil || request.MimeType == "" {
		return ArtifactV4ContentEncoding
	}
	if _, _, err := mime.ParseMediaType(request.MimeType); err != nil {
		return ArtifactV4ContentEncoding
	}
	return request.MimeType
}

func (r artifactV4Routes) artifactV4Path(runID int64, artifactName string) string {
	safeRunPath := safeResolve(r.baseDir, fmt.Sprint(runID))
	safePath := safeResolve(safeRunPath, artifactName)
	return safeResolve(safePath, artifactName+".zip")
}

func (r artifactV4Routes) artifactV4MetadataPath(runID int64, artifactName string) string {
	safeRunPath := safeResolve(r.baseDir, fmt.Sprint(runID))
	safePath := safeResolve(safeRunPath, artifactName)
	return safeResolve(safePath, artifactV4MetadataFile)
}

func (r artifactV4Routes) writeArtifactV4Metadata(runID int64, artifactName string, metadata artifactV4Metadata) error {
	file, err := r.fs.OpenWritable(r.artifactV4MetadataPath(runID, artifactName))
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(metadata)
}

func (r artifactV4Routes) readArtifactV4Metadata(runID int64, artifactName string) artifactV4Metadata {
	metadata := artifactV4Metadata{ContentType: ArtifactV4ContentEncoding}
	file, err := r.rfs.Open(r.artifactV4MetadataPath(runID, artifactName))
	if err != nil {
		return metadata
	}
	defer file.Close()

	var stored artifactV4Metadata
	if err := json.NewDecoder(file).Decode(&stored); err != nil {
		return metadata
	}
	if _, _, err := mime.ParseMediaType(stored.ContentType); err != nil {
		return metadata
	}
	return stored
}

func (r *artifactV4Routes) sendProtbufBody(ctx *ArtifactContext, req protoreflect.ProtoMessage) {
	resp, err := protojson.Marshal(req)
	if err != nil {
		log.Errorf("Error encode response body: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error encode response body")
		return
	}
	ctx.Resp.Header().Set("Content-Type", "application/json;charset=utf-8")
	ctx.Resp.WriteHeader(http.StatusOK)
	_, _ = ctx.Resp.Write(resp)
}

func (r *artifactV4Routes) createArtifact(ctx *ArtifactContext) {
	var req CreateArtifactRequest

	body, ok := r.parseProtbufBody(ctx, &req)
	if !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	artifactName := req.Name

	file, err := r.fs.OpenWritable(r.artifactV4Path(runID, artifactName))

	if err != nil {
		panic(err)
	}
	file.Close()
	if err := r.writeArtifactV4Metadata(runID, artifactName, artifactV4Metadata{
		ContentType: artifactV4ContentType(body),
	}); err != nil {
		panic(err)
	}

	respData := CreateArtifactResponse{
		Ok:              true,
		SignedUploadUrl: r.buildArtifactURL("UploadArtifact", artifactName, runID),
	}
	r.sendProtbufBody(ctx, &respData)
}

func (r *artifactV4Routes) uploadArtifact(ctx *ArtifactContext) {
	task, artifactName, ok := r.verifySignature(ctx, "UploadArtifact")
	if !ok {
		return
	}

	comp := ctx.Req.URL.Query().Get("comp")
	switch comp {
	case "block", "appendBlock":
		file, err := r.fs.OpenAppendable(r.artifactV4Path(task, artifactName))

		if err != nil {
			panic(err)
		}
		defer file.Close()

		writer, ok := file.(io.Writer)
		if !ok {
			panic(errors.New("File is not writable"))
		}

		if ctx.Req.Body == nil {
			panic(errors.New("No body given"))
		}

		_, err = io.Copy(writer, ctx.Req.Body)
		if err != nil {
			panic(err)
		}
		file.Close()
		ctx.JSON(http.StatusCreated, "appended")
	case "blocklist":
		ctx.JSON(http.StatusCreated, "created")
	}
}

func (r *artifactV4Routes) finalizeArtifact(ctx *ArtifactContext) {
	var req FinalizeArtifactRequest

	if _, ok := r.parseProtbufBody(ctx, &req); !ok {
		return
	}
	_, _, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	respData := FinalizeArtifactResponse{
		Ok:         true,
		ArtifactId: artifactNameToID(req.Name),
	}
	r.sendProtbufBody(ctx, &respData)
}

func (r *artifactV4Routes) listArtifacts(ctx *ArtifactContext) {
	var req ListArtifactsRequest

	if _, ok := r.parseProtbufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	safePath := safeResolve(r.baseDir, fmt.Sprint(runID))

	entries, err := fs.ReadDir(r.rfs, safePath)
	if err != nil {
		panic(err)
	}

	list := []*ListArtifactsResponse_MonolithArtifact{}

	for _, entry := range entries {
		id := artifactNameToID(entry.Name())
		if (req.NameFilter == nil || req.NameFilter.Value == entry.Name()) && (req.IdFilter == nil || req.IdFilter.Value == id) {
			data := &ListArtifactsResponse_MonolithArtifact{
				Name:                    entry.Name(),
				CreatedAt:               timestamppb.Now(),
				DatabaseId:              id,
				WorkflowRunBackendId:    req.WorkflowRunBackendId,
				WorkflowJobRunBackendId: req.WorkflowJobRunBackendId,
				Size:                    0,
			}
			if info, err := entry.Info(); err == nil {
				data.Size = info.Size()
				data.CreatedAt = timestamppb.New(info.ModTime())
			}
			list = append(list, data)
		}
	}

	respData := ListArtifactsResponse{
		Artifacts: list,
	}
	r.sendProtbufBody(ctx, &respData)
}

func (r *artifactV4Routes) getSignedArtifactURL(ctx *ArtifactContext) {
	var req GetSignedArtifactURLRequest

	if _, ok := r.parseProtbufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	artifactName := req.Name

	respData := GetSignedArtifactURLResponse{}

	respData.SignedUrl = r.buildArtifactURL("DownloadArtifact", artifactName, runID)
	r.sendProtbufBody(ctx, &respData)
}

func (r *artifactV4Routes) downloadArtifact(ctx *ArtifactContext) {
	task, artifactName, ok := r.verifySignature(ctx, "DownloadArtifact")
	if !ok {
		return
	}

	file, err := r.rfs.Open(r.artifactV4Path(task, artifactName))
	if err != nil {
		ctx.Error(http.StatusNotFound, "Artifact not found")
		return
	}
	defer file.Close()

	metadata := r.readArtifactV4Metadata(task, artifactName)
	filename := artifactName
	if metadata.ContentType == ArtifactV4ContentEncoding {
		filename += ".zip"
	}
	ctx.Resp.Header().Set("Content-Type", metadata.ContentType)
	ctx.Resp.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	_, _ = io.Copy(ctx.Resp, file)
}

func (r *artifactV4Routes) deleteArtifact(ctx *ArtifactContext) {
	var req DeleteArtifactRequest

	if _, ok := r.parseProtbufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}
	safeRunPath := safeResolve(r.baseDir, fmt.Sprint(runID))
	safePath := safeResolve(safeRunPath, req.Name)

	_ = os.RemoveAll(safePath)

	respData := DeleteArtifactResponse{
		Ok:         true,
		ArtifactId: artifactNameToID(req.Name),
	}
	r.sendProtbufBody(ctx, &respData)
}
