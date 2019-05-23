package container

import (
	"bytes"
	"context"
	"github.com/nektos/act/common"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
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

	runner := NewDockerRunExecutor(NewDockerRunExecutorInput{
		DockerExecutorInput: DockerExecutorInput{
			Ctx:    context.TODO(),
			Logger: logrus.NewEntry(logger),
		},
		Image: "hello-world",
	})

	puller := NewDockerPullExecutor(NewDockerPullExecutorInput{
		DockerExecutorInput: DockerExecutorInput{
			Ctx:    context.TODO(),
			Logger: logrus.NewEntry(noopLogger),
		},
		Image: "hello-world",
	})

	pipeline := common.NewPipelineExecutor(puller, runner)
	err := pipeline()
	assert.NoError(t, err)

	expected := `docker run image=hello-world entrypoint=[] cmd=[]Hello from Docker!`
	actual := buf.String()
	assert.Equal(t, expected, actual[:len(expected)])
}
