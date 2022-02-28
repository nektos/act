package agent

import (
	"bytes"
	"context"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/nektos/act/pkg/protocol"

	"github.com/google/uuid"
	_ "github.com/mtibben/androiddnsfix"
	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner"
	"github.com/robertkrimen/otto"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type ghaFormatter struct {
	rqt            *protocol.AgentJobRequestMessage
	rc             *runner.RunContext
	wrap           *protocol.TimelineRecordWrapper
	current        *protocol.TimelineRecord
	updateTimeLine func()
	logline        func(startLine int64, recordId string, lines []string)
	uploadLogFile  func(log string) int
	startLine      int64
	stepBuffer     *bytes.Buffer
	linefeedregex  *regexp.Regexp
}

func (f *ghaFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	if f.rc.Parent == nil && (f.current == nil || f.current.RefName != f.rc.CurrentStep) {
		res, ok := f.rc.StepResults[f.current.RefName]
		if ok {
			f.startLine = 1
			if f.current != nil {
				if res.Conclusion == 0 {
					f.current.Complete("Succeeded")
				} else {
					f.current.Complete("Failed")
				}
				if f.stepBuffer.Len() > 0 {
					f.current.Log = &protocol.TaskLogReference{Id: f.uploadLogFile(f.stepBuffer.String())}
				}
			}
			f.stepBuffer = &bytes.Buffer{}
			for i := range f.wrap.Value {
				if f.wrap.Value[i].RefName == f.rc.CurrentStep {
					f.current = &f.wrap.Value[i]
					f.current.Start()
					break
				}
			}
			f.updateTimeLine()
		}
	}
	if f.rqt.MaskHints != nil {
		for _, v := range f.rqt.MaskHints {
			if strings.ToLower(v.Type) == "regex" {
				r, _ := regexp.Compile(v.Value)
				entry.Message = r.ReplaceAllString(entry.Message, "***")
			}
		}
	}
	if f.rqt.Variables != nil {
		for _, v := range f.rqt.Variables {
			if v.IsSecret && len(v.Value) > 0 {
				entry.Message = strings.ReplaceAll(entry.Message, v.Value, "***")
			}
		}
	}

	if f.linefeedregex == nil {
		f.linefeedregex = regexp.MustCompile(`(\r\n|\r|\n)`)
	}

	prefix := ""
	if entry.Level == logrus.DebugLevel {
		prefix = "##[debug]"
	} else if entry.Level == logrus.WarnLevel {
		prefix = "##[warning]"
	} else if entry.Level == logrus.ErrorLevel {
		prefix = "##[error]"
	}
	entry.Message = f.linefeedregex.ReplaceAllString(prefix+strings.Trim(entry.Message, "\r\n"), "\n"+prefix)

	b.WriteString(entry.Message)
	b.WriteByte('\n')
	lines := strings.Split(entry.Message, "\n")
	f.logline(f.startLine, f.current.Id, lines)
	f.startLine += int64(len(lines))
	f.stepBuffer.Write(b.Bytes())
	return b.Bytes(), nil
}

type ConfigureRemoveRunner struct {
	Url        string
	Name       string
	Token      string
	Pat        string
	Unattended bool
	Trace      bool
}

type ConfigureRunner struct {
	ConfigureRemoveRunner
	Labels          []string
	NoDefaultLabels bool
	SystemLabels    []string
	RunnerGroup     string
	Ephemeral       bool
	RunnerGuard     string
	Replace         bool
}

type RemoveRunner struct {
	ConfigureRemoveRunner
	Force bool
}

type RunnerInstance struct {
	PoolId          int64
	RegistrationUrl string
	Auth            *protocol.GitHubAuthResult
	Agent           *protocol.TaskAgent
	Key             string
	PKey            *rsa.PrivateKey `json:"-"`
	RunnerGuard     string
}

type RunnerSettings struct {
	PoolId          int64
	RegistrationUrl string
	Instances       []*RunnerInstance
}

func WriteJson(path string, value interface{}) error {
	b, err := json.MarshalIndent(value, "", "    ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, b, 0600)
}

func ReadJson(path string, value interface{}) error {
	cont, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(cont, value)
}

func (config *ConfigureRunner) Configure() int {
	settings := &RunnerSettings{RegistrationUrl: config.Url}
	_ = ReadJson("settings.json", settings)
	instance := &RunnerInstance{
		RunnerGuard: config.RunnerGuard,
	}
	loadConfiguration()
	if config.Ephemeral && len(settings.Instances) > 0 || containsEphemeralConfiguration() {
		fmt.Println("Ephemeral is not supported for multi runners, runner already configured.")
		return 1
	}
	settings.Instances = append(settings.Instances, instance)
	if len(config.Url) == 0 {
		if !config.Unattended {
			config.Url = GetInput("Please enter your repository, organization or enterprise url:", "")
		} else {
			fmt.Println("No url provided")
			return 1
		}
	}
	if len(config.Url) == 0 {
		fmt.Println("No url provided")
		return 1
	}
	instance.RegistrationUrl = config.Url
	c := &http.Client{
		Timeout: 100 * time.Second,
	}
	res, shouldReturn, returnValue := gitHubAuth(&config.ConfigureRemoveRunner, c, "register", "registration-token")
	if shouldReturn {
		return returnValue
	}

	instance.Auth = res
	vssConnection := &protocol.VssConnection{
		Client:    c,
		TenantUrl: res.TenantUrl,
		Token:     res.Token,
		Trace:     config.Trace,
	}
	vssConnection.ConnectionData = vssConnection.GetConnectionData()
	{
		taskAgentPool := ""
		taskAgentPools := []string{}
		_taskAgentPools, err := vssConnection.GetAgentPools()
		if err != nil {
			fmt.Printf("Failed to configure runner: %v\n", err)
			return 1
		}
		for _, val := range _taskAgentPools.Value {
			if !val.IsHosted {
				taskAgentPools = append(taskAgentPools, val.Name)
			}
		}
		if len(taskAgentPools) == 0 {
			fmt.Println("Failed to configure runner, no self-hosted runner group available")
			return 1
		}
		if len(config.RunnerGroup) > 0 {
			taskAgentPool = config.RunnerGroup
		} else {
			taskAgentPool = taskAgentPools[0]
			if len(taskAgentPools) > 1 && !config.Unattended {
				taskAgentPool = RunnerGroupSurvey(taskAgentPool, taskAgentPools)
			}
		}
		vssConnection.PoolId = -1
		for _, val := range _taskAgentPools.Value {
			if !val.IsHosted && strings.EqualFold(val.Name, taskAgentPool) {
				vssConnection.PoolId = val.Id
			}
		}
		if vssConnection.PoolId < 0 {
			fmt.Printf("Runner Pool %v not found\n", taskAgentPool)
			return 1
		}
	}
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	instance.Key = base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(key))

	taskAgent := &protocol.TaskAgent{}
	bs := make([]byte, 4)
	ui := uint32(key.E)
	binary.BigEndian.PutUint32(bs, ui)
	expof := 0
	for ; expof < 3 && bs[expof] == 0; expof++ {
	}
	taskAgent.Authorization.PublicKey = protocol.TaskAgentPublicKey{Exponent: base64.StdEncoding.EncodeToString(bs[expof:]), Modulus: base64.StdEncoding.EncodeToString(key.N.Bytes())}
	taskAgent.Version = "3.0.0" // version, will not use fips crypto if set to 0.0.0 *
	taskAgent.OSDescription = "github-act-runner " + runtime.GOOS + "/" + runtime.GOARCH
	if config.Name != "" {
		taskAgent.Name = config.Name
	} else {
		taskAgent.Name = "golang_" + uuid.NewString()
		if !config.Unattended {
			taskAgent.Name = GetInput("Please enter a name of your new runner:", taskAgent.Name)
		}
	}
	if !config.Unattended && len(config.Labels) == 0 {
		if res := GetInput("Please enter custom labels of your new runner (case insensitive, separated by ','):", ""); len(res) > 0 {
			config.Labels = strings.Split(res, ",")
		}
	}
	systemLabels := make([]string, 0, 3)
	if !config.NoDefaultLabels {
		systemLabels = append(systemLabels, "self-hosted", runtime.GOOS, runtime.GOARCH)
	}
	taskAgent.Labels = make([]protocol.AgentLabel, len(systemLabels)+len(config.SystemLabels)+len(config.Labels))
	for i := 0; i < len(systemLabels); i++ {
		taskAgent.Labels[i] = protocol.AgentLabel{Name: systemLabels[i], Type: "system"}
	}
	for i := 0; i < len(config.SystemLabels); i++ {
		taskAgent.Labels[i+len(systemLabels)] = protocol.AgentLabel{Name: config.SystemLabels[i], Type: "system"}
	}
	for i := 0; i < len(config.Labels); i++ {
		taskAgent.Labels[i+len(systemLabels)+len(config.SystemLabels)] = protocol.AgentLabel{Name: config.Labels[i], Type: "user"}
	}
	taskAgent.MaxParallelism = 1
	taskAgent.ProvisioningState = "Provisioned"
	taskAgent.CreatedOn = time.Now().UTC().Format("2006-01-02T15:04:05")
	taskAgent.Ephemeral = config.Ephemeral
	{
		err := vssConnection.Request("e298ef32-5878-4cab-993c-043836571f42", "6.0-preview.2", "POST", map[string]string{
			"poolId": fmt.Sprint(vssConnection.PoolId),
		}, map[string]string{}, taskAgent, taskAgent)
		// TODO Replace Runner support
		// {
		// 	poolsreq, _ := http.NewRequest("GET", url, nil)
		// 	AddBearer(poolsreq.Header, res.Token)
		// 	AddContentType(poolsreq.Header, "6.0-preview.2")
		// 	poolsresp, err := c.Do(poolsreq)
		// 	if err != nil {
		// 		fmt.Printf("Failed to create taskAgent: %v\n", err.Error())
		// 		return 1
		// 	} else if poolsresp.StatusCode != 200 {
		// 		bytes, _ := ioutil.ReadAll(poolsresp.Body)
		// 		fmt.Println(string(bytes))
		// 		fmt.Println(buf.String())
		// 		return 1
		// 	} else {
		// 		bytes, _ := ioutil.ReadAll(poolsresp.Body)
		// 		// fmt.Println(string(bytes))
		// 		taskAgent := ""
		// 		taskAgents := []string{}
		// 		// xttr := json.Unmarshal(bytes)
		// 		_taskAgents := &TaskAgents{}
		// 		json.Unmarshal(bytes, _taskAgents)
		// 		for _, val := range _taskAgents.Value {
		// 			taskAgents = append(taskAgents, val.Name)
		// 		}
		// 		prompt := &survey.Select{
		// 			Message: "Choose a runner:",
		// 			Options: taskAgents,
		// 		}
		// 		survey.AskOne(prompt, &taskAgent)
		// 	}
		// }
		if err != nil {
			if !config.Replace {
				fmt.Printf("Failed to create taskAgent: %v\n", err.Error())
				return 1
			}
			// Try replaceing runner if creation failed
			taskAgents := &protocol.TaskAgents{}
			err := vssConnection.Request("e298ef32-5878-4cab-993c-043836571f42", "6.0-preview.2", "GET", map[string]string{
				"poolId": fmt.Sprint(vssConnection.PoolId),
			}, map[string]string{}, nil, taskAgents)
			if err != nil {
				fmt.Printf("Failed to update taskAgent: %v\n", err.Error())
				return 1
			}
			invalid := true
			for i := 0; i < len(taskAgents.Value); i++ {
				if taskAgents.Value[i].Name == taskAgent.Name {
					taskAgent.Id = taskAgents.Value[i].Id
					invalid = false
					break
				}
			}
			if invalid {
				fmt.Println("Failed to update taskAgent: Failed to find agent")
				return 1
			}
			err = vssConnection.Request("e298ef32-5878-4cab-993c-043836571f42", "6.0-preview.2", "PUT", map[string]string{
				"poolId":  fmt.Sprint(vssConnection.PoolId),
				"agentId": fmt.Sprint(taskAgent.Id),
			}, map[string]string{}, taskAgent, taskAgent)
			if err != nil {
				fmt.Printf("Failed to update taskAgent: %v\n", err.Error())
				return 1
			}
		}
	}
	instance.Agent = taskAgent
	instance.PoolId = vssConnection.PoolId
	if err := WriteJson("settings.json", settings); err != nil {
		fmt.Printf("Failed to save settings.json: %v\n", err.Error())
	}
	fmt.Println("success")
	return 0
}

func gitHubAuth(config *ConfigureRemoveRunner, c *http.Client, runnerEvent string, apiEndpoint string) (*protocol.GitHubAuthResult, bool, int) {
	registerUrl, err := url.Parse(config.Url)
	if err != nil {
		fmt.Printf("Invalid Url: %v\n", config.Url)
		return nil, true, 1
	}
	apiscope := "/"
	if strings.ToLower(registerUrl.Host) == "github.com" {
		registerUrl.Host = "api." + registerUrl.Host
	} else {
		apiscope = "/api/v3"
	}

	if len(config.Token) == 0 {
		if len(config.Pat) > 0 {
			paths := strings.Split(strings.TrimPrefix(registerUrl.Path, "/"), "/")
			url := *registerUrl
			if len(paths) == 1 {
				url.Path = path.Join(apiscope, "orgs", paths[0], "actions/runners", apiEndpoint)
			} else if len(paths) == 2 {
				scope := "repos"
				if strings.EqualFold(paths[0], "enterprises") {
					scope = ""
				}
				url.Path = path.Join(apiscope, scope, paths[0], paths[1], "actions/runners", apiEndpoint)
			} else {
				fmt.Println("Unsupported registration url")
				return nil, true, 1
			}
			req, _ := http.NewRequest("POST", url.String(), nil)
			req.SetBasicAuth("github", config.Pat)
			req.Header.Add("Accept", "application/vnd.github.v3+json")
			resp, err := c.Do(req)
			if err != nil {
				fmt.Printf("Failed to retrieve %v token via pat: %v\n", apiEndpoint, err.Error())
				return nil, true, 1
			}
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				body, _ := ioutil.ReadAll(resp.Body)
				fmt.Printf("Failed to retrieve %v via pat [%v]: %v\n", apiEndpoint, fmt.Sprint(resp.StatusCode), string(body))
				return nil, true, 1
			}
			tokenresp := &protocol.GitHubRunnerRegisterToken{}
			dec := json.NewDecoder(resp.Body)
			if err := dec.Decode(tokenresp); err != nil {
				fmt.Printf("Failed to decode registration token via pat: " + err.Error())
				return nil, true, 1
			}
			config.Token = tokenresp.Token
		} else {
			if !config.Unattended {
				config.Token = GetInput("Please enter your runner registration token:", "")
			}
		}
	}
	if len(config.Token) == 0 {
		fmt.Println("No runner registration token provided")
		return nil, true, 1
	}
	registerUrl.Path = path.Join(apiscope, "actions/runner-registration")

	buf := new(bytes.Buffer)
	req := &protocol.RunnerAddRemove{}
	req.Url = config.Url
	req.RunnerEvent = runnerEvent
	enc := json.NewEncoder(buf)
	if err := enc.Encode(req); err != nil {
		return nil, true, 1
	}
	finalregisterUrl := registerUrl.String()

	r, _ := http.NewRequest("POST", finalregisterUrl, buf)
	r.Header["Authorization"] = []string{"RemoteAuth " + config.Token}
	resp, err := c.Do(r)
	if err != nil {
		fmt.Printf("Failed to register Runner: %v\n", err)
		return nil, true, 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Printf("Failed to register Runner with status code: %v\n", resp.StatusCode)
		return nil, true, 1
	}

	res := &protocol.GitHubAuthResult{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(res); err != nil {
		fmt.Printf("error decoding struct from JSON: %v\n", err)
		return nil, true, 1
	}
	return res, false, 0
}

type RunRunner struct {
	Once     bool
	Terminal bool
	Trace    bool
}

type JobRun struct {
	RequestId       int64
	JobId           string
	Plan            *protocol.TaskOrchestrationPlanReference
	Name            string
	RegistrationUrl string
}

func ToStringMap(src interface{}) interface{} {
	bi, ok := src.(map[interface{}]interface{})
	if ok {
		res := make(map[string]interface{})
		for k, v := range bi {
			res[k.(string)] = ToStringMap(v)
		}
		return res
	}
	return src
}

func readLegacyInstance(settings *RunnerSettings, instance *RunnerInstance) int {
	taskAgent := &protocol.TaskAgent{}
	var key *rsa.PrivateKey
	req := &protocol.GitHubAuthResult{}
	{
		cont, err := ioutil.ReadFile("agent.json")
		if err != nil {
			return 1
		}
		err = json.Unmarshal(cont, taskAgent)
		if err != nil {
			return 1
		}
	}
	{
		cont, err := ioutil.ReadFile("cred.pkcs1")
		if err != nil {
			return 1
		}
		key, err = x509.ParsePKCS1PrivateKey(cont)
		if err != nil {
			return 1
		}
	}
	{
		cont, err := ioutil.ReadFile("auth.json")
		if err != nil {
			return 1
		}
		err = json.Unmarshal(cont, req)
		if err != nil {
			return 1
		}
	}
	instance.Agent = taskAgent
	instance.PKey = key
	instance.PoolId = settings.PoolId
	instance.RegistrationUrl = settings.RegistrationUrl
	instance.Auth = req
	return 0
}

func loadConfiguration() (*RunnerSettings, error) {
	settings := &RunnerSettings{}
	{
		cont, err := ioutil.ReadFile("settings.json")
		if err != nil {
			// Backward compat <= 0.0.3
			// fmt.Printf("The runner needs to be configured first: %v\n", err.Error())
			// return 1
			settings.PoolId = 1
		} else {
			err = json.Unmarshal(cont, settings)
			if err != nil {
				return nil, err
			}
		}
	}
	{
		for i := 0; i < len(settings.Instances); i++ {
			key, _ := base64.StdEncoding.DecodeString(settings.Instances[i].Key)
			pkey, _ := x509.ParsePKCS1PrivateKey(key)
			settings.Instances[i].PKey = pkey
		}
		instance := &RunnerInstance{}
		if readLegacyInstance(settings, instance) == 0 {
			settings.Instances = append(settings.Instances, instance)
		}
	}
	return settings, nil
}

func containsEphemeralConfiguration() bool {
	settings, err := loadConfiguration()
	if err != nil || settings == nil {
		return false
	}
	for _, instance := range settings.Instances {
		if instance.Agent != nil && instance.Agent.Ephemeral {
			return true
		}
	}
	return false
}

func (run *RunRunner) Run() int {
	// act fork
	// container.SetContainerAllocateTerminal(run.Terminal)
	// trap Ctrl+C
	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	firstJobReceived := false
	jobctx, cancelJob := context.WithCancel(context.Background())
	cancelJob()
	go func() {
		<-channel
		select {
		case <-jobctx.Done():
			fmt.Println("CTRL+C received, no job is running shutdown")
			cancel()
		default:
			fmt.Println("CTRL+C received, stop accepting new jobs and exit after the current job finishes")
			// Switch to run once mode
			run.Once = true
			firstJobReceived = true
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-channel:
				fmt.Println("CTRL+C received again, cancel current Job if it is still running")
				cancel()
			}
		}
	}()
	defer func() {
		cancel()
		signal.Stop(channel)
	}()
	defer func() {
		<-jobctx.Done()
	}()
	settings, err := loadConfiguration()
	if err != nil {
		fmt.Printf("settings.json is corrupted: %v, please reconfigure the runner\n", err.Error())
		return 1
	}
	if len(settings.Instances) <= 0 {
		fmt.Printf("Please configure the runner")
		return 1
	}
	isEphemeral := len(settings.Instances) == 1 && settings.Instances[0].Agent.Ephemeral
	// isEphemeral => run.Once
	run.Once = run.Once || isEphemeral
	defer func() {
		if firstJobReceived && isEphemeral {
			if err := os.Remove("settings.json"); err != nil {
				fmt.Printf("Warning: Cannot delete settings.json after ephemeral exit: %v\n", err.Error())
			}
			if err := os.Remove("sessions.json"); err != nil {
				fmt.Printf("Warning: Cannot delete sessions.json after ephemeral exit: %v\n", err.Error())
			}
		}
	}()
	var sessions []*protocol.TaskAgentSession
	if err := ReadJson("sessions.json", &sessions); err != nil && run.Trace {
		fmt.Printf("sessions.json is corrupted or does not exist: %v\n", err.Error())
	}
	{
		// Backward compatibility
		var session protocol.TaskAgentSession
		if err := ReadJson("session.json", &session); err != nil {
			if run.Trace {
				fmt.Printf("session.json is corrupted or does not exist: %v\n", err.Error())
			}
		} else {
			sessions = append(sessions, &session)
			// Save new format
			WriteJson("sessions.json", sessions)
			// Cleanup old files
			if err := os.Remove("session.json"); err != nil {
				fmt.Printf("Warning: Cannot delete session.json: %v\n", err.Error())
			}
		}
	}

	firstRun := true

	for {
		mu := &sync.Mutex{}
		joblisteningctx, cancelJobListening := context.WithCancel(ctx)
		defer cancelJobListening()
		wg := new(sync.WaitGroup)
		wg.Add(len(settings.Instances))
		deleteSessions := firstRun
		firstRun = false
		// No retry on Fatal failures, like runner was removed or we received multiple jobs
		fatalFailure := false
		for _, instance := range settings.Instances {
			go func(instance *RunnerInstance) (exitcode int) {
				defer wg.Done()
				defer func() {
					// Without this the inner return 1 got lost and we would retry it
					if exitcode != 0 {
						fatalFailure = true
					}
				}()
				vssConnection := &protocol.VssConnection{
					Client: &http.Client{
						Timeout: 100 * time.Second,
						Transport: &http.Transport{
							MaxIdleConns:    1,
							IdleConnTimeout: 100 * time.Second,
						},
					},
					TenantUrl: instance.Auth.TenantUrl,
					PoolId:    instance.PoolId,
					TaskAgent: instance.Agent,
					Key:       instance.PKey,
					Trace:     run.Trace,
				}
				for i := 1; ; {
					vssConnection.ConnectionData = vssConnection.GetConnectionData()
					if vssConnection.ConnectionData != nil {
						break
					}
					maxtime := 60 * 10
					var dtime time.Duration = time.Duration(i)
					if i < maxtime {
						i *= 2
					} else {
						dtime = time.Duration(maxtime)
					}
					fmt.Printf("Retry retrieving connectiondata from the server in %v seconds\n", dtime)
					select {
					case <-ctx.Done():
						return 0
					case <-time.After(time.Second * dtime):
					}
				}
				jobrun := &JobRun{}
				if ReadJson("jobrun.json", jobrun) == nil && ((jobrun.RegistrationUrl == instance.RegistrationUrl && jobrun.Name == instance.Agent.Name) || (len(settings.Instances) == 1)) {
					result := "Failed"
					finish := &protocol.JobEvent{
						Name:      "JobCompleted",
						JobId:     jobrun.JobId,
						RequestId: jobrun.RequestId,
						Result:    result,
					}
					go func() {
						for i := 0; ; i++ {
							if err := vssConnection.FinishJob(finish, jobrun.Plan); err != nil {
								fmt.Printf("Failed to finish previous stuck job with Status Failed: %v\n", err.Error())
							} else {
								fmt.Println("Finished previous stuck job with Status Failed")
								break
							}
							if i < 10 {
								fmt.Printf("Retry finishing the job in 10 seconds attempt %v of 10\n", i+1)
								<-time.After(time.Second * 10)
							} else {
								break
							}
						}
					}()
					os.Remove("jobrun.json")
				}
				mu.Lock()
				var _session *protocol.AgentMessageConnection = nil
				for _, session := range sessions {
					if session.Agent.Name == instance.Agent.Name && session.Agent.Authorization.PublicKey == instance.Agent.Authorization.PublicKey {
						session, err := vssConnection.LoadSession(session)
						if deleteSessions {
							_ = session.Delete()
							for i, _session := range sessions {
								if session.TaskAgentSession.SessionId == _session.SessionId {
									sessions[i] = sessions[len(sessions)-1]
									sessions = sessions[:len(sessions)-1]
								}
							}
							WriteJson("sessions.json", sessions)
						} else if err == nil {
							_session = session
						}
					}
				}
				mu.Unlock()
				var session *protocol.AgentMessageConnection
				if _session != nil {
					session = _session
				}
				deleteSession := func() {
					if session != nil {
						if err := session.Delete(); err != nil {
							fmt.Printf("WARNING: Failed to delete active session: %v\n", err)
						} else {
							mu.Lock()
							for i, _session := range sessions {
								if session.TaskAgentSession.SessionId == _session.SessionId {
									sessions[i] = sessions[len(sessions)-1]
									sessions = sessions[:len(sessions)-1]
								}
							}
							WriteJson("sessions.json", sessions)
							session = nil
							mu.Unlock()
						}
					}
				}
				defer deleteSession()
				xctx, _c := context.WithCancel(joblisteningctx)
				lastSuccess := time.Now()
				defer _c()
				for {
					message := &protocol.TaskAgentMessage{}
					success := false
					for !success {
						select {
						case <-joblisteningctx.Done():
							return 0
						default:
						}
						if session == nil || time.Now().After(lastSuccess.Add(5*time.Minute)) {
							deleteSession()
							session2, err := vssConnection.CreateSession()
							if err != nil {
								if strings.Contains(err.Error(), "invalid_client") || strings.Contains(err.Error(), "TaskAgentNotFoundException") {
									fmt.Printf("Fatal: It seems this runner was removed from GitHub, Failed to recreate Session for %v ( %v ): %v\n", instance.Agent.Name, instance.RegistrationUrl, err.Error())
									return 1
								}
								fmt.Printf("Failed to recreate Session for %v ( %v ), waiting 30 sec before retry: %v\n", instance.Agent.Name, instance.RegistrationUrl, err.Error())
								select {
								case <-joblisteningctx.Done():
									return 0
								case <-time.After(30 * time.Second):
								}
								continue
							} else if session2 != nil {
								session = session2
								mu.Lock()
								sessions = append(sessions, session.TaskAgentSession)
								err := WriteJson("sessions.json", sessions)
								if err != nil {
									fmt.Printf("error: %v\n", err)
								} else {
									fmt.Printf("Listening for Jobs: %v ( %v )\n", instance.Agent.Name, instance.RegistrationUrl)
								}
								mu.Unlock()
							} else {
								fmt.Println("Failed to recreate Session, waiting 30 sec before retry")
								select {
								case <-joblisteningctx.Done():
									return 0
								case <-time.After(30 * time.Second):
								}
								continue
							}
						}
						err := vssConnection.RequestWithContext(xctx, "c3a054f6-7a8a-49c0-944e-3a8e5d7adfd7", "5.1-preview", "GET", map[string]string{
							"poolId": fmt.Sprint(instance.PoolId),
						}, map[string]string{
							"sessionId": session.TaskAgentSession.SessionId,
						}, nil, message)
						if err != nil {
							if errors.Is(err, context.Canceled) {
								return 0
							} else if !errors.Is(err, io.EOF) {
								if strings.Contains(err.Error(), "TaskAgentSessionExpiredException") {
									fmt.Printf("Failed to get message, Session expired: %v\n", err.Error())
									session = nil
									continue
								} else if strings.Contains(err.Error(), "AccessDeniedException") {
									fmt.Printf("Failed to get message, GitHub has rejected our authorization, recreate Session earlier: %v\n", err.Error())
									session = nil
									continue
								} else {
									fmt.Printf("Failed to get message, waiting 10 sec before retry: %v\n", err.Error())
									select {
									case <-joblisteningctx.Done():
										return 0
									case <-time.After(10 * time.Second):
									}
								}
							} else {
								lastSuccess = time.Now()
							}
						} else {
							lastSuccess = time.Now()
							if firstJobReceived && strings.EqualFold(message.MessageType, "PipelineAgentJobRequest") {
								// It seems run once isn't supported by the backend, do the same as the official runner
								// Skip deleting the job message and cancel earlier
								fmt.Println("Received a second job, but running in run once mode abort")
								return 1
							}
							success = true
							err := vssConnection.Request("c3a054f6-7a8a-49c0-944e-3a8e5d7adfd7", "5.1-preview", "DELETE", map[string]string{
								"poolId":    fmt.Sprint(instance.PoolId),
								"messageId": fmt.Sprint(message.MessageId),
							}, map[string]string{
								"sessionId": session.TaskAgentSession.SessionId,
							}, nil, nil)
							if err != nil {
								fmt.Println("Failed to delete Message")
								success = false
							}
						}
					}
					if success {
						if message != nil && strings.EqualFold(message.MessageType, "PipelineAgentJobRequest") {
							cancelJobListening()
							for message != nil && !firstJobReceived && strings.EqualFold(message.MessageType, "PipelineAgentJobRequest") {
								if run.Once {
									firstJobReceived = true
								}
								var finishJob context.CancelFunc
								jobctx, finishJob = context.WithCancel(context.Background())
								var jobExecCtx context.Context
								jobExecCtx, cancelJob = context.WithCancel(ctx)
								runJob(vssConnection, run, cancel, cancelJob, finishJob, jobExecCtx, jobctx, session, *message, instance)
								{
									message, err = session.GetNextMessage(jobExecCtx)
									if !errors.Is(err, context.Canceled) && message != nil {
										if firstJobReceived && strings.EqualFold(message.MessageType, "PipelineAgentJobRequest") {
											fmt.Println("Skip deleting the duplicated job request, we hope that the actions service reschedules your job to a different runner")
										} else {
											_ = session.DeleteMessage(message)
										}
										if strings.EqualFold(message.MessageType, "JobCancellation") && cancelJob != nil {
											message = nil
											fmt.Println("JobCancellation request received, cancel running job")
											cancelJob()
										} else {
											fmt.Println("Received message, while still executing a job, of type: " + message.MessageType)
										}
										fmt.Println("Wait for worker to finish current job")
										<-jobctx.Done()
									}
								}
							}
							// Skip deleting session for ephemeral, since the official actions service throws access denied
							if !run.Once || isEphemeral {
								session = nil
							}
						}
						if message != nil {
							fmt.Println("Ignoring incoming message of type: " + message.MessageType)
						}
					}
				}
			}(instance)
		}
		wg.Wait()
		if fatalFailure {
			return 1
		}
		select {
		case <-jobctx.Done():
			if run.Once {
				return 0
			}
		case <-ctx.Done():
			return 0
		}
	}
}

func runJob(vssConnection *protocol.VssConnection, run *RunRunner, cancel context.CancelFunc, cancelJob context.CancelFunc, finishJob context.CancelFunc, jobExecCtx context.Context, jobctx context.Context, session *protocol.AgentMessageConnection, message protocol.TaskAgentMessage, instance *RunnerInstance) {
	go func() {
		defer func() {
			if run.Once {
				// cancel Message Loop
				fmt.Println("Last Job finished, cancel Message loop")
				cancel()
			}
			cancelJob()
			finishJob()
		}()
		iv, _ := base64.StdEncoding.DecodeString(message.IV)
		src, _ := base64.StdEncoding.DecodeString(message.Body)
		cbcdec := cipher.NewCBCDecrypter(session.Block, iv)
		cbcdec.CryptBlocks(src, src)
		maxlen := session.Block.BlockSize()
		validlen := len(src)
		if int(src[len(src)-1]) < maxlen {
			ok := true
			for i := 2; i <= int(src[len(src)-1]); i++ {
				if src[len(src)-i] != src[len(src)-1] {
					ok = false
					break
				}
			}
			if ok {
				validlen -= int(src[len(src)-1])
			}
		}
		off := 0
		// skip utf8 bom, c# cryptostream uses it for utf8
		if src[0] == 239 && src[1] == 187 && src[2] == 191 {
			off = 3
		}
		if run.Trace {
			fmt.Println(string(src[off:validlen]))
		}
		jobreq := &protocol.AgentJobRequestMessage{}
		{
			dec := json.NewDecoder(bytes.NewReader(src[off:validlen]))
			if err := dec.Decode(jobreq); err != nil {
				fmt.Printf("Fatal failed to parse job request %v\n", err)
				return
			}
		}
		jobrun := &JobRun{
			RequestId:       jobreq.RequestId,
			JobId:           jobreq.JobId,
			Plan:            jobreq.Plan,
			RegistrationUrl: instance.RegistrationUrl,
			Name:            instance.Agent.Name,
		}
		{
			if err := WriteJson("jobrun.json", jobrun); err != nil {
				fmt.Printf("INFO: Failed to create jobrun.json: %v\n", err)
			}
		}
		fmt.Printf("Running Job '%v' of %v ( %v )\n", jobreq.JobDisplayName, instance.Agent.Name, instance.RegistrationUrl)
		finishJob2 := func(result string, outputs *map[string]protocol.VariableValue) {
			finish := &protocol.JobEvent{
				Name:      "JobCompleted",
				JobId:     jobreq.JobId,
				RequestId: jobreq.RequestId,
				Result:    result,
				Outputs:   outputs,
			}
			for i := 0; ; i++ {
				if err := vssConnection.FinishJob(finish, jobrun.Plan); err != nil {
					fmt.Printf("Failed to finish Job '%v' with Status %v: %v\n", jobreq.JobDisplayName, result, err.Error())
				} else {
					fmt.Printf("Finished Job '%v' with Status %v of %v ( %v )\n", jobreq.JobDisplayName, result, instance.Agent.Name, instance.RegistrationUrl)
					break
				}
				if i < 10 {
					fmt.Printf("Retry finishing '%v' in 10 seconds attempt %v of 10\n", jobreq.JobDisplayName, i+1)
					<-time.After(time.Second * 10)
				} else {
					break
				}
			}
			os.Remove("jobrun.json")
		}
		finishJob := func(result string) {
			finishJob2(result, nil)
		}
		rqt := jobreq
		secrets := map[string]string{}
		if rqt.Variables != nil {
			for k, v := range rqt.Variables {
				if v.IsSecret && k != "system.github.token" {
					secrets[k] = v.Value
				}
			}
			if rawGithubToken, ok := rqt.Variables["system.github.token"]; ok {
				secrets["GITHUB_TOKEN"] = rawGithubToken.Value
			}
		}
		runnerConfig := &runner.Config{
			Secrets: secrets,
			CompositeRestrictions: &model.CompositeRestrictions{
				AllowCompositeUses:            true,
				AllowCompositeIf:              true,
				AllowCompositeContinueOnError: true,
			},
		}
		if len(instance.RunnerGuard) > 0 {
			vm := otto.New()
			{
				var req interface{}
				e := json.Unmarshal(src[off:validlen], &req)
				fmt.Println(e)
				_ = vm.Set("runnerInstance", instance)
				_ = vm.Set("jobrequest", req)
				_ = vm.Set("jobrun", jobrun)
				_ = vm.Set("runnerConfig", runnerConfig)
				_ = vm.Set("TemplateTokenToObject", func(p interface{}) interface{} {
					val, err := vm.Call("JSON.stringify", nil, p)
					if err != nil {
						panic(vm.MakeCustomError("TemplateTokenToObject", err.Error()))
					}
					s, err := val.ToString()
					if err != nil {
						panic(vm.MakeCustomError("TemplateTokenToObject", err.Error()))
					}
					var token protocol.TemplateToken
					err = json.Unmarshal([]byte(s), &token)
					if err != nil {
						panic(vm.MakeCustomError("TemplateTokenToObject", err.Error()))
					}
					return token.ToJsonRawObject()
				})
				contextData := make(map[string]interface{})
				if jobreq.ContextData != nil {
					for k, ctxdata := range jobreq.ContextData {
						contextData[k] = ctxdata.ToRawObject()
					}
				}
				_ = vm.Set("contextData", contextData)
				val, err := vm.Run(instance.RunnerGuard)
				if err != nil {
					fmt.Printf("Failed to run `%v`: %v", instance.RunnerGuard, err)
					finishJob("Failed")
					return
				}
				res, _ := val.ToBoolean()
				if !res {
					finishJob("Failed")
					return
				}
			}
		}
		wrap := &protocol.TimelineRecordWrapper{}
		wrap.Count = 2
		wrap.Value = make([]protocol.TimelineRecord, wrap.Count)
		wrap.Value[0] = protocol.CreateTimelineEntry("", rqt.JobName, rqt.JobDisplayName)
		wrap.Value[0].Id = rqt.JobId
		wrap.Value[0].Type = "Job"
		wrap.Value[0].Order = 0
		wrap.Value[0].Start()
		wrap.Value[1] = protocol.CreateTimelineEntry(rqt.JobId, "__setup", "Setup Job")
		wrap.Value[1].Order = 1
		wrap.Value[1].Start()
		_ = vssConnection.UpdateTimeLine(jobreq.Timeline.Id, jobreq, wrap)
		failInitJob := func(message string) {
			wrap.Value[1].Log = &protocol.TaskLogReference{Id: vssConnection.UploadLogFile(jobreq.Timeline.Id, jobreq, message)}
			wrap.Value[1].Complete("Failed")
			wrap.Value[0].Complete("Failed")
			_ = vssConnection.UpdateTimeLine(jobreq.Timeline.Id, jobreq, wrap)
			fmt.Println(message)
			finishJob("Failed")
		}
		defer func() {
			if err := recover(); err != nil {
				failInitJob("The worker panicked with message: " + fmt.Sprint(err) + "\n" + string(debug.Stack()))
			}
		}()
		con := *vssConnection
		go func() {
			for {
				err := con.Request("fc825784-c92a-4299-9221-998a02d1b54f", "5.1-preview", "PATCH", map[string]string{
					"poolId":    fmt.Sprint(instance.PoolId),
					"requestId": fmt.Sprint(jobreq.RequestId),
				}, map[string]string{
					"lockToken": "00000000-0000-0000-0000-000000000000",
				}, &protocol.RenewAgent{RequestId: jobreq.RequestId}, nil)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					} else {
						fmt.Printf("Failed to renew job: %v\n", err.Error())
					}
				}
				select {
				case <-jobctx.Done():
					return
				case <-time.After(60 * time.Second):
				}
			}
		}()
		if jobreq.Resources == nil {
			failInitJob("Missing Job Resources")
			return
		}
		if jobreq.Resources.Endpoints == nil {
			failInitJob("Missing Job Resources Endpoints")
			return
		}
		// orchid := ""
		cacheUrl := ""
		idTokenUrl := ""
		for _, endpoint := range jobreq.Resources.Endpoints {
			if strings.EqualFold(endpoint.Name, "SystemVssConnection") && endpoint.Authorization.Parameters != nil && endpoint.Authorization.Parameters["AccessToken"] != "" {
				jobToken := endpoint.Authorization.Parameters["AccessToken"]
				jobTenant := endpoint.Url
				// Seems to be not required, but actions/runner did that to get orchid which was passed to some api calls
				// claims := jwt.MapClaims{}
				// jwt.ParseWithClaims(jobToken, claims, func(t *jwt.Token) (interface{}, error) {
				// 	return nil, nil
				// })
				// if _orchid, suc := claims["orchid"]; suc {
				// 	orchid = _orchid.(string)
				// }
				_cacheUrl, ok := endpoint.Data["CacheServerUrl"]
				if ok {
					cacheUrl = _cacheUrl
				}
				_idTokenUrl, ok := endpoint.Data["GenerateIdTokenUrl"]
				if ok {
					idTokenUrl = _idTokenUrl
				}
				vssConnection = &protocol.VssConnection{
					Client:    vssConnection.Client,
					TenantUrl: jobTenant,
					Token:     jobToken,
					Trace:     run.Trace,
				}
				vssConnection.ConnectionData = vssConnection.GetConnectionData()
			}
		}

		rawGithubCtx, ok := rqt.ContextData["github"]
		if !ok {
			fmt.Println("missing github context in ContextData")
			finishJob("Failed")
			return
		}
		githubCtx := rawGithubCtx.ToRawObject()
		matrix := make(map[string]interface{})
		if rawMatrix, ok := rqt.ContextData["matrix"]; ok {
			rawobj := rawMatrix.ToRawObject()
			if tmpmatrix, ok := rawobj.(map[string]interface{}); ok {
				matrix = tmpmatrix
			} else if rawobj != nil {
				failInitJob("matrix: not a map")
				return
			}
		}
		env := make(map[string]string)
		if rqt.EnvironmentVariables != nil {
			for _, rawenv := range rqt.EnvironmentVariables {
				if tmpenv, ok := rawenv.ToRawObject().(map[interface{}]interface{}); ok {
					for k, v := range tmpenv {
						key, ok := k.(string)
						if !ok {
							failInitJob("env key: act doesn't support non strings")
							return
						}
						value, ok := v.(string)
						if !ok {
							failInitJob("env value: act doesn't support non strings")
							return
						}
						env[key] = value
					}
				} else {
					failInitJob("env: not a map")
					return
				}
			}
		}
		env["ACTIONS_RUNTIME_URL"] = vssConnection.TenantUrl
		env["ACTIONS_RUNTIME_TOKEN"] = vssConnection.Token
		if len(cacheUrl) > 0 {
			env["ACTIONS_CACHE_URL"] = cacheUrl
		}
		if len(idTokenUrl) > 0 {
			env["ACTIONS_ID_TOKEN_REQUEST_URL"] = idTokenUrl
			env["ACTIONS_ID_TOKEN_REQUEST_TOKEN"] = vssConnection.Token
		}

		defaults := model.Defaults{}
		if rqt.Defaults != nil {
			for _, rawenv := range rqt.Defaults {
				rawobj := rawenv.ToRawObject()
				rawobj = ToStringMap(rawobj)
				b, err := json.Marshal(rawobj)
				if err != nil {
					failInitJob("Failed to eval defaults")
					return
				}
				json.Unmarshal(b, &defaults)
			}
		}
		steps := []*model.Step{}
		for _, step := range rqt.Steps {
			st := strings.ToLower(step.Reference.Type)
			inputs := make(map[interface{}]interface{})
			if step.Inputs != nil {
				if tmpinputs, ok := step.Inputs.ToRawObject().(map[interface{}]interface{}); ok {
					inputs = tmpinputs
				} else {
					failInitJob("step.Inputs: not a map")
					return
				}
			}

			env := &yaml.Node{}
			if step.Environment != nil {
				env = step.Environment.ToYamlNode()
			}

			continueOnError := false
			if step.ContinueOnError != nil {
				tmpcontinueOnError, ok := step.ContinueOnError.ToRawObject().(bool)
				if !ok {
					failInitJob("ContinueOnError: act doesn't support expressions here")
					return
				}
				continueOnError = tmpcontinueOnError
			}
			var timeoutMinutes int64 = 0
			if step.TimeoutInMinutes != nil {
				rawTimeout, ok := step.TimeoutInMinutes.ToRawObject().(float64)
				if !ok {
					failInitJob("TimeoutInMinutes: act doesn't support expressions here")
					return
				}
				timeoutMinutes = int64(rawTimeout)
			}
			var displayName string = ""
			if step.DisplayNameToken != nil {
				rawDisplayName, ok := step.DisplayNameToken.ToRawObject().(string)
				if !ok {
					failInitJob("DisplayNameToken: act doesn't support no strings")
					return
				}
				displayName = rawDisplayName
			}
			if step.ContextName == "" {
				step.ContextName = "___" + uuid.New().String()
			}

			switch st {
			case "script":
				rawwd, haswd := inputs["workingDirectory"]
				var wd string
				if haswd {
					tmpwd, ok := rawwd.(string)
					if !ok {
						failInitJob("workingDirectory: act doesn't support non strings")
						return
					}
					wd = tmpwd
				} else {
					wd = ""
				}
				rawshell, hasshell := inputs["shell"]
				shell := ""
				if hasshell {
					sshell, ok := rawshell.(string)
					if ok {
						shell = sshell
					} else {
						failInitJob("shell is not a string")
						return
					}
				}
				scriptContent, ok := inputs["script"].(string)
				if ok {
					steps = append(steps, &model.Step{
						ID:               step.ContextName,
						If:               yaml.Node{Kind: yaml.ScalarNode, Value: step.Condition},
						Name:             displayName,
						Run:              scriptContent,
						WorkingDirectory: wd,
						Shell:            shell,
						ContinueOnError:  continueOnError,
						TimeoutMinutes:   timeoutMinutes,
						Env:              *env,
					})
				} else {
					failInitJob("Missing script")
					return
				}
			case "containerregistry", "repository":
				uses := ""
				if st == "containerregistry" {
					uses = "docker://" + step.Reference.Image
				} else if strings.ToLower(step.Reference.RepositoryType) == "self" {
					uses = step.Reference.Path
				} else {
					uses = step.Reference.Name
					if len(step.Reference.Path) > 0 {
						uses = uses + "/" + step.Reference.Path
					}
					uses = uses + "@" + step.Reference.Ref
				}
				with := map[string]string{}
				for k, v := range inputs {
					k, ok := k.(string)
					if !ok {
						failInitJob("with input key is not a string")
						return
					}
					val, ok := v.(string)
					if !ok {
						failInitJob("with input value is not a string")
						return
					}
					with[k] = val
				}

				steps = append(steps, &model.Step{
					ID:               step.ContextName,
					If:               yaml.Node{Kind: yaml.ScalarNode, Value: step.Condition},
					Name:             displayName,
					Uses:             uses,
					WorkingDirectory: "",
					With:             with,
					ContinueOnError:  continueOnError,
					TimeoutMinutes:   timeoutMinutes,
					Env:              *env,
				})
			}
		}
		actions_step_debug := false
		if sd, ok := rqt.Variables["ACTIONS_STEP_DEBUG"]; ok && (sd.Value == "true" || sd.Value == "1") {
			actions_step_debug = true
		}
		rawContainer := yaml.Node{}
		if rqt.JobContainer != nil {
			rawContainer = *rqt.JobContainer.ToYamlNode()
			if actions_step_debug {
				// Fake step to catch the post debug log
				steps = append(steps, &model.Step{
					ID:               "___finish_job",
					If:               yaml.Node{Kind: yaml.ScalarNode, Value: "false"},
					Name:             "Finish Job",
					Run:              "",
					Env:              yaml.Node{},
					ContinueOnError:  true,
					WorkingDirectory: "",
					Shell:            "",
				})
			}
		}
		services := make(map[string]*model.ContainerSpec)
		if rqt.JobServiceContainers != nil {
			rawServiceContainer, ok := rqt.JobServiceContainers.ToRawObject().(map[interface{}]interface{})
			if !ok {
				failInitJob("Job service container is not nil, but also not a map")
				return
			}
			for name, rawcontainer := range rawServiceContainer {
				containerName, ok := name.(string)
				if !ok {
					failInitJob("containername is not a string")
					return
				}
				spec := &model.ContainerSpec{}
				b, err := json.Marshal(ToStringMap(rawcontainer))
				if err != nil {
					failInitJob("Failed to serialize ContainerSpec")
					return
				}
				err = json.Unmarshal(b, &spec)
				if err != nil {
					failInitJob("Failed to deserialize ContainerSpec")
					return
				}
				services[containerName] = spec
			}
		}
		githubCtxMap, ok := githubCtx.(map[string]interface{})
		if !ok {
			failInitJob("Github ctx is not a map")
			return
		}
		var payload string
		{
			e, _ := json.Marshal(githubCtxMap["event"])
			payload = string(e)
		}
		// Non customizable config
		runnerConfig.Workdir = "./"
		if runtime.GOOS == "windows" {
			runnerConfig.Workdir = ".\\"
		}
		runnerConfig.Platforms = map[string]string{
			"dummy": "-self-hosted",
		}
		runnerConfig.LogOutput = true
		runnerConfig.EventName = githubCtxMap["event_name"].(string)
		// nektos/act
		serverUrl := githubCtxMap["server_url"].(string)
		https := "https://"
		if !strings.HasPrefix(serverUrl, https) {
			failInitJob("")
			return
		}
		runnerConfig.GitHubInstance = serverUrl[len(https):]
		// act fork
		// runnerConfig.GitHubServerUrl = githubCtxMap["server_url"].(string)
		// runnerConfig.GitHubApiServerUrl = githubCtxMap["api_url"].(string)
		// runnerConfig.GitHubGraphQlApiServerUrl = githubCtxMap["graphql_url"].(string)
		// runnerConfig.ForceRemoteCheckout = true // Needed to avoid copy the non exiting working dir
		runnerConfig.AutoRemove = true // Needed to cleanup always cleanup container
		rc := &runner.RunContext{
			Name:   uuid.New().String(),
			Config: runnerConfig,
			Env:    env,
			Run: &model.Run{
				JobID: rqt.JobId,
				Workflow: &model.Workflow{
					Name:     githubCtxMap["workflow"].(string),
					Defaults: defaults,
					Jobs: map[string]*model.Job{
						rqt.JobId: {
							If:           yaml.Node{Value: "always()"},
							Name:         rqt.JobDisplayName,
							RawRunsOn:    yaml.Node{Kind: yaml.ScalarNode, Value: "dummy"},
							Steps:        steps,
							RawContainer: rawContainer,
							Services:     services,
							Outputs:      make(map[string]string),
						},
					},
				},
			},
			Matrix:    matrix,
			EventJSON: payload,
		}

		// Prepare act to provide inputs for workflow_run
		if rawInputsCtx, ok := rqt.ContextData["inputs"]; ok {
			rawInputs := rawInputsCtx.ToRawObject()
			if rawInputsMap, ok := rawInputs.(map[string]interface{}); ok {
				rc.Inputs = rawInputsMap
			}
		}
		// Prepare act to fill previous job outputs
		if rawNeedstx, ok := rqt.ContextData["needs"]; ok {
			needsCtx := rawNeedstx.ToRawObject()
			if needsCtxMap, ok := needsCtx.(map[string]interface{}); ok {
				a := make([]*yaml.Node, 0)
				for k, v := range needsCtxMap {
					a = append(a, &yaml.Node{Kind: yaml.ScalarNode, Style: yaml.DoubleQuotedStyle, Value: k})
					outputs := make(map[string]string)
					result := "success"
					if jobMap, ok := v.(map[string]interface{}); ok {
						if jobOutputs, ok := jobMap["outputs"]; ok {
							if outputMap, ok := jobOutputs.(map[string]interface{}); ok {
								for k, v := range outputMap {
									if sv, ok := v.(string); ok {
										outputs[k] = sv
									}
								}
							}
						}
						if res, ok := jobMap["result"]; ok {
							if resstr, ok := res.(string); ok {
								result = resstr
							}
						}
					}
					rc.Run.Workflow.Jobs[k] = &model.Job{
						Outputs: outputs,
						Result:  result,
					}
				}
				rc.Run.Workflow.Jobs[rqt.JobId].RawNeeds = yaml.Node{Kind: yaml.SequenceNode, Content: a}
			}
		}
		// Prepare act to add job outputs to current job
		if rqt.JobOutputs != nil {
			o := rqt.JobOutputs.ToRawObject()
			if m, ok := o.(map[interface{}]interface{}); ok {
				for k, v := range m {
					if kv, ok := k.(string); ok {
						if sv, ok := v.(string); ok {
							rc.Run.Workflow.Jobs[rqt.JobId].Outputs[kv] = sv
						}
					}
				}
			}
		}

		if name, ok := rqt.Variables["system.github.job"]; ok {
			rc.JobName = name.Value
			// Add the job name to the overlay, otherwise this property is empty
			if githubCtxMap != nil {
				githubCtxMap["job"] = name.Value
			}
		}
		// act fork
		// val, _ := json.Marshal(githubCtx)
		// sv := string(val)
		// rc.GithubContextBase = &sv

		ee := rc.NewExpressionEvaluator()
		rc.ExprEval = ee
		logger := logrus.New()

		formatter := new(ghaFormatter)
		formatter.rc = rc
		formatter.rqt = rqt
		formatter.stepBuffer = &bytes.Buffer{}

		logger.SetFormatter(formatter)
		logger.SetOutput(io.MultiWriter())
		if actions_step_debug {
			logger.SetLevel(logrus.DebugLevel)
			logrus.SetLevel(logrus.DebugLevel)
		} else {
			logger.SetLevel(logrus.InfoLevel)
			logrus.SetLevel(logrus.InfoLevel)
		}
		logrus.SetFormatter(formatter)
		logrus.SetOutput(io.MultiWriter())

		rc.CurrentStep = "__setup"
		rc.StepResults = make(map[string]*model.StepResult)
		rc.StepResults[rc.CurrentStep] = &model.StepResult{}

		for i := 0; i < len(steps); i++ {
			wrap.Value = append(wrap.Value, protocol.CreateTimelineEntry(rqt.JobId, steps[i].ID, steps[i].String()))
			wrap.Value[i+2].Order = int32(i + 2)
		}
		formatter.current = &wrap.Value[1]
		wrap.Count = int64(len(wrap.Value))
		_ = vssConnection.UpdateTimeLine(jobreq.Timeline.Id, jobreq, wrap)
		{
			formatter.updateTimeLine = func() {
				_ = vssConnection.UpdateTimeLine(jobreq.Timeline.Id, jobreq, wrap)
			}
			formatter.uploadLogFile = func(log string) int {
				return vssConnection.UploadLogFile(jobreq.Timeline.Id, jobreq, log)
			}
		}
		var outputMap *map[string]protocol.VariableValue
		jobStatus := "success"
		cancelled := false
		{
			runCtx, cancelRun := context.WithCancel(context.Background())
			logctx, cancelLog := context.WithCancel(context.Background())
			defer func() {
				cancelRun()
				<-logctx.Done()
			}()
			{
				logchan := make(chan *protocol.TimelineRecordFeedLinesWrapper, 64)
				formatter.logline = func(startLine int64, recordId string, lines []string) {
					wrapper := &protocol.TimelineRecordFeedLinesWrapper{}
					wrapper.Value = lines
					wrapper.Count = int64(len(lines))
					wrapper.StartLine = &startLine
					wrapper.StepId = recordId
					logchan <- wrapper
				}
				go func() {
					defer cancelLog()
					sendLog := func(lines *protocol.TimelineRecordFeedLinesWrapper) {
						err := vssConnection.Request("858983e4-19bd-4c5e-864c-507b59b58b12", "5.1-preview", "POST", map[string]string{
							"scopeIdentifier": jobreq.Plan.ScopeIdentifier,
							"planId":          jobreq.Plan.PlanId,
							"hubName":         jobreq.Plan.PlanType,
							"timelineId":      jobreq.Timeline.Id,
							"recordId":        lines.StepId,
						}, map[string]string{}, lines, nil)
						if err != nil {
							fmt.Println("Failed to upload logline: " + err.Error())
						}
					}
					for {
						select {
						case <-runCtx.Done():
							return
						case lines := <-logchan:
							st := time.Now()
							lp := st
							logsexit := false
							for {
								b := false
								div := lp.Sub(st)
								if div > time.Second {
									break
								}
								select {
								case line := <-logchan:
									if line.StepId == lines.StepId {
										lines.Count += line.Count
										lines.Value = append(lines.Value, line.Value...)
									} else {
										sendLog(lines)
										lines = line
										st = time.Now()
									}
								case <-time.After(time.Second - div):
									b = true
								case <-runCtx.Done():
									b = true
									logsexit = true
								}
								if b {
									break
								}
								lp = time.Now()
							}
							sendLog(lines)
							if logsexit {
								return
							}
						}
					}
				}()
			}
			formatter.wrap = wrap

			logger.Log(logrus.InfoLevel, "Runner Name: "+instance.Agent.Name)
			logger.Log(logrus.InfoLevel, "Runner OSDescription: github-act-runner "+runtime.GOOS+"/"+runtime.GOARCH)
			logger.Log(logrus.InfoLevel, "Runner Version: "+version)
			err := rc.Executor()(common.WithJobErrorContainer(common.WithLogger(jobExecCtx, logger)))
			if err != nil {
				logger.Logf(logrus.ErrorLevel, "%v", err.Error())
				jobStatus = "failure"
			}
			// Prepare results for github server
			if rqt.JobOutputs != nil {
				m := make(map[string]protocol.VariableValue)
				outputMap = &m
				for k, v := range rc.Run.Workflow.Jobs[rqt.JobId].Outputs {
					m[k] = protocol.VariableValue{Value: v}
				}
			}

			for _, stepStatus := range rc.StepResults {
				if stepStatus.Conclusion != 0 {
					jobStatus = "failure"
					break
				}
			}
			select {
			case <-jobExecCtx.Done():
				cancelled = true
			default:
			}
			{
				f := formatter
				f.startLine = 1
				if f.current != nil {
					if f.current == &wrap.Value[1] {
						// Workaround check for init failure, e.g. docker fails
						if cancelled {
							f.current.Complete("Canceled")
						} else {
							jobStatus = "failure"
							f.current.Complete("Failed")
						}
					} else if f.rc.StepResults[f.current.RefName].Conclusion == 0 {
						f.current.Complete("Succeeded")
					} else {
						f.current.Complete("Failed")
					}
					if f.stepBuffer.Len() > 0 {
						f.current.Log = &protocol.TaskLogReference{Id: f.uploadLogFile(f.stepBuffer.String())}
					}
				}
			}
			for i := 2; i < len(wrap.Value); i++ {
				if !strings.EqualFold(wrap.Value[i].State, "Completed") {
					wrap.Value[i].Complete("Skipped")
				}
			}
			if cancelled {
				wrap.Value[0].Complete("Canceled")
			} else if jobStatus == "success" {
				wrap.Value[0].Complete("Succeeded")
			} else {
				wrap.Value[0].Complete("Failed")
			}
		}
		for i := 0; ; i++ {
			if vssConnection.UpdateTimeLine(jobreq.Timeline.Id, jobreq, wrap) != nil && i < 10 {
				fmt.Printf("Retry uploading the final timeline of the job in 10 seconds attempt %v of 10\n", i+1)
				<-time.After(time.Second * 10)
			} else {
				break
			}
		}
		result := "Failed"
		if cancelled {
			result = "Canceled"
		} else if jobStatus == "success" {
			result = "Succeeded"
		}
		finishJob2(result, outputMap)
	}()
}

func (config *RemoveRunner) Remove() int {
	c := &http.Client{}
	settings, err := loadConfiguration()
	if err != nil {
		fmt.Printf("settings.json is corrupted: %v, please reconfigure the runner\n", err.Error())
		return 1
	}
	defer func() {
		os.Remove("agent.json")
		os.Remove("auth.json")
		os.Remove("cred.pkcs1")
		WriteJson("settings.json", settings)
	}()
	var instancesToRemove []*RunnerInstance
	for _, i := range settings.Instances {
		if (len(config.Url) == 0 || i.RegistrationUrl == config.Url) || (len(config.Name) == 0 || i.Agent.Name == config.Name) {
			instancesToRemove = append(instancesToRemove, i)
		}
	}
	if len(instancesToRemove) == 0 {
		fmt.Println("Nothing to do, no runner matches")
		return 0
	}
	if !config.Unattended && len(instancesToRemove) > 1 {
		options := make([]string, len(instancesToRemove))
		for i, instance := range instancesToRemove {
			options[i] = fmt.Sprintf("%v ( %v )", instance.Agent.Name, instance.RegistrationUrl)
		}
		result := GetMultiSelectInput("Please select the instances to remove, use --unattended to remove all", options)
		var instancesToRemoveFiltered []*RunnerInstance
		for _, res := range result {
			for i := 0; i < len(options); i++ {
				if options[i] == res {
					instancesToRemoveFiltered = append(instancesToRemoveFiltered, instancesToRemove[i])
				}
			}
		}
		instancesToRemove = instancesToRemoveFiltered
		if len(instancesToRemove) == 0 {
			fmt.Println("Nothing selected, no runner matches")
			return 0
		}
	}
	regurl := ""
	needsPat := false
	for _, i := range instancesToRemove {
		if len(regurl) > 0 && regurl != i.RegistrationUrl {
			needsPat = true
		} else {
			regurl = i.RegistrationUrl
		}
	}
	if needsPat && len(config.Pat) == 0 {
		if !config.Unattended {
			config.Pat = GetInput("Please enter your Personal Access token", "")
		}
		if len(config.Pat) == 0 {
			fmt.Println("You have to provide a Personal access token with access to the repositories to remove or use the --url parameter")
			return 1
		}
	}
	for _, instance := range instancesToRemove {
		result := func() int {
			confremove := config.ConfigureRemoveRunner
			confremove.Url = instance.RegistrationUrl
			res, shouldReturn, returnValue := gitHubAuth(&confremove, c, "remove", "remove-token")
			if shouldReturn {
				return returnValue
			}

			vssConnection := &protocol.VssConnection{
				Client:    c,
				TenantUrl: res.TenantUrl,
				Token:     res.Token,
				PoolId:    instance.PoolId,
				Trace:     config.Trace,
			}
			vssConnection.ConnectionData = vssConnection.GetConnectionData()
			if err := vssConnection.DeleteAgent(instance.Agent); err != nil {
				fmt.Printf("Failed to remove Runner from server: %v\n", err)
				return 1
			}
			return 0
		}()
		if result != 0 && !config.Force {
			return result
		}
		for i := range settings.Instances {
			if settings.Instances[i] == instance {
				settings.Instances[i] = settings.Instances[len(settings.Instances)-1]
				settings.Instances = settings.Instances[:len(settings.Instances)-1]
				break
			}
		}
	}
	fmt.Println("success")
	return 0
}

var version string = "0.2.x-dev"
