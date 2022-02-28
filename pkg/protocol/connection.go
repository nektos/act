package protocol

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type VssConnection struct {
	Client         *http.Client
	TenantUrl      string
	ConnectionData *ConnectionData
	Token          string
	PoolId         int64
	TaskAgent      *TaskAgent
	Key            *rsa.PrivateKey
	Trace          bool
}

func (vssConnection *VssConnection) BuildUrl(relativePath string, ppath map[string]string, query map[string]string) string {
	url2, _ := url.Parse(vssConnection.TenantUrl)
	url := relativePath
	for p, v := range ppath {
		url = strings.ReplaceAll(url, "{"+p+"}", v)
	}
	re := regexp.MustCompile(`/*\{[^\}]+\}`)
	url = re.ReplaceAllString(url, "")
	url2.Path = path.Join(url2.Path, url)
	q := url2.Query()
	for p, v := range query {
		q.Add(p, v)
	}
	url2.RawQuery = q.Encode()
	return url2.String()
}

func (vssConnection *VssConnection) authorize() (*VssOAuthTokenResponse, error) {
	var authResponse *VssOAuthTokenResponse
	var err error
	authResponse, err = vssConnection.TaskAgent.Authorize(vssConnection.Client, vssConnection.Key)
	if err == nil {
		return authResponse, nil
	}
	return nil, err
}

func (vssConnection *VssConnection) Request(serviceId string, protocol string, method string, urlParameter map[string]string, queryParameter map[string]string, requestBody interface{}, responseBody interface{}) error {
	return vssConnection.RequestWithContext(context.Background(), serviceId, protocol, method, urlParameter, queryParameter, requestBody, responseBody)
}

func AddContentType(header http.Header, apiversion string) {
	header["Content-Type"] = []string{"application/json; charset=utf-8; api-version=" + apiversion}
	header["Accept"] = []string{"application/json; api-version=" + apiversion}
}

func AddBearer(header http.Header, token string) {
	header["Authorization"] = []string{"bearer " + token}
}

func AddHeaders(header http.Header) {
	header["X-VSS-E2EID"] = []string{uuid.NewString()}
	header["X-TFS-FedAuthRedirect"] = []string{"Suppress"}
	header["X-TFS-Session"] = []string{uuid.NewString()}
}

func (vssConnection *VssConnection) RequestWithContext(ctx context.Context, serviceId string, protocol string, method string, urlParameter map[string]string, queryParameter map[string]string, requestBody interface{}, responseBody interface{}) error {
	serv := vssConnection.ConnectionData.GetServiceDefinition(serviceId)
	if urlParameter == nil {
		urlParameter = map[string]string{}
	}
	urlParameter["area"] = serv.ServiceType
	urlParameter["resource"] = serv.DisplayName
	if queryParameter == nil {
		queryParameter = map[string]string{}
	}
	url := vssConnection.BuildUrl(serv.RelativePath, urlParameter, queryParameter)
	for i := 0; i < 2; i++ {
		var buf io.Reader = nil
		if requestBody != nil {
			if _buf, ok := requestBody.(*bytes.Buffer); ok {
				buf = _buf
			} else {
				_buf := new(bytes.Buffer)
				enc := json.NewEncoder(_buf)
				if err := enc.Encode(requestBody); err != nil {
					return err
				}
				buf = _buf
			}
		}
		request, err := http.NewRequestWithContext(ctx, method, url, buf)
		if err != nil {
			return err
		}
		if len(protocol) > 0 {
			AddContentType(request.Header, protocol)
		}
		AddHeaders(request.Header)
		if vssConnection.Trace {
			headerbuf := new(bytes.Buffer)
			_ = request.Header.Write(headerbuf)
			body := ""
			if _buf, ok := buf.(*bytes.Buffer); ok {
				body = _buf.String()
			}
			fmt.Printf("Http %v Request started %v Headers: %v Body: `%v`\n", method, url, headerbuf.String(), body)
		}
		AddBearer(request.Header, vssConnection.Token)

		response, err := vssConnection.Client.Do(request)
		if err != nil {
			return err
		}
		if response == nil {
			return fmt.Errorf("failed to send request response is nil")
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			if i == 0 && (response.StatusCode == 401 || response.StatusCode == 400) && vssConnection.TaskAgent != nil && vssConnection.Key != nil {
				authResponse, err := vssConnection.authorize()
				if err != nil {
					return err
				}
				vssConnection.Token = authResponse.AccessToken
				continue
			}
			body := ""
			if _buf, ok := buf.(*bytes.Buffer); ok {
				body = _buf.String()
			} else if requestBody != nil {
				if b, err := json.Marshal(requestBody); err == nil {
					body = string(b)
				}
			}
			bytes, err := ioutil.ReadAll(response.Body)
			if err != nil {
				bytes = []byte("no response: " + err.Error())
			}
			err = fmt.Errorf("request %v %v failed with status %v, requestBody: `%v` and responseBody: `%v`", method, url, response.StatusCode, body, string(bytes))
			if vssConnection.Trace {
				fmt.Println(err.Error())
			}
			return err
		}
		if responseBody != nil {
			if response.StatusCode != 200 {
				return io.EOF
			}
			if vssConnection.Trace {
				headerbuf := new(bytes.Buffer)
				_ = request.Header.Write(headerbuf)
				bytes, err := ioutil.ReadAll(response.Body)
				if err != nil {
					bytes = []byte("no response: " + err.Error())
				}
				fmt.Printf("Http %v Request succeeded %v Headers: %v Body: `%v`\n", method, url, headerbuf.String(), string(bytes))

				if err := json.Unmarshal(bytes, responseBody); err != nil {
					return err
				}
			} else {
				dec := json.NewDecoder(response.Body)
				if err := dec.Decode(responseBody); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return fmt.Errorf("failed to send request unable to authenticate")
}

func (vssConnection *VssConnection) GetAgentPools() (*TaskAgentPools, error) {
	_taskAgentPools := &TaskAgentPools{}
	if err := vssConnection.Request("a8c47e17-4d56-4a56-92bb-de7ea7dc65be", "", "GET", map[string]string{}, map[string]string{}, nil, _taskAgentPools); err != nil {
		return nil, err
	}
	return _taskAgentPools, nil
}
func (vssConnection *VssConnection) CreateSession() (*AgentMessageConnection, error) {
	session := &TaskAgentSession{}
	session.Agent = *vssConnection.TaskAgent
	session.UseFipsEncryption = false // Have to be set to false for "GitHub Enterprise Server 3.0.11", github.com reset it to false 24-07-2021
	session.OwnerName = "RUNNER"
	if err := vssConnection.Request("134e239e-2df3-4794-a6f6-24f1f19ec8dc", "5.1-preview", "POST", map[string]string{
		"poolId": fmt.Sprint(vssConnection.PoolId),
	}, map[string]string{}, session, session); err != nil {
		return nil, err
	}

	con := &AgentMessageConnection{VssConnection: vssConnection, TaskAgentSession: session}
	var err error
	con.Block, err = con.TaskAgentSession.GetSessionKey(vssConnection.Key)
	if err != nil {
		_ = con.Delete()
		return nil, err
	}
	return con, nil
}

func (vssConnection *VssConnection) LoadSession(session *TaskAgentSession) (*AgentMessageConnection, error) {
	con := &AgentMessageConnection{VssConnection: vssConnection, TaskAgentSession: session}
	var err error
	con.Block, err = con.TaskAgentSession.GetSessionKey(vssConnection.Key)
	if err != nil {
		_ = con.Delete()
		return nil, err
	}
	return con, nil
}

func (vssConnection *VssConnection) UpdateTimeLine(timelineId string, jobreq *AgentJobRequestMessage, wrap *TimelineRecordWrapper) error {
	return vssConnection.Request("8893bc5b-35b2-4be7-83cb-99e683551db4", "5.1-preview", "PATCH", map[string]string{
		"scopeIdentifier": jobreq.Plan.ScopeIdentifier,
		"planId":          jobreq.Plan.PlanId,
		"hubName":         jobreq.Plan.PlanType,
		"timelineId":      timelineId,
	}, map[string]string{}, wrap, nil)
}

func (vssConnection *VssConnection) UploadLogFile(timelineId string, jobreq *AgentJobRequestMessage, logContent string) int {
	log := &TaskLog{}
	p := "logs/" + uuid.NewString()
	log.Path = &p
	log.CreatedOn = time.Now().UTC().Format("2006-01-02T15:04:05")
	log.LastChangedOn = time.Now().UTC().Format("2006-01-02T15:04:05")

	vssConnection.Request("46f5667d-263a-4684-91b1-dff7fdcf64e2", "5.1-preview", "POST", map[string]string{
		"scopeIdentifier": jobreq.Plan.ScopeIdentifier,
		"planId":          jobreq.Plan.PlanId,
		"hubName":         jobreq.Plan.PlanType,
		"timelineId":      timelineId,
	}, map[string]string{}, log, log)
	vssConnection.Request("46f5667d-263a-4684-91b1-dff7fdcf64e2", "5.1-preview", "POST", map[string]string{
		"scopeIdentifier": jobreq.Plan.ScopeIdentifier,
		"planId":          jobreq.Plan.PlanId,
		"hubName":         jobreq.Plan.PlanType,
		"timelineId":      timelineId,
		"logId":           fmt.Sprint(log.Id),
	}, map[string]string{}, bytes.NewBufferString(logContent), nil)
	return log.Id
}

func (vssConnection *VssConnection) DeleteAgent(taskAgent *TaskAgent) error {
	return vssConnection.Request("e298ef32-5878-4cab-993c-043836571f42", "6.0-preview.2", "DELETE", map[string]string{
		"poolId":  fmt.Sprint(vssConnection.PoolId),
		"agentId": fmt.Sprint(taskAgent.Id),
	}, map[string]string{}, nil, nil)
}

func (vssConnection *VssConnection) FinishJob(e *JobEvent, plan *TaskOrchestrationPlanReference) error {
	return vssConnection.Request("557624af-b29e-4c20-8ab0-0399d2204f3f", "2.0-preview.1", "POST", map[string]string{
		"scopeIdentifier": plan.ScopeIdentifier,
		"planId":          plan.PlanId,
		"hubName":         plan.PlanType,
	}, map[string]string{}, e, nil)
}
