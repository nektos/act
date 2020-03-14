package container

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	gitignore "github.com/monochromegane/go-gitignore"
	"github.com/nektos/act/pkg/common"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

// NewContainerInput the input for the New function
type NewContainerInput struct {
	Image       string
	Entrypoint  []string
	Cmd         []string
	WorkingDir  string
	Env         []string
	Binds       []string
	Mounts      map[string]string
	Name        string
	Stdout      io.Writer
	Stderr      io.Writer
	NetworkMode string
}

// FileEntry is a file to copy to a container
type FileEntry struct {
	Name string
	Mode int64
	Body string
}

// Container for managing docker run containers
type Container interface {
	Create() common.Executor
	Copy(destPath string, files ...*FileEntry) common.Executor
	CopyDir(destPath string, srcPath string) common.Executor
	Pull(forcePull bool) common.Executor
	Start(attach bool) common.Executor
	Exec(command []string, env map[string]string) common.Executor
	Remove() common.Executor
}

// NewContainer creates a reference to a container
func NewContainer(input *NewContainerInput) Container {
	cr := new(containerReference)
	cr.input = input
	return cr
}

func (cr *containerReference) Create() common.Executor {
	return common.
		NewDebugExecutor("%sdocker create image=%s entrypoint=%+q cmd=%+q", logPrefix, cr.input.Image, cr.input.Entrypoint, cr.input.Cmd).
		Then(
			common.NewPipelineExecutor(
				cr.connect(),
				cr.find(),
				cr.create(),
			).IfNot(common.Dryrun),
		)
}
func (cr *containerReference) Start(attach bool) common.Executor {
	return common.
		NewInfoExecutor("%sdocker run image=%s entrypoint=%+q cmd=%+q", logPrefix, cr.input.Image, cr.input.Entrypoint, cr.input.Cmd).
		Then(
			common.NewPipelineExecutor(
				cr.connect(),
				cr.find(),
				cr.attach().IfBool(attach),
				cr.start(),
				cr.wait().IfBool(attach),
			).IfNot(common.Dryrun),
		)
}
func (cr *containerReference) Pull(forcePull bool) common.Executor {
	return NewDockerPullExecutor(NewDockerPullExecutorInput{
		Image:     cr.input.Image,
		ForcePull: forcePull,
	})
}
func (cr *containerReference) Copy(destPath string, files ...*FileEntry) common.Executor {
	return common.NewPipelineExecutor(
		cr.connect(),
		cr.find(),
		cr.copyContent(destPath, files...),
	).IfNot(common.Dryrun)
}

func (cr *containerReference) CopyDir(destPath string, srcPath string) common.Executor {
	return common.NewPipelineExecutor(
		common.NewInfoExecutor("%sdocker cp src=%s dst=%s", logPrefix, srcPath, destPath),
		cr.connect(),
		cr.find(),
		cr.exec([]string{"mkdir", "-p", destPath}, nil),
		cr.copyDir(destPath, srcPath),
	).IfNot(common.Dryrun)
}

func (cr *containerReference) Exec(command []string, env map[string]string) common.Executor {

	return common.NewPipelineExecutor(
		cr.connect(),
		cr.find(),
		cr.exec(command, env),
	).IfNot(common.Dryrun)
}
func (cr *containerReference) Remove() common.Executor {
	return common.NewPipelineExecutor(
		cr.connect(),
		cr.find(),
	).Finally(
		cr.remove(),
	).IfNot(common.Dryrun)
}

type containerReference struct {
	cli   *client.Client
	id    string
	input *NewContainerInput
}

func (cr *containerReference) connect() common.Executor {
	return func(ctx context.Context) error {
		if cr.cli != nil {
			return nil
		}
		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return errors.WithStack(err)
		}
		cli.NegotiateAPIVersion(ctx)
		cr.cli = cli
		return nil
	}
}

func (cr *containerReference) find() common.Executor {
	return func(ctx context.Context) error {
		if cr.id != "" {
			return nil
		}
		containers, err := cr.cli.ContainerList(ctx, types.ContainerListOptions{
			All: true,
		})
		if err != nil {
			return errors.WithStack(err)
		}

		for _, container := range containers {
			for _, name := range container.Names {
				if name[1:] == cr.input.Name {
					cr.id = container.ID
					return nil
				}
			}
		}

		cr.id = ""
		return nil
	}
}

func (cr *containerReference) remove() common.Executor {
	return func(ctx context.Context) error {
		if cr.id == "" {
			return nil
		}

		logger := common.Logger(ctx)
		err := cr.cli.ContainerRemove(context.Background(), cr.id, types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		})
		if err != nil {
			logger.Error(errors.WithStack(err))
		}

		logger.Debugf("Removed container: %v", cr.id)
		cr.id = ""
		return nil
	}
}

func (cr *containerReference) create() common.Executor {
	return func(ctx context.Context) error {
		if cr.id != "" {
			return nil
		}
		logger := common.Logger(ctx)
		isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))

		input := cr.input
		config := &container.Config{
			Image:      input.Image,
			Cmd:        input.Cmd,
			Entrypoint: input.Entrypoint,
			WorkingDir: input.WorkingDir,
			Env:        input.Env,
			Tty:        isTerminal,
		}

		mounts := make([]mount.Mount, 0)
		for mountSource, mountTarget := range input.Mounts {
			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeVolume,
				Source: mountSource,
				Target: mountTarget,
			})
		}

		resp, err := cr.cli.ContainerCreate(ctx, config, &container.HostConfig{
			Binds:       input.Binds,
			Mounts:      mounts,
			NetworkMode: container.NetworkMode(input.NetworkMode),
		}, nil, input.Name)
		if err != nil {
			return errors.WithStack(err)
		}
		logger.Debugf("Created container name=%s id=%v from image %v", input.Name, resp.ID, input.Image)
		logger.Debugf("ENV ==> %v", input.Env)

		cr.id = resp.ID
		return nil
	}
}

func (cr *containerReference) exec(cmd []string, env map[string]string) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		logger.Debugf("Exec command '%s'", cmd)
		isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))
		envList := make([]string, 0)
		for k, v := range env {
			envList = append(envList, fmt.Sprintf("%s=%s", k, v))
		}

		idResp, err := cr.cli.ContainerExecCreate(ctx, cr.id, types.ExecConfig{
			Cmd:          cmd,
			WorkingDir:   cr.input.WorkingDir,
			Env:          envList,
			Tty:          isTerminal,
			AttachStderr: true,
			AttachStdout: true,
		})
		if err != nil {
			return errors.WithStack(err)
		}

		resp, err := cr.cli.ContainerExecAttach(ctx, idResp.ID, types.ExecStartCheck{
			Tty: isTerminal,
		})
		if err != nil {
			return errors.WithStack(err)
		}
		var outWriter io.Writer
		outWriter = cr.input.Stdout
		if outWriter == nil {
			outWriter = os.Stdout
		}
		errWriter := cr.input.Stderr
		if errWriter == nil {
			errWriter = os.Stderr
		}

		err = cr.cli.ContainerExecStart(ctx, idResp.ID, types.ExecStartCheck{
			Tty: isTerminal,
		})
		if err != nil {
			return errors.WithStack(err)
		}

		if !isTerminal || os.Getenv("NORAW") != "" {
			_, err = stdcopy.StdCopy(outWriter, errWriter, resp.Reader)
		} else {
			_, err = io.Copy(outWriter, resp.Reader)
		}
		if err != nil {
			logger.Error(err)
		}

		inspectResp, err := cr.cli.ContainerExecInspect(ctx, idResp.ID)
		if err != nil {
			return errors.WithStack(err)
		}

		if inspectResp.ExitCode == 0 {
			return nil
		}

		return fmt.Errorf("exit with `FAILURE`: %v", inspectResp.ExitCode)
	}
}

// nolint: gocyclo
func (cr *containerReference) copyDir(dstPath string, srcPath string) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		tarFile, err := ioutil.TempFile("", "act")
		if err != nil {
			return err
		}
		log.Debugf("Writing tarball %s from %s", tarFile.Name(), srcPath)
		defer tarFile.Close()
		defer os.Remove(tarFile.Name())
		tw := tar.NewWriter(tarFile)

		srcPrefix := filepath.Dir(srcPath)
		if !strings.HasSuffix(srcPrefix, string(filepath.Separator)) {
			srcPrefix += string(filepath.Separator)
		}
		log.Debugf("Stripping prefix:%s src:%s", srcPrefix, srcPath)

		gitignore, err := gitignore.NewGitIgnore(filepath.Join(srcPath, ".gitignore"))
		if err != nil {
			log.Debugf("Error loading .gitignore: %v", err)
		}

		err = filepath.Walk(srcPath, func(file string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// return on non-regular files (thanks to [kumo](https://medium.com/@komuw/just-like-you-did-fbdd7df829d3) for this suggested update)
			if !fi.Mode().IsRegular() {
				return nil
			}

			if gitignore != nil && gitignore.Match(file, fi.IsDir()) {
				return nil
			}

			// create a new dir/file header
			header, err := tar.FileInfoHeader(fi, fi.Name())
			if err != nil {
				return err
			}

			// update the name to correctly reflect the desired destination when untaring
			header.Name = strings.TrimPrefix(file, srcPrefix)
			header.Mode = int64(fi.Mode())
			header.ModTime = fi.ModTime()

			// write the header
			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			// open files for taring
			f, err := os.Open(file)
			if err != nil {
				return err
			}

			// copy file data into tar writer
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()

			return nil
		})
		if err != nil {
			return err
		}
		if err := tw.Close(); err != nil {
			return err
		}

		logger.Debugf("Extracting content from '%s' to '%s'", tarFile.Name(), dstPath)
		_, err = tarFile.Seek(0, 0)
		if err != nil {
			return errors.WithStack(err)
		}
		err = cr.cli.CopyToContainer(ctx, cr.id, dstPath, tarFile, types.CopyToContainerOptions{})
		if err != nil {
			return errors.WithStack(err)
		}
		return nil
	}
}

func (cr *containerReference) copyContent(dstPath string, files ...*FileEntry) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		for _, file := range files {
			log.Debugf("Writing entry to tarball %s len:%d", file.Name, len(file.Body))
			hdr := &tar.Header{
				Name: file.Name,
				Mode: file.Mode,
				Size: int64(len(file.Body)),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if _, err := tw.Write([]byte(file.Body)); err != nil {
				return err
			}
		}
		if err := tw.Close(); err != nil {
			return err
		}

		logger.Debugf("Extracting content to '%s'", dstPath)
		err := cr.cli.CopyToContainer(ctx, cr.id, dstPath, &buf, types.CopyToContainerOptions{})
		if err != nil {
			return errors.WithStack(err)
		}
		return nil
	}
}

func (cr *containerReference) attach() common.Executor {
	return func(ctx context.Context) error {
		out, err := cr.cli.ContainerAttach(ctx, cr.id, types.ContainerAttachOptions{
			Stream: true,
			Stdout: true,
			Stderr: true,
		})
		if err != nil {
			return errors.WithStack(err)
		}
		isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))

		var outWriter io.Writer
		outWriter = cr.input.Stdout
		if outWriter == nil {
			outWriter = os.Stdout
		}
		errWriter := cr.input.Stderr
		if errWriter == nil {
			errWriter = os.Stderr
		}
		go func() {
			if !isTerminal || os.Getenv("NORAW") != "" {
				_, err = stdcopy.StdCopy(outWriter, errWriter, out.Reader)
			} else {
				_, err = io.Copy(outWriter, out.Reader)
			}
			if err != nil {
				common.Logger(ctx).Error(err)
			}
		}()
		return nil
	}
}

func (cr *containerReference) start() common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		logger.Debugf("Starting container: %v", cr.id)

		if err := cr.cli.ContainerStart(ctx, cr.id, types.ContainerStartOptions{}); err != nil {
			return errors.WithStack(err)
		}

		logger.Debugf("Started container: %v", cr.id)
		return nil
	}
}

func (cr *containerReference) wait() common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		statusCh, errCh := cr.cli.ContainerWait(ctx, cr.id, container.WaitConditionNotRunning)
		var statusCode int64
		select {
		case err := <-errCh:
			if err != nil {
				return errors.WithStack(err)
			}
		case status := <-statusCh:
			statusCode = status.StatusCode
		}

		logger.Debugf("Return status: %v", statusCode)

		if statusCode == 0 {
			return nil
		}

		return fmt.Errorf("exit with `FAILURE`: %v", statusCode)
	}
}
