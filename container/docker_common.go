package container

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

// DockerExecutorInput common input params
type DockerExecutorInput struct {
	Ctx    context.Context
	Logger *logrus.Entry
	Dryrun bool
}

type dockerMessage struct {
	ID          string `json:"id"`
	Stream      string `json:"stream"`
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string
	}
	Status   string `json:"status"`
	Progress string `json:"progress"`
}

func (i *DockerExecutorInput) logDockerOutput(dockerResponse io.Reader) {
	scanner := bufio.NewScanner(dockerResponse)
	if i.Logger == nil {
		return
	}
	for scanner.Scan() {
		i.Logger.Infof(scanner.Text())
	}
}

func (i *DockerExecutorInput) streamDockerOutput(dockerResponse io.Reader) {
	out := os.Stdout
	go func() {
		<-i.Ctx.Done()
		fmt.Println()
	}()

	_, err := io.Copy(out, dockerResponse)
	if err != nil {
		i.Logger.Error(err)
	}
}

func (i *DockerExecutorInput) writeLog(isError bool, format string, args ...interface{}) {
	if i.Logger == nil {
		return
	}
	if isError {
		i.Logger.Errorf(format, args...)
	} else {
		i.Logger.Debugf(format, args...)
	}

}

func (i *DockerExecutorInput) logDockerResponse(dockerResponse io.ReadCloser, isError bool) error {
	if dockerResponse == nil {
		return nil
	}
	defer dockerResponse.Close()

	scanner := bufio.NewScanner(dockerResponse)
	msg := dockerMessage{}

	for scanner.Scan() {
		line := scanner.Bytes()

		msg.ID = ""
		msg.Stream = ""
		msg.Error = ""
		msg.ErrorDetail.Message = ""
		msg.Status = ""
		msg.Progress = ""

		if err := json.Unmarshal(line, &msg); err != nil {
			i.writeLog(false, "Unable to unmarshal line [%s] ==> %v", string(line), err)
			continue
		}

		if msg.Error != "" {
			i.writeLog(isError, "%s", msg.Error)
			return errors.New(msg.Error)
		}

		if msg.ErrorDetail.Message != "" {
			i.writeLog(isError, "%s", msg.ErrorDetail.Message)
			return errors.New(msg.Error)
		}

		if msg.Status != "" {
			if msg.Progress != "" {
				i.writeLog(isError, "%s :: %s :: %s\n", msg.Status, msg.ID, msg.Progress)
			} else {
				i.writeLog(isError, "%s :: %s\n", msg.Status, msg.ID)
			}
		} else if msg.Stream != "" {
			i.writeLog(isError, msg.Stream)
		} else {
			i.writeLog(false, "Unable to handle line: %s", string(line))
		}
	}

	return nil
}
