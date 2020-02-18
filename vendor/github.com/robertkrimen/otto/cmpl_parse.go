package otto

import (
	"fmt"
	"regexp"

	"github.com/robertkrimen/otto/ast"
	"github.com/robertkrimen/otto/file"
	"github.com/robertkrimen/otto/token"
)

var trueLiteral = &_nodeLiteral{value: toValue_bool(true)}
var falseLiteral = &_nodeLiteral{value: toValue_bool(false)}
var nullLiteral = &_nodeLiteral{value: nullValue}
var emptyStatement = &_nodeEmptyStatement{}

func (cmpl *_compiler) parseExpression(in ast.Expression) _nodeExpression {
	if in == nil {
		return nil
	}

	switch in := in.(type) {

	case *ast.ArrayLiteral:
		out := &_nodeArrayLiteral{
			value: make([]_nodeExpression, len(in.Value)),
		}
		for i, value := range in.Value {
			out.value[i] = cmpl.parseExpression(value)
		}
		return out

	case *ast.AssignExpression:
		return &_nodeAssignExpression{
			operator: in.Operator,
			left:     cmpl.parseExpression(in.Left),
			right:    cmpl.parseExpression(in.Right),
		}

	case *ast.BinaryExpression:
		return &_nodeBinaryExpression{
			operator:   in.Operator,
			left:       cmpl.parseExpression(in.Left),
			right:      cmpl.parseExpression(in.Right),
			comparison: in.Comparison,
		}

	case *ast.BooleanLiteral:
		if in.Value {
			return trueLiteral
		}
		return falseLiteral

	case *ast.BracketExpression:
		return &_nodeBracketExpression{
			idx:    in.Left.Idx0(),
			left:   cmpl.parseExpression(in.Left),
			member: cmpl.parseExpression(in.Member),
		}

	case *ast.CallExpression:
		out := &_nodeCallExpression{
			callee:       cmpl.parseExpression(in.Callee),
			argumentList: make([]_nodeExpression, len(in.ArgumentList)),
		}
		for i, value := range in.ArgumentList {
			out.argumentList[i] = cmpl.parseExpression(value)
		}
		return out

	case *ast.ConditionalExpression:
		return &_nodeConditionalExpression{
			test:       cmpl.parseExpression(in.Test),
			consequent: cmpl.parseExpression(in.Consequent),
			alternate:  cmpl.parseExpression(in.Alternate),
		}

	case *ast.DotExpression:
		return &_nodeDotExpression{
			idx:        in.Left.Idx0(),
			left:       cmpl.parseExpression(in.Left),
			identifier: in.Identifier.Name,
		}

	case *ast.EmptyExpression:
		return nil

	case *ast.FunctionLiteral:
		name := ""
		if in.Name != nil {
			name = in.Name.Name
		}
		out := &_nodeFunctionLiteral{
			name:   name,
			body:   cmpl.parseStatement(in.Body),
			source: in.Source,
			file:   cmpl.file,
		}
		if in.ParameterList != nil {
			list := in.ParameterList.List
			out.parameterList = make([]string, len(list))
			for i, value := range list {
				out.parameterList[i] = value.Name
			}
		}
		for _, value := range in.DeclarationList {
			switch value := value.(type) {
			case *ast.FunctionDeclaration:
				out.functionList = append(out.functionList, cmpl.parseExpression(value.Function).(*_nodeFunctionLiteral))
			case *ast.VariableDeclaration:
				for _, value := range value.List {
					out.varList = append(out.varList, value.Name)
				}
			default:
				panic(fmt.Errorf("Here be dragons: parseProgram.declaration(%T)", value))
			}
		}
		return out

	case *ast.Identifier:
		return &_nodeIdentifier{
			idx:  in.Idx,
			name: in.Name,
		}

	case *ast.NewExpression:
		out := &_nodeNewExpression{
			callee:       cmpl.parseExpression(in.Callee),
			argumentList: make([]_nodeExpression, len(in.ArgumentList)),
		}
		for i, value := range in.ArgumentList {
			out.argumentList[i] = cmpl.parseExpression(value)
		}
		return out

	case *ast.NullLiteral:
		return nullLiteral

	case *ast.NumberLiteral:
		return &_nodeLiteral{
			value: toValue(in.Value),
		}

	case *ast.ObjectLiteral:
		out := &_nodeObjectLiteral{
			value: make([]_nodeProperty, len(in.Value)),
		}
		for i, value := range in.Value {
			out.value[i] = _nodeProperty{
				key:   value.Key,
				kind:  value.Kind,
				value: cmpl.parseExpression(value.Value),
			}
		}
		return out

	case *ast.RegExpLiteral:
		return &_nodeRegExpLiteral{
			flags:   in.Flags,
			pattern: in.Pattern,
		}

	case *ast.SequenceExpression:
		out := &_nodeSequenceExpression{
			sequence: make([]_nodeExpression, len(in.Sequence)),
		}
		for i, value := range in.Sequence {
			out.sequence[i] = cmpl.parseExpression(value)
		}
		return out

	case *ast.StringLiteral:
		return &_nodeLiteral{
			value: toValue_string(in.Value),
		}

	case *ast.ThisExpression:
		return &_nodeThisExpression{}

	case *ast.UnaryExpression:
		return &_nodeUnaryExpression{
			operator: in.Operator,
			operand:  cmpl.parseExpression(in.Operand),
			postfix:  in.Postfix,
		}

	case *ast.VariableExpression:
		return &_nodeVariableExpression{
			idx:         in.Idx0(),
			name:        in.Name,
			initializer: cmpl.parseExpression(in.Initializer),
		}

	}

	panic(fmt.Errorf("Here be dragons: cmpl.parseExpression(%T)", in))
}

func (cmpl *_compiler) parseStatement(in ast.Statement) _nodeStatement {
	if in == nil {
		return nil
	}

	switch in := in.(type) {

	case *ast.BlockStatement:
		out := &_nodeBlockStatement{
			list: make([]_nodeStatement, len(in.List)),
		}
		for i, value := range in.List {
			out.list[i] = cmpl.parseStatement(value)
		}
		return out

	case *ast.BranchStatement:
		out := &_nodeBranchStatement{
			branch: in.Token,
		}
		if in.Label != nil {
			out.label = in.Label.Name
		}
		return out

	case *ast.DebuggerStatement:
		return &_nodeDebuggerStatement{}

	case *ast.DoWhileStatement:
		out := &_nodeDoWhileStatement{
			test: cmpl.parseExpression(in.Test),
		}
		body := cmpl.parseStatement(in.Body)
		if block, ok := body.(*_nodeBlockStatement); ok {
			out.body = block.list
		} else {
			out.body = append(out.body, body)
		}
		return out

	case *ast.EmptyStatement:
		return emptyStatement

	case *ast.ExpressionStatement:
		return &_nodeExpressionStatement{
			expression: cmpl.parseExpression(in.Expression),
		}

	case *ast.ForInStatement:
		out := &_nodeForInStatement{
			into:   cmpl.parseExpression(in.Into),
			source: cmpl.parseExpression(in.Source),
		}
		body := cmpl.parseStatement(in.Body)
		if block, ok := body.(*_nodeBlockStatement); ok {
			out.body = block.list
		} else {
			out.body = append(out.body, body)
		}
		return out

	case *ast.ForStatement:
		out := &_nodeForStatement{
			initializer: cmpl.parseExpression(in.Initializer),
			update:      cmpl.parseExpression(in.Update),
			test:        cmpl.parseExpression(in.Test),
		}
		body := cmpl.parseStatement(in.Body)
		if block, ok := body.(*_nodeBlockStatement); ok {
			out.body = block.list
		} else {
			out.body = append(out.body, body)
		}
		return out

	case *ast.FunctionStatement:
		return emptyStatement

	case *ast.IfStatement:
		return &_nodeIfStatement{
			test:       cmpl.parseExpression(in.Test),
			consequent: cmpl.parseStatement(in.Consequent),
			alternate:  cmpl.parseStatement(in.Alternate),
		}

	case *ast.LabelledStatement:
		return &_nodeLabelledStatement{
			label:     in.Label.Name,
			statement: cmpl.parseStatement(in.Statement),
		}

	case *ast.ReturnStatement:
		return &_nodeReturnStatement{
			argument: cmpl.parseExpression(in.Argument),
		}

	case *ast.SwitchStatement:
		out := &_nodeSwitchStatement{
			discriminant: cmpl.parseExpression(in.Discriminant),
			default_:     in.Default,
			body:         make([]*_nodeCaseStatement, len(in.Body)),
		}
		for i, clause := range in.Body {
			out.body[i] = &_nodeCaseStatement{
				test:       cmpl.parseExpression(clause.Test),
				consequent: make([]_nodeStatement, len(clause.Consequent)),
			}
			for j, value := range clause.Consequent {
				out.body[i].consequent[j] = cmpl.parseStatement(value)
			}
		}
		return out

	case *ast.ThrowStatement:
		return &_nodeThrowStatement{
			argument: cmpl.parseExpression(in.Argument),
		}

	case *ast.TryStatement:
		out := &_nodeTryStatement{
			body:    cmpl.parseStatement(in.Body),
			finally: cmpl.parseStatement(in.Finally),
		}
		if in.Catch != nil {
			out.catch = &_nodeCatchStatement{
				parameter: in.Catch.Parameter.Name,
				body:      cmpl.parseStatement(in.Catch.Body),
			}
		}
		return out

	case *ast.VariableStatement:
		out := &_nodeVariableStatement{
			list: make([]_nodeExpression, len(in.List)),
		}
		for i, value := range in.List {
			out.list[i] = cmpl.parseExpression(value)
		}
		return out

	case *ast.WhileStatement:
		out := &_nodeWhileStatement{
			test: cmpl.parseExpression(in.Test),
		}
		body := cmpl.parseStatement(in.Body)
		if block, ok := body.(*_nodeBlockStatement); ok {
			out.body = block.list
		} else {
			out.body = append(out.body, body)
		}
		return out

	case *ast.WithStatement:
		return &_nodeWithStatement{
			object: cmpl.parseExpression(in.Object),
			body:   cmpl.parseStatement(in.Body),
		}

	}

	panic(fmt.Errorf("Here be dragons: cmpl.parseStatement(%T)", in))
}

func cmpl_parse(in *ast.Program) *_nodeProgram {
	cmpl := _compiler{
		program: in,
	}
	return cmpl.parse()
}

func (cmpl *_compiler) _parse(in *ast.Program) *_nodeProgram {
	out := &_nodeProgram{
		body: make([]_nodeStatement, len(in.Body)),
		file: in.File,
	}
	for i, value := range in.Body {
		out.body[i] = cmpl.parseStatement(value)
	}
	for _, value := range in.DeclarationList {
		switch value := value.(type) {
		case *ast.FunctionDeclaration:
			out.functionList = append(out.functionList, cmpl.parseExpression(value.Function).(*_nodeFunctionLiteral))
		case *ast.VariableDeclaration:
			for _, value := range value.List {
				out.varList = append(out.varList, value.Name)
			}
		default:
			panic(fmt.Errorf("Here be dragons: cmpl.parseProgram.DeclarationList(%T)", value))
		}
	}
	return out
}

type _nodeProgram struct {
	body []_nodeStatement

	varList      []string
	functionList []*_nodeFunctionLiteral

	variableList []_nodeDeclaration

	file *file.File
}

type _nodeDeclaration struct {
	name       string
	definition _node
}

type _node interface {
}

type (
	_nodeExpression interface {
		_node
		_expressionNode()
	}

	_nodeArrayLiteral struct {
		value []_nodeExpression
	}

	_nodeAssignExpression struct {
		operator token.Token
		left     _nodeExpression
		right    _nodeExpression
	}

	_nodeBinaryExpression struct {
		operator   token.Token
		left       _nodeExpression
		right      _nodeExpression
		comparison bool
	}

	_nodeBracketExpression struct {
		idx    file.Idx
		left   _nodeExpression
		member _nodeExpression
	}

	_nodeCallExpression struct {
		callee       _nodeExpression
		argumentList []_nodeExpression
	}

	_nodeConditionalExpression struct {
		test       _nodeExpression
		consequent _nodeExpression
		alternate  _nodeExpression
	}

	_nodeDotExpression struct {
		idx        file.Idx
		left       _nodeExpression
		identifier string
	}

	_nodeFunctionLiteral struct {
		name          string
		body          _nodeStatement
		source        string
		parameterList []string
		varList       []string
		functionList  []*_nodeFunctionLiteral
		file          *file.File
	}

	_nodeIdentifier struct {
		idx  file.Idx
		name string
	}

	_nodeLiteral struct {
		value Value
	}

	_nodeNewExpression struct {
		callee       _nodeExpression
		argumentList []_nodeExpression
	}

	_nodeObjectLiteral struct {
		value []_nodeProperty
	}

	_nodeProperty struct {
		key   string
		kind  string
		value _nodeExpression
	}

	_nodeRegExpLiteral struct {
		flags   string
		pattern string // Value?
		regexp  *regexp.Regexp
	}

	_nodeSequenceExpression struct {
		sequence []_nodeExpression
	}

	_nodeThisExpression struct {
	}

	_nodeUnaryExpression struct {
		operator token.Token
		operand  _nodeExpression
		postfix  bool
	}

	_nodeVariableExpression struct {
		idx         file.Idx
		name        string
		initializer _nodeExpression
	}
)

type (
	_nodeStatement interface {
		_node
		_statementNode()
	}

	_nodeBlockStatement struct {
		list []_nodeStatement
	}

	_nodeBranchStatement struct {
		branch token.Token
		label  string
	}

	_nodeCaseStatement struct {
		test       _nodeExpression
		consequent []_nodeStatement
	}

	_nodeCatchStatement struct {
		parameter string
		body      _nodeStatement
	}

	_nodeDebuggerStatement struct {
	}

	_nodeDoWhileStatement struct {
		test _nodeExpression
		body []_nodeStatement
	}

	_nodeEmptyStatement struct {
	}

	_nodeExpressionStatement struct {
		expression _nodeExpression
	}

	_nodeForInStatement struct {
		into   _nodeExpression
		source _nodeExpression
		body   []_nodeStatement
	}

	_nodeForStatement struct {
		initializer _nodeExpression
		update      _nodeExpression
		test        _nodeExpression
		body        []_nodeStatement
	}

	_nodeIfStatement struct {
		test       _nodeExpression
		consequent _nodeStatement
		alternate  _nodeStatement
	}

	_nodeLabelledStatement struct {
		label     string
		statement _nodeStatement
	}

	_nodeReturnStatement struct {
		argument _nodeExpression
	}

	_nodeSwitchStatement struct {
		discriminant _nodeExpression
		default_     int
		body         []*_nodeCaseStatement
	}

	_nodeThrowStatement struct {
		argument _nodeExpression
	}

	_nodeTryStatement struct {
		body    _nodeStatement
		catch   *_nodeCatchStatement
		finally _nodeStatement
	}

	_nodeVariableStatement struct {
		list []_nodeExpression
	}

	_nodeWhileStatement struct {
		test _nodeExpression
		body []_nodeStatement
	}

	_nodeWithStatement struct {
		object _nodeExpression
		body   _nodeStatement
	}
)

// _expressionNode

func (*_nodeArrayLiteral) _expressionNode()          {}
func (*_nodeAssignExpression) _expressionNode()      {}
func (*_nodeBinaryExpression) _expressionNode()      {}
func (*_nodeBracketExpression) _expressionNode()     {}
func (*_nodeCallExpression) _expressionNode()        {}
func (*_nodeConditionalExpression) _expressionNode() {}
func (*_nodeDotExpression) _expressionNode()         {}
func (*_nodeFunctionLiteral) _expressionNode()       {}
func (*_nodeIdentifier) _expressionNode()            {}
func (*_nodeLiteral) _expressionNode()               {}
func (*_nodeNewExpression) _expressionNode()         {}
func (*_nodeObjectLiteral) _expressionNode()         {}
func (*_nodeRegExpLiteral) _expressionNode()         {}
func (*_nodeSequenceExpression) _expressionNode()    {}
func (*_nodeThisExpression) _expressionNode()        {}
func (*_nodeUnaryExpression) _expressionNode()       {}
func (*_nodeVariableExpression) _expressionNode()    {}

// _statementNode

func (*_nodeBlockStatement) _statementNode()      {}
func (*_nodeBranchStatement) _statementNode()     {}
func (*_nodeCaseStatement) _statementNode()       {}
func (*_nodeCatchStatement) _statementNode()      {}
func (*_nodeDebuggerStatement) _statementNode()   {}
func (*_nodeDoWhileStatement) _statementNode()    {}
func (*_nodeEmptyStatement) _statementNode()      {}
func (*_nodeExpressionStatement) _statementNode() {}
func (*_nodeForInStatement) _statementNode()      {}
func (*_nodeForStatement) _statementNode()        {}
func (*_nodeIfStatement) _statementNode()         {}
func (*_nodeLabelledStatement) _statementNode()   {}
func (*_nodeReturnStatement) _statementNode()     {}
func (*_nodeSwitchStatement) _statementNode()     {}
func (*_nodeThrowStatement) _statementNode()      {}
func (*_nodeTryStatement) _statementNode()        {}
func (*_nodeVariableStatement) _statementNode()   {}
func (*_nodeWhileStatement) _statementNode()      {}
func (*_nodeWithStatement) _statementNode()       {}
