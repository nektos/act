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
// 1.2. Upload Zip Content to Blobstorage (authorized by the URL signature)
// PUT: http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact?sig=mO7y35r4GyjN7fwg0DTv3-Fv1NDXD84KLEgLpoPOtDI=&expires=2024-01-23+21%3A48%3A37.20833956+%2B0100+CET&artifactName=test&taskID=75&comp=block
// 1.3. Continue Upload Zip Content to Blobstorage (authorized by the URL signature), repeat until everything is uploaded
// PUT: http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/UploadArtifact?sig=mO7y35r4GyjN7fwg0DTv3-Fv1NDXD84KLEgLpoPOtDI=&expires=2024-01-23+21%3A48%3A37.20833956+%2B0100+CET&artifactName=test&taskID=75&comp=appendBlock
// 1.4. Unknown xml payload to Blobstorage (authorized by the URL signature), ignored for now
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
// 2.3. Download Zip from Blobstorage (authorized by the URL signature)
// GET: http://localhost:3000/twirp/github.actions.results.api.v1.ArtifactService/DownloadArtifact?sig=wHzFOwpF-6220-5CA0CIRmAX9VbiTC2Mji89UOqo1E8=&expires=2024-01-23+21%3A51%3A56.872846295+%2B0100+CET&artifactName=test&taskID=76

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
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
)

const signedURLLifetime = 60 * time.Minute

type artifactV4Routes struct {
	prefix string
	fs     WriteFS
	rfs    fs.FS
	// AppURL, when set, is the host[:port] used in signed URLs instead of the
	// request's Host header.
	AppURL  string
	baseDir string
	// signingKey is the HMAC key for signed upload and download URLs.
	// It is generated per process so a signature cannot be produced without a token from this server.
	signingKey []byte
}

func newSigningKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(fmt.Errorf("generate artifact url signing key: %w", err))
	}
	return key
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
		fs:         fsys,
		rfs:        rfs,
		baseDir:    baseDir,
		prefix:     ArtifactV4RouteBase,
		signingKey: newSigningKey(),
	}
	router.POST(path.Join(ArtifactV4RouteBase, "CreateArtifact"), func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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
		route.getSignedArtifactURL(&ArtifactContext{
			Req:  r,
			Resp: w,
		})
	})
	router.POST(path.Join(ArtifactV4RouteBase, "DeleteArtifact"), func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

func (r *artifactV4Routes) artifactZipPath(runID int64, name string) (string, bool) {
	if !validArtifactName(name) {
		return "", false
	}
	runDir := safeResolve(r.baseDir, strconv.FormatInt(runID, 10))
	dir, ok := pathUnder(runDir, name)
	if !ok {
		return "", false
	}
	return pathUnder(dir, name+".zip")
}

func (r artifactV4Routes) buildSignature(endp, expires, artifactName string, taskID int64) []byte {
	mac := hmac.New(sha256.New, r.signingKey)
	// Length-prefix each field so adjacent values cannot be rearranged into the same input.
	for _, field := range []string{endp, expires, artifactName, strconv.FormatInt(taskID, 10)} {
		mac.Write([]byte(strconv.Itoa(len(field))))
		mac.Write([]byte{0})
		mac.Write([]byte(field))
	}
	return mac.Sum(nil)
}

// signedURLHost returns the host[:port] for signed URLs. The request Host is
// only used when it is a bare authority, and is read per request so concurrent
// requests cannot change each other's URLs.
func (r artifactV4Routes) signedURLHost(req *http.Request) (string, bool) {
	if r.AppURL != "" {
		return strings.TrimSuffix(r.AppURL, "/"), true
	}
	u, err := url.Parse("http://" + req.Host)
	if err != nil || req.Host == "" || u.Host != req.Host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	return req.Host, true
}

func (r artifactV4Routes) buildArtifactURL(host, endp, artifactName string, taskID int64) string {
	expires := time.Now().Add(signedURLLifetime).Format("2006-01-02 15:04:05.999999999 -0700 MST")
	uploadURL := "http://" + host + strings.TrimSuffix(r.prefix, "/") +
		"/" + endp + "?sig=" + base64.URLEncoding.EncodeToString(r.buildSignature(endp, expires, artifactName, taskID)) + "&expires=" + url.QueryEscape(expires) + "&artifactName=" + url.QueryEscape(artifactName) + "&taskID=" + fmt.Sprint(taskID)
	return uploadURL
}

func (r artifactV4Routes) verifySignature(ctx *ArtifactContext, endp string) (int64, string, bool) {
	rawTaskID := ctx.Req.URL.Query().Get("taskID")
	sig := ctx.Req.URL.Query().Get("sig")
	expires := ctx.Req.URL.Query().Get("expires")
	artifactName := ctx.Req.URL.Query().Get("artifactName")
	dsig, _ := base64.URLEncoding.DecodeString(sig)
	taskID, _ := strconv.ParseInt(rawTaskID, 10, 64)

	expecedsig := r.buildSignature(endp, expires, artifactName, taskID)
	if !hmac.Equal(dsig, expecedsig) {
		log.Error("Error unauthorized")
		ctx.Error(http.StatusUnauthorized, "Error unauthorized")
		return -1, "", false
	}
	t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", expires)
	if now := time.Now(); err != nil || t.Before(now) || t.After(now.Add(signedURLLifetime+time.Minute)) {
		log.Error("Error link expired")
		ctx.Error(http.StatusUnauthorized, "Error link expired")
		return -1, "", false
	}
	return taskID, artifactName, true
}

func (r *artifactV4Routes) parseProtbufBody(ctx *ArtifactContext, req protoreflect.ProtoMessage) bool {
	body, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		log.Errorf("Error decode request body: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error decode request body")
		return false
	}
	err = protojson.Unmarshal(body, req)
	if err != nil {
		log.Errorf("Error decode request body: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error decode request body")
		return false
	}
	return true
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

	if ok := r.parseProtbufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	artifactName := req.Name
	safePath, ok := r.artifactZipPath(runID, artifactName)
	if !ok {
		ctx.Error(http.StatusBadRequest, "invalid path")
		return
	}
	host, ok := r.signedURLHost(ctx.Req)
	if !ok {
		ctx.Error(http.StatusBadRequest, "invalid host")
		return
	}
	file, err := r.fs.OpenWritable(safePath)

	if err != nil {
		panic(err)
	}
	file.Close()

	respData := CreateArtifactResponse{
		Ok:              true,
		SignedUploadUrl: r.buildArtifactURL(host, "UploadArtifact", artifactName, runID),
	}
	r.sendProtbufBody(ctx, &respData)
}

func (r *artifactV4Routes) uploadArtifact(ctx *ArtifactContext) {
	task, artifactName, ok := r.verifySignature(ctx, "UploadArtifact")
	if !ok {
		return
	}

	safePath, pathOK := r.artifactZipPath(task, artifactName)
	if !pathOK {
		ctx.Error(http.StatusBadRequest, "invalid path")
		return
	}

	comp := ctx.Req.URL.Query().Get("comp")
	switch comp {
	case "block", "appendBlock":
		file, err := r.fs.OpenAppendable(safePath)

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

	if ok := r.parseProtbufBody(ctx, &req); !ok {
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

	if ok := r.parseProtbufBody(ctx, &req); !ok {
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

	if ok := r.parseProtbufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}

	artifactName := req.Name
	if !validArtifactName(artifactName) {
		ctx.Error(http.StatusBadRequest, "invalid path")
		return
	}

	host, ok := r.signedURLHost(ctx.Req)
	if !ok {
		ctx.Error(http.StatusBadRequest, "invalid host")
		return
	}

	respData := GetSignedArtifactURLResponse{}

	respData.SignedUrl = r.buildArtifactURL(host, "DownloadArtifact", artifactName, runID)
	r.sendProtbufBody(ctx, &respData)
}

func (r *artifactV4Routes) downloadArtifact(ctx *ArtifactContext) {
	task, artifactName, ok := r.verifySignature(ctx, "DownloadArtifact")
	if !ok {
		return
	}

	safePath, pathOK := r.artifactZipPath(task, artifactName)
	if !pathOK {
		ctx.Error(http.StatusBadRequest, "invalid path")
		return
	}

	file, err := r.rfs.Open(safePath)
	if err != nil {
		log.Errorf("Error open artifact: %v", err)
		ctx.Error(http.StatusNotFound, "artifact not found")
		return
	}
	defer file.Close()

	_, _ = io.Copy(ctx.Resp, file)
}

func (r *artifactV4Routes) deleteArtifact(ctx *ArtifactContext) {
	var req DeleteArtifactRequest

	if ok := r.parseProtbufBody(ctx, &req); !ok {
		return
	}
	_, runID, ok := validateRunIDV4(ctx, req.WorkflowRunBackendId)
	if !ok {
		return
	}
	if !validArtifactName(req.Name) {
		ctx.Error(http.StatusBadRequest, "invalid path")
		return
	}
	safePath, pathOK := pathUnder(safeResolve(r.baseDir, strconv.FormatInt(runID, 10)), req.Name)
	if !pathOK {
		ctx.Error(http.StatusBadRequest, "invalid path")
		return
	}

	if rfs, ok := r.fs.(interface{ RemoveAll(string) error }); ok {
		_ = rfs.RemoveAll(safePath)
	} else {
		_ = os.RemoveAll(safePath)
	}

	respData := DeleteArtifactResponse{
		Ok:         true,
		ArtifactId: artifactNameToID(req.Name),
	}
	r.sendProtbufBody(ctx, &respData)
}
