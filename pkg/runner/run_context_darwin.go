package runner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/tart"
)

func (rc *RunContext) startTartEnvironment() common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		rawLogger := logger.WithField("raw_output", true)
		logWriter := common.NewLineWriter(rc.commandHandler(ctx), func(s string) bool {
			if rc.Config.LogOutput {
				rawLogger.Infof("%s", s)
			} else {
				rawLogger.Debugf("%s", s)
			}
			return true
		})
		cacheDir := rc.ActionCacheDir()
		randBytes := make([]byte, 8)
		_, _ = rand.Read(randBytes)
		miscpath := filepath.Join(cacheDir, hex.EncodeToString(randBytes))
		actPath := filepath.Join(miscpath, "act")
		if err := os.MkdirAll(actPath, 0o777); err != nil {
			return err
		}
		path := filepath.Join(miscpath, "hostexecutor")
		if err := os.MkdirAll(path, 0o777); err != nil {
			return err
		}
		runnerTmp := filepath.Join(miscpath, "tmp")
		if err := os.MkdirAll(runnerTmp, 0o777); err != nil {
			return err
		}
		toolCache := filepath.Join(cacheDir, "tool_cache")
		rc.JobContainer = &tart.Environment{
			HostEnvironment: container.HostEnvironment{
				Path:      path,
				TmpDir:    runnerTmp,
				ToolCache: toolCache,
				Workdir:   rc.Config.Workdir,
				ActPath:   actPath,
				CleanUp: func() {
					os.RemoveAll(miscpath)
				},
				StdOut: logWriter,
			},
			Config: tart.Config{
				SSHUsername: "admin",
				SSHPassword: "admin",
				Softnet:     false,
				Headless:    true,
				AlwaysPull:  false,
			},
			Env: &tart.Env{
				JobImage: "ghcr.io/cirruslabs/macos-sonoma-base:latest",
				JobID:    "43",
			},
			Miscpath: miscpath,
		}
		rc.cleanUpJobContainer = rc.JobContainer.Remove()
		for k, v := range rc.JobContainer.GetRunnerContext(ctx) {
			if v, ok := v.(string); ok {
				rc.Env[fmt.Sprintf("RUNNER_%s", strings.ToUpper(k))] = v
			}
		}
		// for _, env := range os.Environ() {
		// 	if k, v, ok := strings.Cut(env, "="); ok {
		// 		// don't override
		// 		if _, ok := rc.Env[k]; !ok {
		// 			rc.Env[k] = v
		// 		}
		// 	}
		// }

		return common.NewPipelineExecutor(
			// rc.JobContainer.Remove(),
			rc.JobContainer.Start(false),
			rc.JobContainer.Copy(rc.JobContainer.GetActPath()+"/", &container.FileEntry{
				Name: "workflow/event.json",
				Mode: 0o644,
				Body: rc.EventJSON,
			}, &container.FileEntry{
				Name: "workflow/envs.txt",
				Mode: 0o666,
				Body: "",
			}),
		)(ctx)
	}
}
