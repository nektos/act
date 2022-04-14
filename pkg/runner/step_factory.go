package runner

import (
	"fmt"

	"github.com/nektos/act/pkg/model"
)

type stepFactory interface {
	newStep(step *model.Step, rc *RunContext) (step, error)
}

type stepFactoryImpl struct{}

func (sf *stepFactoryImpl) newStep(stepModel *model.Step, rc *RunContext) (step, error) {
	switch stepModel.Type() {
	case model.StepTypeUsesAndRun:
		return nil, fmt.Errorf("Invalid run/uses syntax for job:%s step:%+v", rc.Run, stepModel)
	case model.StepTypeMissingRun:
		return nil, fmt.Errorf("Required property is missing: run")
	case model.StepTypeRun:
		return &stepRun{
			Step:       stepModel,
			RunContext: rc,
		}, nil
	case model.StepTypeUsesActionLocal:
		return &stepActionLocal{
			Step:       stepModel,
			RunContext: rc,
			readAction: readActionImpl,
			runAction:  runActionImpl,
		}, nil
	case model.StepTypeUsesActionRemote:
		return &stepActionRemote{
			Step:       stepModel,
			RunContext: rc,
			readAction: readActionImpl,
			runAction:  runActionImpl,
		}, nil
	case model.StepTypeUsesDockerURL:
		return &stepDocker{
			Step:       stepModel,
			RunContext: rc,
		}, nil
	}

	return nil, fmt.Errorf("Unable to determine how to run job:%s step:%+v", rc.Run, stepModel)
}
