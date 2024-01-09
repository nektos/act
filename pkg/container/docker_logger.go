//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"

	"github.com/sirupsen/logrus"
)

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

const logPrefix = "  \U0001F433  "

func logDockerResponse(logger logrus.FieldLogger, dockerResponse io.ReadCloser, isError bool) error {
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
			writeLog(logger, false, "Unable to unmarshal line [%s] ==> %v", string(line), err)
			continue
		}

		if msg.Error != "" {
			writeLog(logger, isError, "%s", msg.Error)
			return errors.New(msg.Error)
		}

		if msg.ErrorDetail.Message != "" {
			writeLog(logger, isError, "%s", msg.ErrorDetail.Message)
			return errors.New(msg.Error)
		}

		if msg.Status != "" {
			if msg.Progress != "" {
				writeLog(logger, isError, "%s :: %s :: %s\n", msg.Status, msg.ID, msg.Progress)
			} else {
				writeLog(logger, isError, "%s :: %s\n", msg.Status, msg.ID)
			}
		} else if msg.Stream != "" {
			writeLog(logger, isError, "%s", msg.Stream)
		} else {
			writeLog(logger, false, "Unable to handle line: %s", string(line))
		}
	}

	return nil
}

func writeLog(logger logrus.FieldLogger, isError bool, format string, args ...interface{}) {
	if isError {
		logger.Errorf(format, args...)
	} else {
		logger.Debugf(format, args...)
	}
}
