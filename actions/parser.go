package actions

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/hcl/hcl/token"
	log "github.com/sirupsen/logrus"
)

func parseWorkflowsFile(workflowReader io.Reader) (*workflowsFile, error) {
	// TODO: add validation logic
	// - check for circular dependencies
	// - check for valid local path refs
	// - check for valid dependencies

	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(workflowReader)
	if err != nil {
		log.Error(err)
	}

	workflows := new(workflowsFile)

	astFile, err := hcl.ParseBytes(buf.Bytes())
	if err != nil {
		return nil, err
	}
	rootNode := ast.Walk(astFile.Node, cleanWorkflowsAST)
	err = hcl.DecodeObject(workflows, rootNode)
	if err != nil {
		return nil, err
	}

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
				parts, err := parseCommand(literalType.Token.Value().(string))
				if err != nil {
					return nil, false
				}
				quote := literalType.Token.Text[0]
				for _, part := range parts {
					part = fmt.Sprintf("%c%s%c", quote, part, quote)
					listType.Add(&ast.LiteralType{
						Token: token.Token{
							Type: token.STRING,
							Text: part,
						},
					})
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

// reused from: https://github.com/laurent22/massren/blob/ae4c57da1e09a95d9383f7eb645a9f69790dec6c/main.go#L172
// nolint: gocyclo
func parseCommand(cmd string) ([]string, error) {
	var args []string
	state := "start"
	current := ""
	quote := "\""
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]

		if state == "quotes" {
			if string(c) != quote {
				current += string(c)
			} else {
				args = append(args, current)
				current = ""
				state = "start"
			}
			continue
		}

		if c == '"' || c == '\'' {
			state = "quotes"
			quote = string(c)
			continue
		}

		if state == "arg" {
			if c == ' ' || c == '\t' {
				args = append(args, current)
				current = ""
				state = "start"
			} else {
				current += string(c)
			}
			continue
		}

		if c != ' ' && c != '\t' {
			state = "arg"
			current += string(c)
		}
	}

	if state == "quotes" {
		return []string{}, fmt.Errorf("unclosed quote in command line: %s", cmd)
	}

	if current != "" {
		args = append(args, current)
	}

	if len(args) == 0 {
		return []string{}, errors.New("empty command line")
	}

	log.Debugf("Parsed literal %+q to list %+q", cmd, args)

	return args, nil
}
