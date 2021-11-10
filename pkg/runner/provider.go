package runner

import (
	"archive/tar"
	"context"
	"embed"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	// Go told me to?
	"path"

	log "github.com/sirupsen/logrus"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

type ActionProvider interface {
	SetupAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor
	RunAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor
	ExecuteNode12Action(sc *StepContext, containerActionDir string, ctx context.Context, maybeCopyToActionDir func() error) error
	ExecuteNode12PostAction(sc *StepContext, containerActionDir string, ctx context.Context) error
}

type actProvider struct{}

func (a *actProvider) ExecuteNode12Action(sc *StepContext, containerActionDir string, ctx context.Context, maybeCopyToActionDir func() error) error {
	return nil
}

func (a *actProvider) ExecuteNode12PostAction(sc *StepContext, containerActionDir string, ctx context.Context) error {
	return nil
}

//go:embed res/trampoline.js
var trampoline embed.FS

func (a *actProvider) SetupAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor {
	return func(ctx context.Context) error {
		var readFile func(filename string) (io.Reader, io.Closer, error)
		if localAction {
			_, cpath := sc.getContainerActionPaths(sc.Step, path.Join(actionDir, actionPath), sc.RunContext)
			readFile = func(filename string) (io.Reader, io.Closer, error) {
				tars, err := sc.RunContext.JobContainer.GetContainerArchive(ctx, path.Join(cpath, filename))
				if err != nil {
					return nil, nil, os.ErrNotExist
				}
				treader := tar.NewReader(tars)
				if _, err := treader.Next(); err != nil {
					return nil, nil, os.ErrNotExist
				}
				return treader, tars, nil
			}
		} else {
			readFile = func(filename string) (io.Reader, io.Closer, error) {
				f, err := os.Open(filepath.Join(actionDir, actionPath, filename))
				return f, f, err
			}
		}

		reader, closer, err := readFile("action.yml")
		if os.IsNotExist(err) {
			reader, closer, err = readFile("action.yaml")
			if err != nil {
				if _, closer, err2 := readFile("Dockerfile"); err2 == nil {
					closer.Close()
					sc.Action = &model.Action{
						Name: "(Synthetic)",
						Runs: model.ActionRuns{
							Using: "docker",
							Image: "Dockerfile",
						},
					}
					log.Debugf("Using synthetic action %v for Dockerfile", sc.Action)
					return nil
				}
				if sc.Step.With != nil {
					if val, ok := sc.Step.With["args"]; ok {
						var b []byte
						if b, err = trampoline.ReadFile("res/trampoline.js"); err != nil {
							return err
						}
						err2 := ioutil.WriteFile(filepath.Join(actionDir, actionPath, "trampoline.js"), b, 0400)
						if err2 != nil {
							return err
						}
						sc.Action = &model.Action{
							Name: "(Synthetic)",
							Inputs: map[string]model.Input{
								"cwd": {
									Description: "(Actual working directory)",
									Required:    false,
									Default:     filepath.Join(actionDir, actionPath),
								},
								"command": {
									Description: "(Actual program)",
									Required:    false,
									Default:     val,
								},
							},
							Runs: model.ActionRuns{
								Using: "node12",
								Main:  "trampoline.js",
							},
						}
						log.Debugf("Using synthetic action %v", sc.Action)
						return nil
					}
				}
				return err
			}
		} else if err != nil {
			return err
		}
		defer closer.Close()

		sc.Action, err = model.ReadAction(reader)
		log.Debugf("Read action %v from '%s'", sc.Action, "Unknown")
		return err
	}
}

func (a *actProvider) RunAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func NewActionProvider() ActionProvider {
	return &actProvider{}
}
