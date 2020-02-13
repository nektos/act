/*
Package ast declares types representing a JavaScript AST.

Warning

The parser and AST interfaces are still works-in-progress (particularly where
node types are concerned) and may change in the future.

*/
package ast

import (
	"github.com/robertkrimen/otto/file"
	"github.com/robertkrimen/otto/token"
)

// All nodes implement the Node interface.
type Node interface {
	Idx0() file.Idx // The index of the first character belonging to the node
	Idx1() file.Idx // The index of the first character immediately after the node
}

// ========== //
// Expression //
// ========== //

type (
	// All expression nodes implement the Expression interface.
	Expression interface {
		Node
		_expressionNode()
	}

	ArrayLiteral struct {
		LeftBracket  file.Idx
		RightBracket file.Idx
		Value        []Expression
	}

	AssignExpression struct {
		Operator token.Token
		Left     Expression
		Right    Expression
	}

	BadExpression struct {
		From file.Idx
		To   file.Idx
	}

	BinaryExpression struct {
		Operator   token.Token
		Left       Expression
		Right      Expression
		Comparison bool
	}

	BooleanLiteral struct {
		Idx     file.Idx
		Literal string
		Value   bool
	}

	BracketExpression struct {
		Left         Expression
		Member       Expression
		LeftBracket  file.Idx
		RightBracket file.Idx
	}

	CallExpression struct {
		Callee           Expression
		LeftParenthesis  file.Idx
		ArgumentList     []Expression
		RightParenthesis file.Idx
	}

	ConditionalExpression struct {
		Test       Expression
		Consequent Expression
		Alternate  Expression
	}

	DotExpression struct {
		Left       Expression
		Identifier *Identifier
	}

	EmptyExpression struct {
		Begin file.Idx
		End   file.Idx
	}

	FunctionLiteral struct {
		Function      file.Idx
		Name          *Identifier
		ParameterList *ParameterList
		Body          Statement
		Source        string

		DeclarationList []Declaration
	}

	Identifier struct {
		Name string
		Idx  file.Idx
	}

	NewExpression struct {
		New              file.Idx
		Callee           Expression
		LeftParenthesis  file.Idx
		ArgumentList     []Expression
		RightParenthesis file.Idx
	}

	NullLiteral struct {
		Idx     file.Idx
		Literal string
	}

	NumberLiteral struct {
		Idx     file.Idx
		Literal string
		Value   interface{}
	}

	ObjectLiteral struct {
		LeftBrace  file.Idx
		RightBrace file.Idx
		Value      []Property
	}

	ParameterList struct {
		Opening file.Idx
		List    []*Identifier
		Closing file.Idx
	}

	Property struct {
		Key   string
		Kind  string
		Value Expression
	}

	RegExpLiteral struct {
		Idx     file.Idx
		Literal string
		Pattern string
		Flags   string
		Value   string
	}

	SequenceExpression struct {
		Sequence []Expression
	}

	StringLiteral struct {
		Idx     file.Idx
		Literal string
		Value   string
	}

	ThisExpression struct {
		Idx file.Idx
	}

	UnaryExpression struct {
		Operator token.Token
		Idx      file.Idx // If a prefix operation
		Operand  Expression
		Postfix  bool
	}

	VariableExpression struct {
		Name        string
		Idx         file.Idx
		Initializer Expression
	}
)

// _expressionNode

func (*ArrayLiteral) _expressionNode()          {}
func (*AssignExpression) _expressionNode()      {}
func (*BadExpression) _expressionNode()         {}
func (*BinaryExpression) _expressionNode()      {}
func (*BooleanLiteral) _expressionNode()        {}
func (*BracketExpression) _expressionNode()     {}
func (*CallExpression) _expressionNode()        {}
func (*ConditionalExpression) _expressionNode() {}
func (*DotExpression) _expressionNode()         {}
func (*EmptyExpression) _expressionNode()       {}
func (*FunctionLiteral) _expressionNode()       {}
func (*Identifier) _expressionNode()            {}
func (*NewExpression) _expressionNode()         {}
func (*NullLiteral) _expressionNode()           {}
func (*NumberLiteral) _expressionNode()         {}
func (*ObjectLiteral) _expressionNode()         {}
func (*RegExpLiteral) _expressionNode()         {}
func (*SequenceExpression) _expressionNode()    {}
func (*StringLiteral) _expressionNode()         {}
func (*ThisExpression) _expressionNode()        {}
func (*UnaryExpression) _expressionNode()       {}
func (*VariableExpression) _expressionNode()    {}

// ========= //
// Statement //
// ========= //

type (
	// All statement nodes implement the Statement interface.
	Statement interface {
		Node
		_statementNode()
	}

	BadStatement struct {
		From file.Idx
		To   file.Idx
	}

	BlockStatement struct {
		LeftBrace  file.Idx
		List       []Statement
		RightBrace file.Idx
	}

	BranchStatement struct {
		Idx   file.Idx
		Token token.Token
		Label *Identifier
	}

	CaseStatement struct {
		Case       file.Idx
		Test       Expression
		Consequent []Statement
	}

	CatchStatement struct {
		Catch     file.Idx
		Parameter *Identifier
		Body      Statement
	}

	DebuggerStatement struct {
		Debugger file.Idx
	}

	DoWhileStatement struct {
		Do   file.Idx
		Test Expression
		Body Statement
	}

	EmptyStatement struct {
		Semicolon file.Idx
	}

	ExpressionStatement struct {
		Expression Expression
	}

	ForInStatement struct {
		For    file.Idx
		Into   Expression
		Source Expression
		Body   Statement
	}

	ForStatement struct {
		For         file.Idx
		Initializer Expression
		Update      Expression
		Test        Expression
		Body        Statement
	}

	FunctionStatement struct {
		Function *FunctionLiteral
	}

	IfStatement struct {
		If         file.Idx
		Test       Expression
		Consequent Statement
		Alternate  Statement
	}

	LabelledStatement struct {
		Label     *Identifier
		Colon     file.Idx
		Statement Statement
	}

	ReturnStatement struct {
		Return   file.Idx
		Argument Expression
	}

	SwitchStatement struct {
		Switch       file.Idx
		Discriminant Expression
		Default      int
		Body         []*CaseStatement
	}

	ThrowStatement struct {
		Throw    file.Idx
		Argument Expression
	}

	TryStatement struct {
		Try     file.Idx
		Body    Statement
		Catch   *CatchStatement
		Finally Statement
	}

	VariableStatement struct {
		Var  file.Idx
		List []Expression
	}

	WhileStatement struct {
		While file.Idx
		Test  Expression
		Body  Statement
	}

	WithStatement struct {
		With   file.Idx
		Object Expression
		Body   Statement
	}
)

// _statementNode

func (*BadStatement) _statementNode()        {}
func (*BlockStatement) _statementNode()      {}
func (*BranchStatement) _statementNode()     {}
func (*CaseStatement) _statementNode()       {}
func (*CatchStatement) _statementNode()      {}
func (*DebuggerStatement) _statementNode()   {}
func (*DoWhileStatement) _statementNode()    {}
func (*EmptyStatement) _statementNode()      {}
func (*ExpressionStatement) _statementNode() {}
func (*ForInStatement) _statementNode()      {}
func (*ForStatement) _statementNode()        {}
func (*FunctionStatement) _statementNode()   {}
func (*IfStatement) _statementNode()         {}
func (*LabelledStatement) _statementNode()   {}
func (*ReturnStatement) _statementNode()     {}
func (*SwitchStatement) _statementNode()     {}
func (*ThrowStatement) _statementNode()      {}
func (*TryStatement) _statementNode()        {}
func (*VariableStatement) _statementNode()   {}
func (*WhileStatement) _statementNode()      {}
func (*WithStatement) _statementNode()       {}

// =========== //
// Declaration //
// =========== //

type (
	// All declaration nodes implement the Declaration interface.
	Declaration interface {
		_declarationNode()
	}

	FunctionDeclaration struct {
		Function *FunctionLiteral
	}

	VariableDeclaration struct {
		Var  file.Idx
		List []*VariableExpression
	}
)

// _declarationNode

func (*FunctionDeclaration) _declarationNode() {}
func (*VariableDeclaration) _declarationNode() {}

// ==== //
// Node //
// ==== //

type Program struct {
	Body []Statement

	DeclarationList []Declaration

	File *file.File

	Comments CommentMap
}

// ==== //
// Idx0 //
// ==== //

func (self *ArrayLiteral) Idx0() file.Idx          { return self.LeftBracket }
func (self *AssignExpression) Idx0() file.Idx      { return self.Left.Idx0() }
func (self *BadExpression) Idx0() file.Idx         { return self.From }
func (self *BinaryExpression) Idx0() file.Idx      { return self.Left.Idx0() }
func (self *BooleanLiteral) Idx0() file.Idx        { return self.Idx }
func (self *BracketExpression) Idx0() file.Idx     { return self.Left.Idx0() }
func (self *CallExpression) Idx0() file.Idx        { return self.Callee.Idx0() }
func (self *ConditionalExpression) Idx0() file.Idx { return self.Test.Idx0() }
func (self *DotExpression) Idx0() file.Idx         { return self.Left.Idx0() }
func (self *EmptyExpression) Idx0() file.Idx       { return self.Begin }
func (self *FunctionLiteral) Idx0() file.Idx       { return self.Function }
func (self *Identifier) Idx0() file.Idx            { return self.Idx }
func (self *NewExpression) Idx0() file.Idx         { return self.New }
func (self *NullLiteral) Idx0() file.Idx           { return self.Idx }
func (self *NumberLiteral) Idx0() file.Idx         { return self.Idx }
func (self *ObjectLiteral) Idx0() file.Idx         { return self.LeftBrace }
func (self *RegExpLiteral) Idx0() file.Idx         { return self.Idx }
func (self *SequenceExpression) Idx0() file.Idx    { return self.Sequence[0].Idx0() }
func (self *StringLiteral) Idx0() file.Idx         { return self.Idx }
func (self *ThisExpression) Idx0() file.Idx        { return self.Idx }
func (self *UnaryExpression) Idx0() file.Idx       { return self.Idx }
func (self *VariableExpression) Idx0() file.Idx    { return self.Idx }

func (self *BadStatement) Idx0() file.Idx        { return self.From }
func (self *BlockStatement) Idx0() file.Idx      { return self.LeftBrace }
func (self *BranchStatement) Idx0() file.Idx     { return self.Idx }
func (self *CaseStatement) Idx0() file.Idx       { return self.Case }
func (self *CatchStatement) Idx0() file.Idx      { return self.Catch }
func (self *DebuggerStatement) Idx0() file.Idx   { return self.Debugger }
func (self *DoWhileStatement) Idx0() file.Idx    { return self.Do }
func (self *EmptyStatement) Idx0() file.Idx      { return self.Semicolon }
func (self *ExpressionStatement) Idx0() file.Idx { return self.Expression.Idx0() }
func (self *ForInStatement) Idx0() file.Idx      { return self.For }
func (self *ForStatement) Idx0() file.Idx        { return self.For }
func (self *FunctionStatement) Idx0() file.Idx   { return self.Function.Idx0() }
func (self *IfStatement) Idx0() file.Idx         { return self.If }
func (self *LabelledStatement) Idx0() file.Idx   { return self.Label.Idx0() }
func (self *Program) Idx0() file.Idx             { return self.Body[0].Idx0() }
func (self *ReturnStatement) Idx0() file.Idx     { return self.Return }
func (self *SwitchStatement) Idx0() file.Idx     { return self.Switch }
func (self *ThrowStatement) Idx0() file.Idx      { return self.Throw }
func (self *TryStatement) Idx0() file.Idx        { return self.Try }
func (self *VariableStatement) Idx0() file.Idx   { return self.Var }
func (self *WhileStatement) Idx0() file.Idx      { return self.While }
func (self *WithStatement) Idx0() file.Idx       { return self.With }

// ==== //
// Idx1 //
// ==== //

func (self *ArrayLiteral) Idx1() file.Idx          { return self.RightBracket }
func (self *AssignExpression) Idx1() file.Idx      { return self.Right.Idx1() }
func (self *BadExpression) Idx1() file.Idx         { return self.To }
func (self *BinaryExpression) Idx1() file.Idx      { return self.Right.Idx1() }
func (self *BooleanLiteral) Idx1() file.Idx        { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *BracketExpression) Idx1() file.Idx     { return self.RightBracket + 1 }
func (self *CallExpression) Idx1() file.Idx        { return self.RightParenthesis + 1 }
func (self *ConditionalExpression) Idx1() file.Idx { return self.Test.Idx1() }
func (self *DotExpression) Idx1() file.Idx         { return self.Identifier.Idx1() }
func (self *EmptyExpression) Idx1() file.Idx       { return self.End }
func (self *FunctionLiteral) Idx1() file.Idx       { return self.Body.Idx1() }
func (self *Identifier) Idx1() file.Idx            { return file.Idx(int(self.Idx) + len(self.Name)) }
func (self *NewExpression) Idx1() file.Idx         { return self.RightParenthesis + 1 }
func (self *NullLiteral) Idx1() file.Idx           { return file.Idx(int(self.Idx) + 4) } // "null"
func (self *NumberLiteral) Idx1() file.Idx         { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *ObjectLiteral) Idx1() file.Idx         { return self.RightBrace }
func (self *RegExpLiteral) Idx1() file.Idx         { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *SequenceExpression) Idx1() file.Idx    { return self.Sequence[0].Idx1() }
func (self *StringLiteral) Idx1() file.Idx         { return file.Idx(int(self.Idx) + len(self.Literal)) }
func (self *ThisExpression) Idx1() file.Idx        { return self.Idx + 4 }
func (self *UnaryExpression) Idx1() file.Idx {
	if self.Postfix {
		return self.Operand.Idx1() + 2 // ++ --
	}
	return self.Operand.Idx1()
}
func (self *VariableExpression) Idx1() file.Idx {
	if self.Initializer == nil {
		return file.Idx(int(self.Idx) + len(self.Name) + 1)
	}
	return self.Initializer.Idx1()
}

func (self *BadStatement) Idx1() file.Idx        { return self.To }
func (self *BlockStatement) Idx1() file.Idx      { return self.RightBrace + 1 }
func (self *BranchStatement) Idx1() file.Idx     { return self.Idx }
func (self *CaseStatement) Idx1() file.Idx       { return self.Consequent[len(self.Consequent)-1].Idx1() }
func (self *CatchStatement) Idx1() file.Idx      { return self.Body.Idx1() }
func (self *DebuggerStatement) Idx1() file.Idx   { return self.Debugger + 8 }
func (self *DoWhileStatement) Idx1() file.Idx    { return self.Test.Idx1() }
func (self *EmptyStatement) Idx1() file.Idx      { return self.Semicolon + 1 }
func (self *ExpressionStatement) Idx1() file.Idx { return self.Expression.Idx1() }
func (self *ForInStatement) Idx1() file.Idx      { return self.Body.Idx1() }
func (self *ForStatement) Idx1() file.Idx        { return self.Body.Idx1() }
func (self *FunctionStatement) Idx1() file.Idx   { return self.Function.Idx1() }
func (self *IfStatement) Idx1() file.Idx {
	if self.Alternate != nil {
		return self.Alternate.Idx1()
	}
	return self.Consequent.Idx1()
}
func (self *LabelledStatement) Idx1() file.Idx { return self.Colon + 1 }
func (self *Program) Idx1() file.Idx           { return self.Body[len(self.Body)-1].Idx1() }
func (self *ReturnStatement) Idx1() file.Idx   { return self.Return }
func (self *SwitchStatement) Idx1() file.Idx   { return self.Body[len(self.Body)-1].Idx1() }
func (self *ThrowStatement) Idx1() file.Idx    { return self.Throw }
func (self *TryStatement) Idx1() file.Idx      { return self.Try }
func (self *VariableStatement) Idx1() file.Idx { return self.List[len(self.List)-1].Idx1() }
func (self *WhileStatement) Idx1() file.Idx    { return self.Body.Idx1() }
func (self *WithStatement) Idx1() file.Idx     { return self.Body.Idx1() }
