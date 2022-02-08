package runner

import (
	"embed"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
)

type ActionReader interface {
	readAction(step *model.Step, actionDir string, actionPath string, readFile actionyamlReader) (*model.Action, error)
}

type actionyamlReader func(filename string) (io.Reader, io.Closer, error)
type fileWriter func(filename string, data []byte, perm fs.FileMode) error

//go:embed res/trampoline.js
var trampoline embed.FS

func (sc *StepContext) readAction(step *model.Step, actionDir string, actionPath string, readFile actionyamlReader, writeFile fileWriter) (*model.Action, error) {
	reader, closer, err := readFile("action.yml")
	if os.IsNotExist(err) {
		reader, closer, err = readFile("action.yaml")
		if err != nil {
			if _, closer, err2 := readFile("Dockerfile"); err2 == nil {
				closer.Close()
				action := &model.Action{
					Name: "(Synthetic)",
					Runs: model.ActionRuns{
						Using: "docker",
						Image: "Dockerfile",
					},
				}
				log.Debugf("Using synthetic action %v for Dockerfile", action)
				return action, nil
			}
			if step.With != nil {
				if val, ok := step.With["args"]; ok {
					var b []byte
					if b, err = trampoline.ReadFile("res/trampoline.js"); err != nil {
						return nil, err
					}
					err2 := writeFile(filepath.Join(actionDir, actionPath, "trampoline.js"), b, 0400)
					if err2 != nil {
						return nil, err2
					}
					action := &model.Action{
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
					log.Debugf("Using synthetic action %v", action)
					return action, nil
				}
			}
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	defer closer.Close()

	action, err := model.ReadAction(reader)
	log.Debugf("Read action %v from '%s'", action, "Unknown")
	return action, err
}
