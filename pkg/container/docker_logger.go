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

/*
func logDockerOutput(ctx context.Context, dockerResponse io.Reader) {
	logger := common.Logger(ctx)
	if entry, ok := logger.(*logrus.Entry); ok {
		w := entry.Writer()
		_, err := stdcopy.StdCopy(w, w, dockerResponse)
		if err != nil {
			logrus.Error(err)
		}
	} else if lgr, ok := logger.(*logrus.Logger); ok {
		w := lgr.Writer()
		_, err := stdcopy.StdCopy(w, w, dockerResponse)
		if err != nil {
			logrus.Error(err)
		}
	} else {
		logrus.Errorf("Unable to get writer from logger (type=%T)", logger)
	}
}
*/

/*
func streamDockerOutput(ctx context.Context, dockerResponse io.Reader) {
	/*
		out := os.Stdout
		go func() {
			<-ctx.Done()
			//fmt.Println()
		}()

		_, err := io.Copy(out, dockerResponse)
		if err != nil {
			logrus.Error(err)
		}
	* /

	logger := common.Logger(ctx)
	reader := bufio.NewReader(dockerResponse)

	for {
		if ctx.Err() != nil {
			break
		}
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		logger.Debugf("%s\n", line)
	}

}
*/

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
			writeLog(logger, isError, msg.Stream)
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
