package runner

import (
	"testing"

	"github.com/nektos/act/pkg/model"

	"github.com/stretchr/testify/assert"
)

func TestStepFactoryNewStep(t *testing.T) {
	table := []struct {
		name  string
		model *model.Step
		check func(s step) bool
	}{
		{
			name: "StepRemoteAction",
			model: &model.Step{
				Uses: "remote/action@v1",
			},
			check: func(s step) bool {
				_, ok := s.(*stepActionRemote)
				return ok
			},
		},
		{
			name: "StepLocalAction",
			model: &model.Step{
				Uses: "./action@v1",
			},
			check: func(s step) bool {
				_, ok := s.(*stepActionLocal)
				return ok
			},
		},
		{
			name: "StepDocker",
			model: &model.Step{
				Uses: "docker://image:tag",
			},
			check: func(s step) bool {
				_, ok := s.(*stepDocker)
				return ok
			},
		},
		{
			name: "StepRun",
			model: &model.Step{
				Run: "cmd",
			},
			check: func(s step) bool {
				_, ok := s.(*stepRun)
				return ok
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			sf := &stepFactoryImpl{}

			step, err := sf.newStep(tt.model, &RunContext{})

			assert.True(t, tt.check((step)))
			assert.Nil(t, err)
		})
	}
}

func TestStepFactoryInvalidStep(t *testing.T) {
	model := &model.Step{
		Uses: "remote/action@v1",
		Run:  "cmd",
	}

	sf := &stepFactoryImpl{}

	_, err := sf.newStep(model, &RunContext{})

	assert.Error(t, err)
}
