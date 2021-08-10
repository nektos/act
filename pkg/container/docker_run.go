package container

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/go-git/go-billy/v5/helper/polyfill"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	specs "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

	"github.com/nektos/act/pkg/common"
)

// NewContainerInput the input for the New function
type NewContainerInput struct {
	Image       string
	Username    string
	Password    string
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
	Privileged  bool
	UsernsMode  string
	Platform    string
}

// FileEntry is a file to copy to a container
type FileEntry struct {
	Name string
	Mode int64
	Body string
}

// Container for managing docker run containers
type Container interface {
	Create(capAdd []string, capDrop []string) common.Executor
	Copy(destPath string, files ...*FileEntry) common.Executor
	CopyDir(destPath string, srcPath string, useGitIgnore bool) common.Executor
	GetContainerArchive(ctx context.Context, srcPath string) (io.ReadCloser, error)
	Pull(forcePull bool) common.Executor
	Start(attach bool) common.Executor
	Exec(command []string, env map[string]string, user, workdir string) common.Executor
	UpdateFromEnv(srcPath string, env *map[string]string) common.Executor
	UpdateFromPath(env *map[string]string) common.Executor
	Remove() common.Executor
}

// NewContainer creates a reference to a container
func NewContainer(input *NewContainerInput) Container {
	cr := new(containerReference)
	cr.input = input
	return cr
}

// supportsContainerImagePlatform returns true if the underlying Docker server
// API version is 1.41 and beyond
func supportsContainerImagePlatform(cli *client.Client) bool {
	ctx := context.TODO()
	logger := common.Logger(ctx)
	ver, err := cli.ServerVersion(ctx)
	if err != nil {
		logger.Panicf("Failed to get Docker API Version: %s", err)
		return false
	}
	sv, err := semver.NewVersion(ver.APIVersion)
	if err != nil {
		logger.Panicf("Failed to unmarshal Docker Version: %s", err)
		return false
	}
	constraint, _ := semver.NewConstraint(">= 1.41")
	return constraint.Check(sv)
}

func (cr *containerReference) Create(capAdd []string, capDrop []string) common.Executor {
	return common.
		NewInfoExecutor("%sdocker create image=%s platform=%s entrypoint=%+q cmd=%+q", logPrefix, cr.input.Image, cr.input.Platform, cr.input.Entrypoint, cr.input.Cmd).
		Then(
			common.NewPipelineExecutor(
				cr.connect(),
				cr.find(),
				cr.create(capAdd, capDrop),
			).IfNot(common.Dryrun),
		)
}

func (cr *containerReference) Start(attach bool) common.Executor {
	return common.
		NewInfoExecutor("%sdocker run image=%s platform=%s entrypoint=%+q cmd=%+q", logPrefix, cr.input.Image, cr.input.Platform, cr.input.Entrypoint, cr.input.Cmd).
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
	return common.
		NewInfoExecutor("%sdocker pull image=%s platform=%s username=%s forcePull=%t", logPrefix, cr.input.Image, cr.input.Platform, cr.input.Username, forcePull).
		Then(
			NewDockerPullExecutor(NewDockerPullExecutorInput{
				Image:     cr.input.Image,
				ForcePull: forcePull,
				Platform:  cr.input.Platform,
				Username:  cr.input.Username,
				Password:  cr.input.Password,
			}),
		)
}

func (cr *containerReference) Copy(destPath string, files ...*FileEntry) common.Executor {
	return common.NewPipelineExecutor(
		cr.connect(),
		cr.find(),
		cr.copyContent(destPath, files...),
	).IfNot(common.Dryrun)
}

func (cr *containerReference) CopyDir(destPath string, srcPath string, useGitIgnore bool) common.Executor {
	return common.NewPipelineExecutor(
		common.NewInfoExecutor("%sdocker cp src=%s dst=%s", logPrefix, srcPath, destPath),
		cr.Exec([]string{"mkdir", "-p", destPath}, nil, "", ""),
		cr.copyDir(destPath, srcPath, useGitIgnore),
	).IfNot(common.Dryrun)
}

func (cr *containerReference) GetContainerArchive(ctx context.Context, srcPath string) (io.ReadCloser, error) {
	a, _, err := cr.cli.CopyFromContainer(ctx, cr.id, srcPath)
	return a, err
}

func (cr *containerReference) UpdateFromEnv(srcPath string, env *map[string]string) common.Executor {
	return cr.extractEnv(srcPath, env).IfNot(common.Dryrun)
}

func (cr *containerReference) UpdateFromPath(env *map[string]string) common.Executor {
	return cr.extractPath(env).IfNot(common.Dryrun)
}

func (cr *containerReference) Exec(command []string, env map[string]string, user, workdir string) common.Executor {
	return common.NewPipelineExecutor(
		common.NewInfoExecutor("%sdocker exec cmd=[%s] user=%s workdir=%s", logPrefix, strings.Join(command, " "), user, workdir),
		cr.connect(),
		cr.find(),
		cr.exec(command, env, user, workdir),
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

func GetDockerClient(ctx context.Context) (*client.Client, error) {
	var err error
	var cli *client.Client

	// TODO: this should maybe need to be a global option, not hidden in here?
	//       though i'm not sure how that works out when there's another Executor :D
	//		 I really would like something that works on OSX native for eg
	dockerHost := os.Getenv("DOCKER_HOST")

	if strings.HasPrefix(dockerHost, "ssh://") {
		var helper *connhelper.ConnectionHelper

		helper, err = connhelper.GetConnectionHelper(dockerHost)
		if err != nil {
			return nil, err
		}
		cli, err = client.NewClientWithOpts(
			client.WithHost(helper.Host),
			client.WithDialContext(helper.Dialer),
		)
	} else {
		cli, err = client.NewClientWithOpts(client.FromEnv)
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}
	cli.NegotiateAPIVersion(ctx)

	return cli, err
}

func (cr *containerReference) connect() common.Executor {
	return func(ctx context.Context) error {
		if cr.cli != nil {
			return nil
		}
		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}
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

		for _, c := range containers {
			for _, name := range c.Names {
				if name[1:] == cr.input.Name {
					cr.id = c.ID
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

func (cr *containerReference) create(capAdd []string, capDrop []string) common.Executor {
	return func(ctx context.Context) error {
		if cr.id != "" {
			return nil
		}
		logger := common.Logger(ctx)
		isTerminal := term.IsTerminal(int(os.Stdout.Fd()))

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

		var platSpecs *specs.Platform
		if supportsContainerImagePlatform(cr.cli) && cr.input.Platform != "" {
			desiredPlatform := strings.SplitN(cr.input.Platform, `/`, 2)

			if len(desiredPlatform) != 2 {
				logger.Panicf("Incorrect container platform option. %s is not a valid platform.", cr.input.Platform)
			}

			platSpecs = &specs.Platform{
				Architecture: desiredPlatform[1],
				OS:           desiredPlatform[0],
			}
		}
		resp, err := cr.cli.ContainerCreate(ctx, config, &container.HostConfig{
			CapAdd:      capAdd,
			CapDrop:     capDrop,
			Binds:       input.Binds,
			Mounts:      mounts,
			NetworkMode: container.NetworkMode(input.NetworkMode),
			Privileged:  input.Privileged,
			UsernsMode:  container.UsernsMode(input.UsernsMode),
		}, nil, platSpecs, input.Name)
		if err != nil {
			return errors.WithStack(err)
		}
		logger.Debugf("Created container name=%s id=%v from image %v (platform: %s)", input.Name, resp.ID, input.Image, input.Platform)
		logger.Debugf("ENV ==> %v", input.Env)

		cr.id = resp.ID
		return nil
	}
}

var singleLineEnvPattern, mulitiLineEnvPattern *regexp.Regexp

func (cr *containerReference) extractEnv(srcPath string, env *map[string]string) common.Executor {
	if singleLineEnvPattern == nil {
		singleLineEnvPattern = regexp.MustCompile("^([^=]+)=([^=]+)$")
		mulitiLineEnvPattern = regexp.MustCompile(`^([^<]+)<<(\w+)$`)
	}

	localEnv := *env
	return func(ctx context.Context) error {
		envTar, _, err := cr.cli.CopyFromContainer(ctx, cr.id, srcPath)
		if err != nil {
			return nil
		}
		defer envTar.Close()
		reader := tar.NewReader(envTar)
		_, err = reader.Next()
		if err != nil && err != io.EOF {
			return errors.WithStack(err)
		}
		s := bufio.NewScanner(reader)
		multiLineEnvKey := ""
		multiLineEnvDelimiter := ""
		multiLineEnvContent := ""
		for s.Scan() {
			line := s.Text()
			if singleLineEnv := singleLineEnvPattern.FindStringSubmatch(line); singleLineEnv != nil {
				localEnv[singleLineEnv[1]] = singleLineEnv[2]
			}
			if line == multiLineEnvDelimiter {
				localEnv[multiLineEnvKey] = multiLineEnvContent
				multiLineEnvKey, multiLineEnvDelimiter, multiLineEnvContent = "", "", ""
			}
			if multiLineEnvKey != "" && multiLineEnvDelimiter != "" {
				if multiLineEnvContent != "" {
					multiLineEnvContent += "\n"
				}
				multiLineEnvContent += line
			}
			if mulitiLineEnvStart := mulitiLineEnvPattern.FindStringSubmatch(line); mulitiLineEnvStart != nil {
				multiLineEnvKey = mulitiLineEnvStart[1]
				multiLineEnvDelimiter = mulitiLineEnvStart[2]
			}
		}
		env = &localEnv
		return nil
	}
}

func (cr *containerReference) extractPath(env *map[string]string) common.Executor {
	localEnv := *env
	return func(ctx context.Context) error {
		pathTar, _, err := cr.cli.CopyFromContainer(ctx, cr.id, localEnv["GITHUB_PATH"])
		if err != nil {
			return errors.WithStack(err)
		}
		defer pathTar.Close()

		reader := tar.NewReader(pathTar)
		_, err = reader.Next()
		if err != nil && err != io.EOF {
			return errors.WithStack(err)
		}
		s := bufio.NewScanner(reader)
		for s.Scan() {
			line := s.Text()
			localEnv["PATH"] = fmt.Sprintf("%s:%s", line, localEnv["PATH"])
		}

		env = &localEnv
		return nil
	}
}

func (cr *containerReference) exec(cmd []string, env map[string]string, user, workdir string) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		// Fix slashes when running on Windows
		if runtime.GOOS == "windows" {
			var newCmd []string
			for _, v := range cmd {
				newCmd = append(newCmd, strings.ReplaceAll(v, `\`, `/`))
			}
			cmd = newCmd
		}

		logger.Debugf("Exec command '%s'", cmd)
		isTerminal := term.IsTerminal(int(os.Stdout.Fd()))
		envList := make([]string, 0)
		for k, v := range env {
			envList = append(envList, fmt.Sprintf("%s=%s", k, v))
		}

		var wd string
		if workdir != "" {
			if strings.HasPrefix(workdir, "/") {
				wd = workdir
			} else {
				wd = fmt.Sprintf("%s/%s", cr.input.WorkingDir, workdir)
			}
		} else {
			wd = cr.input.WorkingDir
		}
		logger.Debugf("Working directory '%s'", wd)

		idResp, err := cr.cli.ContainerExecCreate(ctx, cr.id, types.ExecConfig{
			User:         user,
			Cmd:          cmd,
			WorkingDir:   wd,
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
		defer resp.Close()

		var outWriter io.Writer
		outWriter = cr.input.Stdout
		if outWriter == nil {
			outWriter = os.Stdout
		}
		errWriter := cr.input.Stderr
		if errWriter == nil {
			errWriter = os.Stderr
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
func (cr *containerReference) copyDir(dstPath string, srcPath string, useGitIgnore bool) common.Executor {
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

		var ignorer gitignore.Matcher
		if useGitIgnore {
			ps, err := gitignore.ReadPatterns(polyfill.New(osfs.New(srcPath)), nil)
			if err != nil {
				log.Debugf("Error loading .gitignore: %v", err)
			}

			ignorer = gitignore.NewMatcher(ps)
		}

		err = filepath.Walk(srcPath, func(file string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			sansPrefix := strings.TrimPrefix(file, srcPrefix)
			split := strings.Split(sansPrefix, string(filepath.Separator))
			if ignorer != nil && ignorer.Match(split, fi.IsDir()) {
				if fi.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// return on non-regular files (thanks to [kumo](https://medium.com/@komuw/just-like-you-did-fbdd7df829d3) for this suggested update)
			linkName := fi.Name()
			if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
				linkName, err = os.Readlink(file)
				if err != nil {
					return errors.WithMessagef(err, "unable to readlink %s", file)
				}
			} else if !fi.Mode().IsRegular() {
				return nil
			}

			// create a new dir/file header
			header, err := tar.FileInfoHeader(fi, linkName)
			if err != nil {
				return err
			}

			// update the name to correctly reflect the desired destination when untaring
			header.Name = filepath.ToSlash(sansPrefix)
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
				if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
					// symlinks don't need to be copied, ignore this error
					err = nil
				}
				return err
			}

			// manually close here after each file operation; deferring would cause each file close
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
		isTerminal := term.IsTerminal(int(os.Stdout.Fd()))

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
