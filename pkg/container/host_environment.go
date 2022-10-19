package container

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"errors"

	"github.com/go-git/go-billy/v5/helper/polyfill"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/lookpath"
	"golang.org/x/term"
)

type HostEnvironment struct {
	Path      string
	TmpDir    string
	ToolCache string
	Workdir   string
	ActPath   string
	CleanUp   func()
	StdOut    io.Writer
}

func (e *HostEnvironment) Create(capAdd []string, capDrop []string) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func (e *HostEnvironment) Close() common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func (e *HostEnvironment) Copy(destPath string, files ...*FileEntry) common.Executor {
	return func(ctx context.Context) error {
		for _, f := range files {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(destPath, f.Name)), 0777); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(destPath, f.Name), []byte(f.Body), fs.FileMode(f.Mode)); err != nil {
				return err
			}
		}
		return nil
	}
}

func (e *HostEnvironment) CopyDir(destPath string, srcPath string, useGitIgnore bool) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		srcPrefix := filepath.Dir(srcPath)
		if !strings.HasSuffix(srcPrefix, string(filepath.Separator)) {
			srcPrefix += string(filepath.Separator)
		}
		logger.Debugf("Stripping prefix:%s src:%s", srcPrefix, srcPath)
		var ignorer gitignore.Matcher
		if useGitIgnore {
			ps, err := gitignore.ReadPatterns(polyfill.New(osfs.New(srcPath)), nil)
			if err != nil {
				logger.Debugf("Error loading .gitignore: %v", err)
			}

			ignorer = gitignore.NewMatcher(ps)
		}
		fc := &fileCollector{
			Fs:        &defaultFs{},
			Ignorer:   ignorer,
			SrcPath:   srcPath,
			SrcPrefix: srcPrefix,
			Handler: &copyCollector{
				DstDir: destPath,
			},
		}
		return filepath.Walk(srcPath, fc.collectFiles(ctx, []string{}))
	}
}

func (e *HostEnvironment) GetContainerArchive(ctx context.Context, srcPath string) (io.ReadCloser, error) {
	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	defer tw.Close()
	srcPath = filepath.Clean(srcPath)
	fi, err := os.Lstat(srcPath)
	if err != nil {
		return nil, err
	}
	tc := &tarCollector{
		TarWriter: tw,
	}
	if fi.IsDir() {
		srcPrefix := filepath.Dir(srcPath)
		if !strings.HasSuffix(srcPrefix, string(filepath.Separator)) {
			srcPrefix += string(filepath.Separator)
		}
		fc := &fileCollector{
			Fs:        &defaultFs{},
			SrcPath:   srcPath,
			SrcPrefix: srcPrefix,
			Handler:   tc,
		}
		err = filepath.Walk(srcPath, fc.collectFiles(ctx, []string{}))
		if err != nil {
			return nil, err
		}
	} else {
		var f io.ReadCloser
		var linkname string
		if fi.Mode()&fs.ModeSymlink != 0 {
			linkname, err = os.Readlink(srcPath)
			if err != nil {
				return nil, err
			}
		} else {
			f, err = os.Open(srcPath)
			if err != nil {
				return nil, err
			}
			defer f.Close()
		}
		err := tc.WriteFile(fi.Name(), fi, linkname, f)
		if err != nil {
			return nil, err
		}
	}
	return io.NopCloser(buf), nil
}

func (e *HostEnvironment) Pull(forcePull bool) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func (e *HostEnvironment) Start(attach bool) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

type ptyWriter struct {
	Out       io.Writer
	AutoStop  bool
	dirtyLine bool
}

func (w *ptyWriter) Write(buf []byte) (int, error) {
	if w.AutoStop && len(buf) > 0 && buf[len(buf)-1] == 4 {
		n, err := w.Out.Write(buf[:len(buf)-1])
		if err != nil {
			return n, err
		}
		if w.dirtyLine || len(buf) > 1 && buf[len(buf)-2] != '\n' {
			_, _ = w.Out.Write([]byte("\n"))
			return n, io.EOF
		}
		return n, io.EOF
	}
	w.dirtyLine = strings.LastIndex(string(buf), "\n") < len(buf)-1
	return w.Out.Write(buf)
}

type localEnv struct {
	env map[string]string
}

func (l *localEnv) Getenv(name string) string {
	if runtime.GOOS == "windows" {
		for k, v := range l.env {
			if strings.EqualFold(name, k) {
				return v
			}
		}
		return ""
	}
	return l.env[name]
}

func lookupPathHost(cmd string, env map[string]string, writer io.Writer) (string, error) {
	f, err := lookpath.LookPath2(cmd, &localEnv{env: env})
	if err != nil {
		err := "Cannot find: " + fmt.Sprint(cmd) + " in PATH"
		if _, _err := writer.Write([]byte(err + "\n")); _err != nil {
			return "", fmt.Errorf("%v: %w", err, _err)
		}
		return "", errors.New(err)
	}
	return f, nil
}

func setupPty(cmd *exec.Cmd, cmdline string) (*os.File, *os.File, error) {
	ppty, tty, err := openPty()
	if err != nil {
		return nil, nil, err
	}
	if term.IsTerminal(int(tty.Fd())) {
		_, err := term.MakeRaw(int(tty.Fd()))
		if err != nil {
			ppty.Close()
			tty.Close()
			return nil, nil, err
		}
	}
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.SysProcAttr = getSysProcAttr(cmdline, true)
	return ppty, tty, nil
}

func writeKeepAlive(ppty io.Writer) {
	c := 1
	var err error
	for c == 1 && err == nil {
		c, err = ppty.Write([]byte{4})
		<-time.After(time.Second)
	}
}

func copyPtyOutput(writer io.Writer, ppty io.Reader, finishLog context.CancelFunc) {
	defer func() {
		finishLog()
	}()
	if _, err := io.Copy(writer, ppty); err != nil {
		return
	}
}

func (e *HostEnvironment) UpdateFromImageEnv(env *map[string]string) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func getEnvListFromMap(env map[string]string) []string {
	envList := make([]string, 0)
	for k, v := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	return envList
}

func (e *HostEnvironment) exec(ctx context.Context, command []string, cmdline string, env map[string]string, user, workdir string) error {
	envList := getEnvListFromMap(env)
	var wd string
	if workdir != "" {
		if filepath.IsAbs(workdir) {
			wd = workdir
		} else {
			wd = filepath.Join(e.Path, workdir)
		}
	} else {
		wd = e.Path
	}
	f, err := lookupPathHost(command[0], env, e.StdOut)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, f)
	cmd.Path = f
	cmd.Args = command
	cmd.Stdin = nil
	cmd.Stdout = e.StdOut
	cmd.Env = envList
	cmd.Stderr = e.StdOut
	cmd.Dir = wd
	cmd.SysProcAttr = getSysProcAttr(cmdline, false)
	var ppty *os.File
	var tty *os.File
	defer func() {
		if ppty != nil {
			ppty.Close()
		}
		if tty != nil {
			tty.Close()
		}
	}()
	if true /* allocate Terminal */ {
		var err error
		ppty, tty, err = setupPty(cmd, cmdline)
		if err != nil {
			common.Logger(ctx).Debugf("Failed to setup Pty %v\n", err.Error())
		}
	}
	writer := &ptyWriter{Out: e.StdOut}
	logctx, finishLog := context.WithCancel(context.Background())
	if ppty != nil {
		go copyPtyOutput(writer, ppty, finishLog)
	} else {
		finishLog()
	}
	if ppty != nil {
		go writeKeepAlive(ppty)
	}
	err = cmd.Run()
	if err != nil {
		return err
	}
	if tty != nil {
		writer.AutoStop = true
		if _, err := tty.Write([]byte("\x04")); err != nil {
			common.Logger(ctx).Debug("Failed to write EOT")
		}
	}
	<-logctx.Done()

	if ppty != nil {
		ppty.Close()
		ppty = nil
	}
	return err
}

func (e *HostEnvironment) Exec(command []string /*cmdline string, */, env map[string]string, user, workdir string) common.Executor {
	return func(ctx context.Context) error {
		if err := e.exec(ctx, command, "" /*cmdline*/, env, user, workdir); err != nil {
			select {
			case <-ctx.Done():
				return fmt.Errorf("this step has been cancelled: %w", err)
			default:
				return err
			}
		}
		return nil
	}
}

func (e *HostEnvironment) UpdateFromEnv(srcPath string, env *map[string]string) common.Executor {
	localEnv := *env
	return func(ctx context.Context) error {
		envTar, err := e.GetContainerArchive(ctx, srcPath)
		if err != nil {
			return nil
		}
		defer envTar.Close()
		reader := tar.NewReader(envTar)
		_, err = reader.Next()
		if err != nil && err != io.EOF {
			return err
		}
		s := bufio.NewScanner(reader)
		for s.Scan() {
			line := s.Text()
			singleLineEnv := strings.Index(line, "=")
			multiLineEnv := strings.Index(line, "<<")
			if singleLineEnv != -1 && (multiLineEnv == -1 || singleLineEnv < multiLineEnv) {
				localEnv[line[:singleLineEnv]] = line[singleLineEnv+1:]
			} else if multiLineEnv != -1 {
				multiLineEnvContent := ""
				multiLineEnvDelimiter := line[multiLineEnv+2:]
				delimiterFound := false
				for s.Scan() {
					content := s.Text()
					if content == multiLineEnvDelimiter {
						delimiterFound = true
						break
					}
					if multiLineEnvContent != "" {
						multiLineEnvContent += "\n"
					}
					multiLineEnvContent += content
				}
				if !delimiterFound {
					return fmt.Errorf("invalid format delimiter '%v' not found before end of file", )
				}
				localEnv[line[:multiLineEnv]] = multiLineEnvContent
			} else {
				return fmt.Errorf("invalid format '%v', expected a line with '=' or '<<'", line)
			}
		}
		env = &localEnv
		return nil
	}
}

func (e *HostEnvironment) UpdateFromPath(env *map[string]string) common.Executor {
	localEnv := *env
	return func(ctx context.Context) error {
		pathTar, err := e.GetContainerArchive(ctx, localEnv["GITHUB_PATH"])
		if err != nil {
			return err
		}
		defer pathTar.Close()

		reader := tar.NewReader(pathTar)
		_, err = reader.Next()
		if err != nil && err != io.EOF {
			return err
		}
		s := bufio.NewScanner(reader)
		for s.Scan() {
			line := s.Text()
			pathSep := string(filepath.ListSeparator)
			localEnv[e.GetPathVariableName()] = fmt.Sprintf("%s%s%s", line, pathSep, localEnv[e.GetPathVariableName()])
		}

		env = &localEnv
		return nil
	}
}

func (e *HostEnvironment) Remove() common.Executor {
	return func(ctx context.Context) error {
		if e.CleanUp != nil {
			e.CleanUp()
		}
		return os.RemoveAll(e.Path)
	}
}

func (e *HostEnvironment) ToContainerPath(path string) string {
	if bp, err := filepath.Rel(e.Workdir, path); err != nil {
		return filepath.Join(e.Path, bp)
	} else if filepath.Clean(e.Workdir) == filepath.Clean(path) {
		return e.Path
	}
	return path
}

func (e *HostEnvironment) GetActPath() string {
	return e.ActPath
}

func (*HostEnvironment) GetPathVariableName() string {
	if runtime.GOOS == "plan9" {
		return "path"
	} else if runtime.GOOS == "windows" {
		return "Path" // Actually we need a case insensitive map
	}
	return "PATH"
}

func (e *HostEnvironment) DefaultPathVariable() string {
	v, _ := os.LookupEnv(e.GetPathVariableName())
	return v
}

func (*HostEnvironment) JoinPathVariable(paths ...string) string {
	return strings.Join(paths, string(filepath.ListSeparator))
}

func (e *HostEnvironment) GetRunnerContext(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"temp":       e.TmpDir,
		"tool_cache": e.ToolCache,
	}
}

func (e *HostEnvironment) ReplaceLogWriter(stdout io.Writer, stderr io.Writer) (io.Writer, io.Writer) {
	org := e.StdOut
	e.StdOut = stdout
	return org, org
}
