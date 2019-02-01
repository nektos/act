package model

// Configuration is a parsed main.workflow file
type Configuration struct {
	Actions   []*Action
	Workflows []*Workflow
}

// Action represents a single "action" stanza in a .workflow file.
type Action struct {
	Identifier string
	Uses       Uses
	Runs, Args Command
	Needs      []string
	Env        map[string]string
	Secrets    []string
}

// Workflow represents a single "workflow" stanza in a .workflow file.
type Workflow struct {
	Identifier string
	On         string
	Resolves   []string
}

// GetAction looks up action by identifier.
//
// If the action is not found, nil is returned.
func (c *Configuration) GetAction(id string) *Action {
	for _, action := range c.Actions {
		if action.Identifier == id {
			return action
		}
	}
	return nil
}

// GetWorkflow looks up a workflow by identifier.
//
// If the workflow is not found, nil is returned.
func (c *Configuration) GetWorkflow(id string) *Workflow {
	for _, workflow := range c.Workflows {
		if workflow.Identifier == id {
			return workflow
		}
	}
	return nil
}

// GetWorkflows gets all Workflow structures that match a given type of event.
// e.g., GetWorkflows("push")
func (c *Configuration) GetWorkflows(eventType string) []*Workflow {
	var ret []*Workflow
	for _, workflow := range c.Workflows {
		if IsMatchingEventType(workflow.On, eventType) {
			ret = append(ret, workflow)
		}
	}
	return ret
}
