package runner

import "github.com/nektos/act/pkg/common"

type compositeSteps struct {
	pre  common.Executor
	main common.Executor
	post common.Executor
}
