package model

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type WorkflowPlanTest struct {
	workflowPath      string
	errorMessage      string
	noWorkflowRecurse bool
}

type WorkflowPlanFilterTest struct {
	workflow          string
	workflowEventPath string
	shouldBeFiltered  bool
	errorMsg          string
}

func TestPlanner(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	tables := []WorkflowPlanTest{
		{"invalid-job-name/invalid-1.yml", "workflow is not valid. 'invalid-job-name-1': Job name 'invalid-JOB-Name-v1.2.3-docker_hub' is invalid. Names must start with a letter or '_' and contain only alphanumeric characters, '-', or '_'", false},
		{"invalid-job-name/invalid-2.yml", "workflow is not valid. 'invalid-job-name-2': Job name '1234invalid-JOB-Name-v123-docker_hub' is invalid. Names must start with a letter or '_' and contain only alphanumeric characters, '-', or '_'", false},
		{"invalid-job-name/valid-1.yml", "", false},
		{"invalid-job-name/valid-2.yml", "", false},
		{"empty-workflow", "unable to read workflow 'push.yml': file is empty: EOF", false},
		{"nested", "unable to read workflow 'fail.yml': file is empty: EOF", false},
		{"nested", "", true},
	}

	workdir, err := filepath.Abs("testdata")
	assert.NoError(t, err, workdir)
	for _, table := range tables {
		fullWorkflowPath := filepath.Join(workdir, table.workflowPath)
		_, err = NewWorkflowPlanner(fullWorkflowPath, table.noWorkflowRecurse, false, "")
		if table.errorMessage == "" {
			assert.NoError(t, err, "WorkflowPlanner should exit without any error")
		} else {
			assert.EqualError(t, err, table.errorMessage)
		}
	}
}

func TestPlannerPushEventFiltering(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	pushSimple := `
    name: test-job
    on: push

    jobs:
      a-job:
        runs-on: ubuntu-latest
        steps:
          - run: echo hi
    `

	pushSimple2 := `
    name: test-job
    on:
        - push

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushSimple3 := `
    name: test-job
    on:
        push:

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pullRequestSimple := `
    name: test-job
    on:
        pull_request:

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushFilterBranch := `
    name: test-job
    on:
        push:
            branches:
                - refs/heads/master

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushFilterTag := `
    name: test-job
    on:
        push:
            tags:
                - refs/tags/v1

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushFilterPath := `
    name: test-job
    on:
        push:
            paths:
                - test/app/**

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushFilterPath2 := `
    name: test-job
    on:
        push:
            paths:
                - test/other/**

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushFilterBranchTag := `
    name: test-job
    on:
        push:
            branches:
                - refs/heads/master
            tags:
                - refs/tags/v1

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushFilterBranchTagPath := `
    name: test-job
    on:
        push:
            branches:
                - refs/heads/master
            tags:
                - refs/tags/v1
            paths:
                - test/app/**
    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushDoubleFilterBranch := `
    name: test-job
    on:
        push:
            branches:
                - does/not/exist

            branches_ignore:
                - does/not/exist

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushDoubleFilterTag := `
    name: test-job
    on:
        push:
            tags:
                - does/not/exist

            tags_ignore:
                - does/not/exist

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushDoubleFilterPath := `
    name: test-job
    on:
        push:
            paths:
                - does/not/exist

            paths_ignore:
                - does/not/exist

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushFilterBranchIgnoreTag := `
    name: test-job
    on:
        push:
            branches:
                - 'refs/heads/*'
            tags_ignore:
                - '**'

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushFilterIgnoreBranchTag := `
    name: test-job
    on:
        push:
            branches_ignore:
                - 'refs/heads/*'
            tags:
                - 'refs/tags/*'

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushFilterIgnorePath := `
    name: test-job
    on:
        push:
            paths_ignore:
                - test/app/**
    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `
	tables := []WorkflowPlanFilterTest{
		{pushSimple, "", false, "Push should not be filtered"},
		{pushSimple2, "", false, "Push should not be filtered"},
		{pushSimple3, "", false, "Push should not be filtered"},
		{pullRequestSimple, "", true, "Pull request workflow for push event should be filtered when no event object passed"},
		{pullRequestSimple, "filter-events/push-master.json", true, "Pull request workflow for push event should be filtered"},
		{pushFilterBranch, "filter-events/push-master.json", false, "Branches should match so workflow should not be filtered"},
		{pushFilterBranch, "filter-events/push-foo.json", true, "Branches should not match so workflow should be filtered"},
		{pushFilterTag, "filter-events/push-tag-v1.json", false, "Tags should match so workflow should not be filtered"},
		{pushFilterTag, "filter-events/push-tag-v2.json", true, "Tags should not match so workflow should be filtered"},
		{pushFilterPath, "filter-events/push-master.json", false, "Paths should match so workflow should not be filtered"},
		{pushFilterPath2, "filter-events/push-master.json", true, "Paths should not match so workflow should be filtered"},
		{pushFilterBranchTag, "filter-events/push-master.json", false, "Branches or tags should match so workflow should not be filtered"},
		{pushFilterBranchTag, "filter-events/push-foo.json", true, "Neither branches or tags should match so workflow should be filtered"},
		{pushFilterBranchTag, "filter-events/push-tag-v1.json", false, "Branches or tags should match so workflow should not be filtered"},
		{pushFilterBranchTagPath, "filter-events/push-master.json", false, "Branches and path should match so workflow should not be filtered"},
		{pushFilterBranchTagPath, "filter-events/push-foo.json", true, "Branches don't match even though path matches so workflow should be filtered"},
		{pushFilterBranchTagPath, "filter-events/push-tag-v1.json", false, "Tag matches and path should be ignored so workflow should not be filtered"},
		{pushDoubleFilterBranch, "filter-events/push-master.json", false, "No match for event but it's malformed so workflow should not be filtered"},
		{pushDoubleFilterTag, "filter-events/push-tag-v1.json", false, "No match for event but it's malformed so workflow should not be filtered"},
		{pushDoubleFilterPath, "filter-events/push-master.json", false, "No match for event but it's malformed so workflow should not be filtered"},
		{pushFilterBranchIgnoreTag, "filter-events/push-foo.json", false, "Wildcard branch and ignore all tags so workflow should not be filtered"},
		{pushFilterBranchIgnoreTag, "filter-events/push-tag-v1.json", true, "Wildcard branch and ignore all tags so workflow should be filtered"},
		{pushFilterIgnoreBranchTag, "filter-events/push-foo.json", true, "Wildcard branch ignore and accept all tags so workflow should be filtered"},
		{pushFilterIgnoreBranchTag, "filter-events/push-tag-v2.json", false, "Wildcard branch ignore and accept all tags so workflow should not be filtered"},
		{pushFilterIgnorePath, "filter-events/push-tag-v2.json", true, "Wildcard paths ignore so workflow should be filtered"},
	}

	workdir, err := filepath.Abs("testdata")
	assert.NoError(t, err, workdir)
	for n, table := range tables {
		fullEventPath := ""
		if table.workflowEventPath != "" {
			fullEventPath = filepath.Join(workdir, table.workflowEventPath)
		}
		planner, err := NewSingleWorkflowPlanner(fmt.Sprintf("Test %d", n+1), strings.NewReader(table.workflow), true, fullEventPath)
		assert.NoError(t, err, "WorkflowPlanner should exit without any error")

		plan, err := planner.PlanEvent("push")
		assert.NoError(t, err)
		if table.shouldBeFiltered {
			assert.Equal(t, 0, len(plan.Stages), fmt.Sprintf("%d: %s", n+1, table.errorMsg))
		} else {
			assert.Equal(t, 1, len(plan.Stages), fmt.Sprintf("%d: %s", n+1, table.errorMsg))
		}
	}
}

//nolint:dupl
func TestPlannerPullRequestEventFiltering(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	prSimple := `
    name: test-job
    on: pull_request

    jobs:
      a-job:
        runs-on: ubuntu-latest
        steps:
          - run: echo hi
    `

	prSimple2 := `
    name: test-job
    on:
        - pull_request

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	prSimple3 := `
    name: test-job
    on:
        pull_request:

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushRequestSimple := `
    name: test-job
    on:
        push:

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	prFilterBranch := `
    name: test-job
    on:
       pull_request:
           branches:
               - main

    jobs:
       a-job:
           runs-on: ubuntu-latest
           steps:
             - run: echo hi
    `

	prFilterBranchFoo := `
    name: test-job
    on:
       pull_request:
           branches:
               - foo

    jobs:
       a-job:
           runs-on: ubuntu-latest
           steps:
             - run: echo hi
    `

	prDoubleFilterBranch := `
    name: test-job
    on:
       pull_request:
           branches:
               - does/not/exist

           branches_ignore:
               - does/not/exist

    jobs:
       a-job:
           runs-on: ubuntu-latest
           steps:
             - run: echo hi
    `

	prDoubleFilterPath := `
    name: test-job
    on:
       pull_request:
           paths:
               - does/not/exist

           paths_ignore:
               - does/not/exist

    jobs:
       a-job:
           runs-on: ubuntu-latest
           steps:
             - run: echo hi
    `

	prFilterIgnoreBranch := `
    name: test-job
    on:
       pull_request:
           branches_ignore:
               - '*'

    jobs:
       a-job:
           runs-on: ubuntu-latest
           steps:
             - run: echo hi
    `

	tables := []WorkflowPlanFilterTest{
		{prSimple, "", false, "PR should not be filtered"},
		{prSimple2, "", false, "PR should not be filtered"},
		{prSimple3, "", false, "PR should not be filtered"},
		{pushRequestSimple, "", true, "Push workflow for PS event should be filtered when no event object passed"},
		{prFilterBranch, "filter-events/pr-opened.json", false, "Branches should match so workflow should not be filtered"},
		{prFilterBranchFoo, "filter-events/pr-opened.json", true, "Branches should not match so workflow should be filtered"},
		{prDoubleFilterBranch, "filter-events/pr-opened.json", false, "No match for event but it's malformed so workflow should not be filtered"},
		{prDoubleFilterPath, "filter-events/pr-opened.json", false, "No match for event but it's malformed so workflow should not be filtered"},
		{prFilterIgnoreBranch, "filter-events/pr-opened.json", true, "Wildcard paths ignore so workflow should be filtered"},
	}

	workdir, err := filepath.Abs("testdata")
	assert.NoError(t, err, workdir)
	for n, table := range tables {
		fullEventPath := ""
		if table.workflowEventPath != "" {
			fullEventPath = filepath.Join(workdir, table.workflowEventPath)
		}
		planner, err := NewSingleWorkflowPlanner(fmt.Sprintf("Test %d", n+1), strings.NewReader(table.workflow), true, fullEventPath)
		assert.NoError(t, err, "WorkflowPlanner should exit without any error")

		plan, err := planner.PlanEvent("pull_request")
		assert.NoError(t, err)
		if table.shouldBeFiltered {
			assert.Equal(t, 0, len(plan.Stages), fmt.Sprintf("%d: %s", n+1, table.errorMsg))
		} else {
			assert.Equal(t, 1, len(plan.Stages), fmt.Sprintf("%d: %s", n+1, table.errorMsg))
		}
	}
}

//nolint:dupl
func TestPlannerPullRequestTargetEventFiltering(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	prtSimple := `
    name: test-job
    on: pull_request_target

    jobs:
      a-job:
        runs-on: ubuntu-latest
        steps:
          - run: echo hi
    `

	prtSimple2 := `
    name: test-job
    on:
        - pull_request_target

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	prtSimple3 := `
    name: test-job
    on:
        pull_request_target:

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	pushRequestSimple := `
    name: test-job
    on:
        push:

    jobs:
        a-job:
            runs-on: ubuntu-latest
            steps:
              - run: echo hi
    `

	prtFilterBranch := `
    name: test-job
    on:
       pull_request_target:
           branches:
               - main

    jobs:
       a-job:
           runs-on: ubuntu-latest
           steps:
             - run: echo hi
    `

	prtFilterBranchFoo := `
    name: test-job
    on:
       pull_request_target:
           branches:
               - foo

    jobs:
       a-job:
           runs-on: ubuntu-latest
           steps:
             - run: echo hi
    `

	prtDoubleFilterBranch := `
    name: test-job
    on:
       pull_request_target:
           branches:
               - does/not/exist

           branches_ignore:
               - does/not/exist

    jobs:
       a-job:
           runs-on: ubuntu-latest
           steps:
             - run: echo hi
    `

	prtDoubleFilterPath := `
    name: test-job
    on:
       pull_request_target:
           paths:
               - does/not/exist

           paths_ignore:
               - does/not/exist

    jobs:
       a-job:
           runs-on: ubuntu-latest
           steps:
             - run: echo hi
    `

	prtFilterIgnoreBranch := `
    name: test-job
    on:
       pull_request_target:
           branches_ignore:
               - '*'

    jobs:
       a-job:
           runs-on: ubuntu-latest
           steps:
             - run: echo hi
    `

	tables := []WorkflowPlanFilterTest{
		{prtSimple, "", false, "PR should not be filtered"},
		{prtSimple2, "", false, "PR should not be filtered"},
		{prtSimple3, "", false, "PR should not be filtered"},
		{pushRequestSimple, "", true, "Push workflow for PS event should be filtered when no event object passed"},
		{prtFilterBranch, "filter-events/pr-opened.json", false, "Branches should match so workflow should not be filtered"},
		{prtFilterBranchFoo, "filter-events/pr-opened.json", true, "Branches should not match so workflow should be filtered"},
		{prtDoubleFilterBranch, "filter-events/pr-opened.json", false, "No match for event but it's malformed so workflow should not be filtered"},
		{prtDoubleFilterPath, "filter-events/pr-opened.json", false, "No match for event but it's malformed so workflow should not be filtered"},
		{prtFilterIgnoreBranch, "filter-events/pr-opened.json", true, "Wildcard paths ignore so workflow should be filtered"},
	}

	workdir, err := filepath.Abs("testdata")
	assert.NoError(t, err, workdir)
	for n, table := range tables {
		fullEventPath := ""
		if table.workflowEventPath != "" {
			fullEventPath = filepath.Join(workdir, table.workflowEventPath)
		}
		planner, err := NewSingleWorkflowPlanner(fmt.Sprintf("Test %d", n+1), strings.NewReader(table.workflow), true, fullEventPath)
		assert.NoError(t, err, "WorkflowPlanner should exit without any error")

		plan, err := planner.PlanEvent("pull_request_target")
		assert.NoError(t, err)
		if table.shouldBeFiltered {
			assert.Equal(t, 0, len(plan.Stages), fmt.Sprintf("%d: %s", n+1, table.errorMsg))
		} else {
			assert.Equal(t, 1, len(plan.Stages), fmt.Sprintf("%d: %s", n+1, table.errorMsg))
		}
	}
}

func TestWorkflow(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	workflow := Workflow{
		Jobs: map[string]*Job{
			"valid_job": {
				Name: "valid_job",
			},
		},
	}

	// Check that a valid job id returns non-error
	result, err := createStages(&workflow, "valid_job")
	assert.Nil(t, err)
	assert.NotNil(t, result)
}
