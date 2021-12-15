package runner

import (
	"context"

	"github.com/nektos/act/pkg/common/executor"
	"github.com/nektos/act/pkg/container"

	"github.com/stretchr/testify/mock"
)

type containerMock struct {
	mock.Mock
	container.Container
}

func (cm *containerMock) Create(capAdd []string, capDrop []string) executor.Executor {
	args := cm.Called(capAdd, capDrop)
	return args.Get(0).(func(context.Context) error)
}

func (cm *containerMock) Pull(forcePull bool) executor.Executor {
	args := cm.Called(forcePull)
	return args.Get(0).(func(context.Context) error)
}

func (cm *containerMock) Start(attach bool) executor.Executor {
	args := cm.Called(attach)
	return args.Get(0).(func(context.Context) error)
}

func (cm *containerMock) Remove() executor.Executor {
	args := cm.Called()
	return args.Get(0).(func(context.Context) error)
}

func (cm *containerMock) Close() executor.Executor {
	args := cm.Called()
	return args.Get(0).(func(context.Context) error)
}

func (cm *containerMock) UpdateFromEnv(srcPath string, env *map[string]string) executor.Executor {
	args := cm.Called(srcPath, env)
	return args.Get(0).(func(context.Context) error)
}

func (cm *containerMock) UpdateFromImageEnv(env *map[string]string) executor.Executor {
	args := cm.Called(env)
	return args.Get(0).(func(context.Context) error)
}

func (cm *containerMock) UpdateFromPath(env *map[string]string) executor.Executor {
	args := cm.Called(env)
	return args.Get(0).(func(context.Context) error)
}

func (cm *containerMock) Copy(destPath string, files ...*container.FileEntry) executor.Executor {
	args := cm.Called(destPath, files)
	return args.Get(0).(func(context.Context) error)
}

func (cm *containerMock) CopyDir(destPath string, srcPath string, useGitIgnore bool) executor.Executor {
	args := cm.Called(destPath, srcPath, useGitIgnore)
	return args.Get(0).(func(context.Context) error)
}
func (cm *containerMock) Exec(command []string, env map[string]string, user, workdir string) executor.Executor {
	args := cm.Called(command, env, user, workdir)
	return args.Get(0).(func(context.Context) error)
}
