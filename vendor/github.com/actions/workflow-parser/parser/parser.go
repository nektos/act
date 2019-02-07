package parser

import (
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/actions/workflow-parser/model"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	hclparser "github.com/hashicorp/hcl/hcl/parser"
	"github.com/hashicorp/hcl/hcl/token"
	"github.com/soniakeys/graph"
)

const minVersion = 0
const maxVersion = 0
const maxSecrets = 100

type Parser struct {
	version   int
	actions   []*model.Action
	workflows []*model.Workflow
	errors    errorList

	posMap           map[interface{}]ast.Node
	suppressSeverity Severity
}

// Parse parses a .workflow file and return the actions and global variables found within.
func Parse(reader io.Reader, options ...OptionFunc) (*model.Configuration, error) {
	// FIXME - check context for deadline?
	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	root, err := hcl.ParseBytes(b)
	if err != nil {
		if pe, ok := err.(*hclparser.PosError); ok {
			pos := ErrorPos{File: pe.Pos.Filename, Line: pe.Pos.Line, Column: pe.Pos.Column}
			errors := errorList{newFatal(pos, pe.Err.Error())}
			return nil, &Error{
				message: "unable to parse",
				Errors:  errors,
			}
		}
		return nil, err
	}

	p := parseAndValidate(root.Node, options...)
	if len(p.errors) > 0 {
		return nil, &Error{
			message:   "unable to parse and validate",
			Errors:    p.errors,
			Actions:   p.actions,
			Workflows: p.workflows,
		}
	}

	return &model.Configuration{
		Actions:   p.actions,
		Workflows: p.workflows,
	}, nil
}

// parseAndValidate converts a HCL AST into a Parser and validates
// high-level structure.
// Parameters:
//  - root - the contents of a .workflow file, as AST
// Returns:
//  - a Parser structure containing actions and workflow definitions
func parseAndValidate(root ast.Node, options ...OptionFunc) *Parser {
	p := &Parser{
		posMap: make(map[interface{}]ast.Node),
	}

	for _, option := range options {
		option(p)
	}

	p.parseRoot(root)
	p.validate()
	p.errors.sort()

	return p
}

func (p *Parser) validate() {
	p.analyzeDependencies()
	p.checkCircularDependencies()
	p.checkActions()
	p.checkFlows()
}

func uniqStrings(items []string) []string {
	seen := make(map[string]bool)
	ret := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			ret = append(ret, item)
		}
	}
	return ret
}

// checkCircularDependencies finds loops in the action graph.
// It emits a fatal error for each cycle it finds, in the order (top to
// bottom, left to right) they appear in the .workflow file.
func (p *Parser) checkCircularDependencies() {
	// make a map from action name to node ID, which is the index in the p.actions array
	// That is, p.actions[actionmap[X]].Identifier == X
	actionmap := make(map[string]graph.NI)
	for i, action := range p.actions {
		actionmap[action.Identifier] = graph.NI(i)
	}

	// make an adjacency list representation of the action dependency graph
	adjList := make(graph.AdjacencyList, len(p.actions))
	for i, action := range p.actions {
		adjList[i] = make([]graph.NI, 0, len(action.Needs))
		for _, depName := range action.Needs {
			if depIdx, ok := actionmap[depName]; ok {
				adjList[i] = append(adjList[i], depIdx)
			}
		}
	}

	// find cycles, and print a fatal error for each one
	g := graph.Directed{AdjacencyList: adjList}
	g.Cycles(func(cycle []graph.NI) bool {
		node := p.posMap[&p.actions[cycle[len(cycle)-1]].Needs]
		p.addFatal(node, "Circular dependency on `%s'", p.actions[cycle[0]].Identifier)
		return true
	})
}

// checkActions returns error if any actions are syntactically correct but
// have structural errors
func (p *Parser) checkActions() {
	secrets := make(map[string]bool)
	for _, t := range p.actions {
		// Ensure the Action has a `uses` attribute
		if t.Uses == nil {
			p.addError(p.posMap[t], "Action `%s' must have a `uses' attribute", t.Identifier)
			// continue, checking other actions
		}

		// Ensure there aren't too many secrets
		for _, str := range t.Secrets {
			if !secrets[str] {
				secrets[str] = true
				if len(secrets) == maxSecrets+1 {
					p.addError(p.posMap[&t.Secrets], "All actions combined must not have more than %d unique secrets", maxSecrets)
				}
			}
		}

		// Ensure that no environment variable or secret begins with
		// "GITHUB_", unless it's "GITHUB_TOKEN".
		// Also ensure that all environment variable names come from the legal
		// form for environment variable names.
		// Finally, ensure that the same key name isn't used more than once
		// between env and secrets, combined.
		for k := range t.Env {
			p.checkEnvironmentVariable(k, p.posMap[&t.Env])
		}
		secretVars := make(map[string]bool)
		for _, k := range t.Secrets {
			p.checkEnvironmentVariable(k, p.posMap[&t.Secrets])
			if _, found := t.Env[k]; found {
				p.addError(p.posMap[&t.Secrets], "Secret `%s' conflicts with an environment variable with the same name", k)
			}
			if secretVars[k] {
				p.addWarning(p.posMap[&t.Secrets], "Secret `%s' redefined", k)
			}
			secretVars[k] = true
		}
	}
}

var envVarChecker = regexp.MustCompile(`\A[A-Za-z_][A-Za-z_0-9]*\z`)

func (p *Parser) checkEnvironmentVariable(key string, node ast.Node) {
	if key != "GITHUB_TOKEN" && strings.HasPrefix(key, "GITHUB_") {
		p.addWarning(node, "Environment variables and secrets beginning with `GITHUB_' are reserved")
	}
	if !envVarChecker.MatchString(key) {
		p.addWarning(node, "Environment variables and secrets must contain only A-Z, a-z, 0-9, and _ characters, got `%s'", key)
	}
}

// checkFlows appends an error if any workflows are syntactically correct but
// have structural errors
func (p *Parser) checkFlows() {
	actionmap := makeActionMap(p.actions)
	for _, f := range p.workflows {
		// make sure there's an `on` attribute
		if f.On == "" {
			p.addError(p.posMap[f], "Workflow `%s' must have an `on' attribute", f.Identifier)
			// continue, checking other workflows
		} else if !isAllowedEventType(f.On) {
			p.addError(p.posMap[&f.On], "Workflow `%s' has unknown `on' value `%s'", f.Identifier, f.On)
			// continue, checking other workflows
		}

		// make sure that the actions that are resolved all exist
		for _, actionID := range f.Resolves {
			_, ok := actionmap[actionID]
			if !ok {
				p.addError(p.posMap[&f.Resolves], "Workflow `%s' resolves unknown action `%s'", f.Identifier, actionID)
				// continue, checking other workflows
			}
		}
	}
}

func makeActionMap(actions []*model.Action) map[string]*model.Action {
	actionmap := make(map[string]*model.Action)
	for _, action := range actions {
		actionmap[action.Identifier] = action
	}
	return actionmap
}

// Fill in Action dependencies for all actions based on explicit dependencies
// declarations.
//
// p.actions is an array of Action objects, as parsed.  The Action objects in
// this array are mutated, by setting Action.dependencies for each.
func (p *Parser) analyzeDependencies() {
	actionmap := makeActionMap(p.actions)
	for _, action := range p.actions {
		// analyze explicit dependencies for each "needs" keyword
		p.analyzeNeeds(action, actionmap)
	}

	// uniq all the dependencies lists
	for _, action := range p.actions {
		if len(action.Needs) >= 2 {
			action.Needs = uniqStrings(action.Needs)
		}
	}
}

func (p *Parser) analyzeNeeds(action *model.Action, actionmap map[string]*model.Action) {
	for _, need := range action.Needs {
		_, ok := actionmap[need]
		if !ok {
			p.addError(p.posMap[&action.Needs], "Action `%s' needs nonexistent action `%s'", action.Identifier, need)
			// continue, checking other actions
		}
	}
}

// literalToStringMap converts a object value from the AST to a
// map[string]string.  For example, the HCL `{ a="b" c="d" }` becomes the
// Go expression map[string]string{ "a": "b", "c": "d" }.
// If the value doesn't adhere to that format -- e.g.,
// if it's not an object, or it has non-assignment attributes, or if any
// of its values are anything other than a string, the function appends an
// appropriate error.
func (p *Parser) literalToStringMap(node ast.Node) map[string]string {
	obj, ok := node.(*ast.ObjectType)

	if !ok {
		p.addError(node, "Expected object, got %s", typename(node))
		return nil
	}

	p.checkAssignmentsOnly(obj.List, "")

	ret := make(map[string]string)
	for _, item := range obj.List.Items {
		if !isAssignment(item) {
			continue
		}
		str, ok := p.literalToString(item.Val)
		if ok {
			key := p.identString(item.Keys[0].Token)
			if key != "" {
				if _, found := ret[key]; found {
					p.addWarning(node, "Environment variable `%s' redefined", key)
				}
				ret[key] = str
			}
		}
	}

	return ret
}

func (p *Parser) identString(t token.Token) string {
	switch t.Type {
	case token.STRING:
		return t.Value().(string)
	case token.IDENT:
		return t.Text
	default:
		p.addErrorFromToken(t,
			"Each identifier should be a string, got %s",
			strings.ToLower(t.Type.String()))
		return ""
	}
}

// literalToStringArray converts a list value from the AST to a []string.
// For example, the HCL `[ "a", "b", "c" ]` becomes the Go expression
// []string{ "a", "b", "c" }.
// If the value doesn't adhere to that format -- it's not a list, or it
// contains anything other than strings, the function appends an
// appropriate error.
// If promoteScalars is true, then values that are scalar strings are
// promoted to a single-entry string array.  E.g., "foo" becomes the Go
// expression []string{ "foo" }.
func (p *Parser) literalToStringArray(node ast.Node, promoteScalars bool) ([]string, bool) {
	literal, ok := node.(*ast.LiteralType)
	if ok {
		if promoteScalars && literal.Token.Type == token.STRING {
			return []string{literal.Token.Value().(string)}, true
		}
		p.addError(node, "Expected list, got %s", typename(node))
		return nil, false
	}

	list, ok := node.(*ast.ListType)
	if !ok {
		p.addError(node, "Expected list, got %s", typename(node))
		return nil, false
	}

	ret := make([]string, 0, len(list.List))
	for _, literal := range list.List {
		str, ok := p.literalToString(literal)
		if ok {
			ret = append(ret, str)
		}
	}

	return ret, true
}

// literalToString converts a literal value from the AST into a string.
// If the value isn't a scalar or isn't a string, the function appends an
// appropriate error and returns "", false.
func (p *Parser) literalToString(node ast.Node) (string, bool) {
	val := p.literalCast(node, token.STRING)
	if val == nil {
		return "", false
	}
	return val.(string), true
}

// literalToInt converts a literal value from the AST into an int64.
// Supported number formats are: 123, 0x123, and 0123.
// Exponents (1e6) and floats (123.456) generate errors.
// If the value isn't a scalar or isn't a number, the function appends an
// appropriate error and returns 0, false.
func (p *Parser) literalToInt(node ast.Node) (int64, bool) {
	val := p.literalCast(node, token.NUMBER)
	if val == nil {
		return 0, false
	}
	return val.(int64), true
}

func (p *Parser) literalCast(node ast.Node, t token.Type) interface{} {
	literal, ok := node.(*ast.LiteralType)
	if !ok {
		p.addError(node, "Expected %s, got %s", strings.ToLower(t.String()), typename(node))
		return nil
	}

	if literal.Token.Type != t {
		p.addError(node, "Expected %s, got %s", strings.ToLower(t.String()), typename(node))
		return nil
	}

	return literal.Token.Value()
}

// parseRoot parses the root of the AST, filling in p.version, p.actions,
// and p.workflows.
func (p *Parser) parseRoot(node ast.Node) {
	objectList, ok := node.(*ast.ObjectList)
	if !ok {
		// It should be impossible for HCL to return anything other than an
		// ObjectList as the root node.  This error should never happen.
		p.addError(node, "Internal error: root node must be an ObjectList")
		return
	}

	p.actions = make([]*model.Action, 0, len(objectList.Items))
	p.workflows = make([]*model.Workflow, 0, len(objectList.Items))
	identifiers := make(map[string]bool)
	for idx, item := range objectList.Items {
		if item.Assign.IsValid() {
			p.parseVersion(idx, item)
			continue
		}
		p.parseBlock(item, identifiers)
	}
}

// parseBlock parses a single, top-level "action" or "workflow" block,
// appending it to p.actions or p.workflows as appropriate.
func (p *Parser) parseBlock(item *ast.ObjectItem, identifiers map[string]bool) {
	if len(item.Keys) != 2 {
		p.addError(item, "Invalid toplevel declaration")
		return
	}

	cmd := p.identString(item.Keys[0].Token)
	var id string

	switch cmd {
	case "action":
		action := p.actionifyItem(item)
		if action != nil {
			id = action.Identifier
			p.actions = append(p.actions, action)
		}
	case "workflow":
		workflow := p.workflowifyItem(item)
		if workflow != nil {
			id = workflow.Identifier
			p.workflows = append(p.workflows, workflow)
		}
	default:
		p.addError(item, "Invalid toplevel keyword, `%s'", cmd)
		return
	}

	if identifiers[id] {
		p.addError(item, "Identifier `%s' redefined", id)
	}

	identifiers[id] = true
}

// parseVersion parses a top-level `version=N` statement, filling in
// p.version.
func (p *Parser) parseVersion(idx int, item *ast.ObjectItem) {
	if len(item.Keys) != 1 || p.identString(item.Keys[0].Token) != "version" {
		// not a valid `version` declaration
		p.addError(item.Val, "Toplevel declarations cannot be assignments")
		return
	}
	if idx != 0 {
		p.addError(item.Val, "`version` must be the first declaration")
		return
	}
	version, ok := p.literalToInt(item.Val)
	if !ok {
		return
	}
	if version < minVersion || version > maxVersion {
		p.addError(item.Val, "`version = %d` is not supported", version)
		return
	}
	p.version = int(version)
}

// parseIdentifier parses the double-quoted identifier (name) for a
// "workflow" or "action" block.
func (p *Parser) parseIdentifier(key *ast.ObjectKey) string {
	id := key.Token.Text
	if len(id) < 3 || id[0] != '"' || id[len(id)-1] != '"' {
		p.addError(key, "Invalid format for identifier `%s'", id)
		return ""
	}
	return id[1 : len(id)-1]
}

// parseRequiredString parses a string value, setting its value into the
// out-parameter `value` and returning true if successful.
func (p *Parser) parseRequiredString(value *string, val ast.Node, nodeType, name, id string) bool {
	if *value != "" {
		p.addWarning(val, "`%s' redefined in %s `%s'", name, nodeType, id)
		// continue, allowing the redefinition
	}

	newVal, ok := p.literalToString(val)
	if !ok {
		p.addError(val, "Invalid format for `%s' in %s `%s', expected string", name, nodeType, id)
		return false
	}

	if newVal == "" {
		p.addError(val, "`%s' value in %s `%s' cannot be blank", name, nodeType, id)
		return false
	}

	*value = newVal
	return true
}

// parseBlockPreamble parses the beginning of a "workflow" or "action"
// block.
func (p *Parser) parseBlockPreamble(item *ast.ObjectItem, nodeType string) (string, *ast.ObjectType) {
	id := p.parseIdentifier(item.Keys[1])
	if id == "" {
		return "", nil
	}

	node := item.Val
	obj, ok := node.(*ast.ObjectType)
	if !ok {
		p.addError(node, "Each %s must have an { ...  } block", nodeType)
		return "", nil
	}

	p.checkAssignmentsOnly(obj.List, id)

	return id, obj
}

// actionifyItem converts an AST block to an Action object.
func (p *Parser) actionifyItem(item *ast.ObjectItem) *model.Action {
	id, obj := p.parseBlockPreamble(item, "action")
	if obj == nil {
		return nil
	}

	action := &model.Action{
		Identifier: id,
	}
	p.posMap[action] = item

	for _, item := range obj.List.Items {
		p.parseActionAttribute(p.identString(item.Keys[0].Token), action, item.Val)
	}

	return action
}

// parseActionAttribute parses a single key-value pair from an "action"
// block.  This function rejects any unknown keys and enforces formatting
// requirements on all values.
// It also has higher-than-normal cyclomatic complexity, so we ask the
// gocyclo linter to ignore it.
// nolint: gocyclo
func (p *Parser) parseActionAttribute(name string, action *model.Action, val ast.Node) {
	switch name {
	case "uses":
		p.parseUses(action, val)
	case "needs":
		if needs, ok := p.literalToStringArray(val, true); ok {
			action.Needs = needs
			p.posMap[&action.Needs] = val
		}
	case "runs":
		if runs := p.parseCommand(action, action.Runs, name, val, false); runs != nil {
			action.Runs = runs
		}
	case "args":
		if args := p.parseCommand(action, action.Args, name, val, true); args != nil {
			action.Args = args
		}
	case "env":
		if env := p.literalToStringMap(val); env != nil {
			action.Env = env
		}
		p.posMap[&action.Env] = val
	case "secrets":
		if secrets, ok := p.literalToStringArray(val, false); ok {
			action.Secrets = secrets
			p.posMap[&action.Secrets] = val
		}
	default:
		p.addWarning(val, "Unknown action attribute `%s'", name)
	}
}

// parseUses sets the action.Uses value based on the contents of the AST
// node.  This function enforces formatting requirements on the value.
func (p *Parser) parseUses(action *model.Action, node ast.Node) {
	if action.Uses != nil {
		p.addWarning(node, "`uses' redefined in action `%s'", action.Identifier)
		// continue, allowing the redefinition
	}
	strVal, ok := p.literalToString(node)
	if !ok {
		return
	}

	if strVal == "" {
		action.Uses = &model.UsesInvalid{}
		p.addError(node, "`uses' value in action `%s' cannot be blank", action.Identifier)
		return
	}
	if strings.HasPrefix(strVal, "./") {
		action.Uses = &model.UsesPath{Path: strings.TrimPrefix(strVal, "./")}
		return
	}

	if strings.HasPrefix(strVal, "docker://") {
		action.Uses = &model.UsesDockerImage{Image: strings.TrimPrefix(strVal, "docker://")}
		return
	}

	tok := strings.Split(strVal, "@")
	if len(tok) != 2 {
		action.Uses = &model.UsesInvalid{Raw: strVal}
		p.addError(node, "The `uses' attribute must be a path, a Docker image, or owner/repo@ref")
		return
	}
	ref := tok[1]
	tok = strings.SplitN(tok[0], "/", 3)
	if len(tok) < 2 {
		action.Uses = &model.UsesInvalid{Raw: strVal}
		p.addError(node, "The `uses' attribute must be a path, a Docker image, or owner/repo@ref")
		return
	}
	usesRepo := &model.UsesRepository{Repository: tok[0] + "/" + tok[1], Ref: ref}
	action.Uses = usesRepo
	if len(tok) == 3 {
		usesRepo.Path = tok[2]
	}
}

// parseUses sets the action.Runs or action.Args value based on the
// contents of the AST node.  This function enforces formatting
// requirements on the value.
func (p *Parser) parseCommand(action *model.Action, cmd model.Command, name string, node ast.Node, allowBlank bool) model.Command {
	if cmd != nil {
		p.addWarning(node, "`%s' redefined in action `%s'", name, action.Identifier)
		// continue, allowing the redefinition
	}

	// Is it a list?
	if _, ok := node.(*ast.ListType); ok {
		if parsed, ok := p.literalToStringArray(node, false); ok {
			return &model.ListCommand{Values: parsed}
		}
		return nil
	}

	// If not, parse a whitespace-separated string into a list.
	var raw string
	var ok bool
	if raw, ok = p.literalToString(node); !ok {
		p.addError(node, "The `%s' attribute must be a string or a list", name)
		return nil
	}
	if raw == "" && !allowBlank {
		p.addError(node, "`%s' value in action `%s' cannot be blank", name, action.Identifier)
		return nil
	}
	return &model.StringCommand{Value: raw}
}

func typename(val interface{}) string {
	switch cast := val.(type) {
	case *ast.ListType:
		return "list"
	case *ast.LiteralType:
		return strings.ToLower(cast.Token.Type.String())
	case *ast.ObjectType:
		return "object"
	default:
		return fmt.Sprintf("%T", val)
	}
}

// workflowifyItem converts an AST block to a Workflow object.
func (p *Parser) workflowifyItem(item *ast.ObjectItem) *model.Workflow {
	id, obj := p.parseBlockPreamble(item, "workflow")
	if obj == nil {
		return nil
	}

	var ok bool
	workflow := &model.Workflow{Identifier: id}
	for _, item := range obj.List.Items {
		name := p.identString(item.Keys[0].Token)

		switch name {
		case "on":
			ok = p.parseRequiredString(&workflow.On, item.Val, "workflow", name, id)
			if ok {
				p.posMap[&workflow.On] = item
			}
		case "resolves":
			if workflow.Resolves != nil {
				p.addWarning(item.Val, "`resolves' redefined in workflow `%s'", id)
				// continue, allowing the redefinition
			}
			workflow.Resolves, ok = p.literalToStringArray(item.Val, true)
			p.posMap[&workflow.Resolves] = item
			if !ok {
				p.addError(item.Val, "Invalid format for `resolves' in workflow `%s', expected list of strings", id)
				// continue, allowing workflow with no `resolves`
			}
		default:
			p.addWarning(item.Val, "Unknown workflow attribute `%s'", name)
			// continue, treat as no-op
		}
	}

	p.posMap[workflow] = item
	return workflow
}

func isAssignment(item *ast.ObjectItem) bool {
	return len(item.Keys) == 1 && item.Assign.IsValid()
}

// checkAssignmentsOnly ensures that all elements in the object are "key =
// value" pairs.
func (p *Parser) checkAssignmentsOnly(objectList *ast.ObjectList, actionID string) {
	for _, item := range objectList.Items {
		if !isAssignment(item) {
			var desc string
			if actionID == "" {
				desc = "the object"
			} else {
				desc = fmt.Sprintf("action `%s'", actionID)
			}
			p.addErrorFromObjectItem(item, "Each attribute of %s must be an assignment", desc)
			continue
		}

		child, ok := item.Val.(*ast.ObjectType)
		if ok {
			p.checkAssignmentsOnly(child.List, actionID)
		}
	}
}

func (p *Parser) addWarning(node ast.Node, format string, a ...interface{}) {
	if p.suppressSeverity < WARNING {
		p.errors = append(p.errors, newWarning(posFromNode(node), format, a...))
	}
}

func (p *Parser) addError(node ast.Node, format string, a ...interface{}) {
	if p.suppressSeverity < ERROR {
		p.errors = append(p.errors, newError(posFromNode(node), format, a...))
	}
}

func (p *Parser) addErrorFromToken(t token.Token, format string, a ...interface{}) {
	if p.suppressSeverity < ERROR {
		p.errors = append(p.errors, newError(posFromToken(t), format, a...))
	}
}

func (p *Parser) addErrorFromObjectItem(objectItem *ast.ObjectItem, format string, a ...interface{}) {
	if p.suppressSeverity < ERROR {
		p.errors = append(p.errors, newError(posFromObjectItem(objectItem), format, a...))
	}
}

func (p *Parser) addFatal(node ast.Node, format string, a ...interface{}) {
	if p.suppressSeverity < FATAL {
		p.errors = append(p.errors, newFatal(posFromNode(node), format, a...))
	}
}

// posFromNode returns an ErrorPos (file, line, and column) from an AST
// node, so we can report specific locations for each parse error.
func posFromNode(node ast.Node) ErrorPos {
	var pos *token.Pos
	switch cast := node.(type) {
	case *ast.ObjectList:
		if len(cast.Items) > 0 {
			if len(cast.Items[0].Keys) > 0 {
				pos = &cast.Items[0].Keys[0].Token.Pos
			}
		}
	case *ast.ObjectItem:
		return posFromNode(cast.Val)
	case *ast.ObjectType:
		pos = &cast.Lbrace
	case *ast.LiteralType:
		pos = &cast.Token.Pos
	case *ast.ListType:
		pos = &cast.Lbrack
	case *ast.ObjectKey:
		pos = &cast.Token.Pos
	}

	if pos == nil {
		return ErrorPos{}
	}
	return ErrorPos{File: pos.Filename, Line: pos.Line, Column: pos.Column}
}

// posFromObjectItem returns an ErrorPos from an ObjectItem.  This is for
// cases where posFromNode(item) would fail because the item has no Val
// set.
func posFromObjectItem(item *ast.ObjectItem) ErrorPos {
	if len(item.Keys) > 0 {
		return posFromNode(item.Keys[0])
	}
	return ErrorPos{}
}

// posFromToken returns an ErrorPos from a Token.  We can't use
// posFromNode here because Tokens aren't Nodes.
func posFromToken(token token.Token) ErrorPos {
	return ErrorPos{File: token.Pos.Filename, Line: token.Pos.Line, Column: token.Pos.Column}
}
