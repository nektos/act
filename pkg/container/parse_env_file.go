package container

import (
	"archive/tar"
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/nektos/act/pkg/common"
)

func parseEnvFile(e Container, srcPath string, env *map[string]string) common.Executor {
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
					return fmt.Errorf("invalid format delimiter '%v' not found before end of file", multiLineEnvDelimiter)
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
