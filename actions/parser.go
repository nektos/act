package actions

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/hcl/hcl/token"
	log "github.com/sirupsen/logrus"
)

// ParseWorkflows will read in the set of actions from the workflow file
func ParseWorkflows(workingDir string, workflowPath string) (Workflows, error) {
	workingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, err
	}
	log.Debugf("Setting working dir to %s", workingDir)

	if !filepath.IsAbs(workflowPath) {
		workflowPath = filepath.Join(workingDir, workflowPath)
	}
	log.Debugf("Loading workflow config from %s", workflowPath)
	workflowReader, err := os.Open(workflowPath)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(workflowReader)

	workflows := new(workflowsFile)
	workflows.WorkingDir = workingDir
	workflows.WorkflowPath = workflowPath

	astFile, err := hcl.ParseBytes(buf.Bytes())
	if err != nil {
		return nil, err
	}
	rootNode := ast.Walk(astFile.Node, cleanWorkflowsAST)
	err = hcl.DecodeObject(workflows, rootNode)
	if err != nil {
		return nil, err
	}

	workflows.TempDir, err = ioutil.TempDir("/tmp", "act-")
	if err != nil {
		return nil, err
	}

	// TODO: add validation logic
	// - check for circular dependencies
	// - check for valid local path refs
	// - check for valid dependencies

	return workflows, nil
}

func cleanWorkflowsAST(node ast.Node) (ast.Node, bool) {
	if objectItem, ok := node.(*ast.ObjectItem); ok {
		key := objectItem.Keys[0].Token.Value()

		// handle condition where value is a string but should be a list
		switch key {
		case "args", "runs":
			if literalType, ok := objectItem.Val.(*ast.LiteralType); ok {
				listType := new(ast.ListType)
				parts := strings.Split(literalType.Token.Value().(string), " ")
				log.Debugf("got list: %v", parts)
				if len(parts) > 0 {
					quote := literalType.Token.Text[0]
					for _, part := range parts {
						part = fmt.Sprintf("%c%s%c", quote, part, quote)
						log.Debugf("Adding part %s", part)
						listType.Add(&ast.LiteralType{
							Token: token.Token{
								Type: token.STRING,
								Text: part,
							},
						})
					}
				}
				objectItem.Val = listType

			}
		case "resolves", "needs":
			if literalType, ok := objectItem.Val.(*ast.LiteralType); ok {
				listType := new(ast.ListType)
				listType.Add(literalType)
				objectItem.Val = listType

			}
		}
	}
	return node, true
}
