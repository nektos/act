package container

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/nektos/act/pkg/common"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type rawFormatter struct{}

func (f *rawFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(entry.Message), nil
}

func TestNewDockerRunExecutor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slower test")
	}

	noopLogger := logrus.New()
	noopLogger.SetOutput(ioutil.Discard)

	buf := &bytes.Buffer{}
	logger := logrus.New()
	logger.SetOutput(buf)
	logger.SetFormatter(&rawFormatter{})

	ctx := common.WithLogger(context.Background(), logger)

	runner := NewDockerRunExecutor(NewDockerRunExecutorInput{
		Image:  "hello-world",
		Stdout: buf,
	})

	puller := NewDockerPullExecutor(NewDockerPullExecutorInput{
		Image: "hello-world",
	})

	pipeline := common.NewPipelineExecutor(puller, runner)
	err := pipeline(ctx)
	assert.NoError(t, err)

	actual := buf.String()
	assert.Contains(t, actual, `docker pull hello-world`)
	assert.Contains(t, actual, `docker run image=hello-world entrypoint=[] cmd=[]`)
	assert.Contains(t, actual, `Hello from Docker!`)
}
