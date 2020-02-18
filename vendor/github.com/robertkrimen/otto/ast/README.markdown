# ast
--
    import "github.com/robertkrimen/otto/ast"

Package ast declares types representing a JavaScript AST.


### Warning

The parser and AST interfaces are still works-in-progress (particularly where
node types are concerned) and may change in the future.

## Usage

#### type ArrayLiteral

```go
type ArrayLiteral struct {
	LeftBracket  file.Idx
	RightBracket file.Idx
	Value        []Expression
}
```


#### func (*ArrayLiteral) Idx0

```go
func (self *ArrayLiteral) Idx0() file.Idx
```

#### func (*ArrayLiteral) Idx1

```go
func (self *ArrayLiteral) Idx1() file.Idx
```

#### type AssignExpression

```go
type AssignExpression struct {
	Operator token.Token
	Left     Expression
	Right    Expression
}
```


#### func (*AssignExpression) Idx0

```go
func (self *AssignExpression) Idx0() file.Idx
```

#### func (*AssignExpression) Idx1

```go
func (self *AssignExpression) Idx1() file.Idx
```

#### type BadExpression

```go
type BadExpression struct {
	From file.Idx
	To   file.Idx
}
```


#### func (*BadExpression) Idx0

```go
func (self *BadExpression) Idx0() file.Idx
```

#### func (*BadExpression) Idx1

```go
func (self *BadExpression) Idx1() file.Idx
```

#### type BadStatement

```go
type BadStatement struct {
	From file.Idx
	To   file.Idx
}
```


#### func (*BadStatement) Idx0

```go
func (self *BadStatement) Idx0() file.Idx
```

#### func (*BadStatement) Idx1

```go
func (self *BadStatement) Idx1() file.Idx
```

#### type BinaryExpression

```go
type BinaryExpression struct {
	Operator   token.Token
	Left       Expression
	Right      Expression
	Comparison bool
}
```


#### func (*BinaryExpression) Idx0

```go
func (self *BinaryExpression) Idx0() file.Idx
```

#### func (*BinaryExpression) Idx1

```go
func (self *BinaryExpression) Idx1() file.Idx
```

#### type BlockStatement

```go
type BlockStatement struct {
	LeftBrace  file.Idx
	List       []Statement
	RightBrace file.Idx
}
```


#### func (*BlockStatement) Idx0

```go
func (self *BlockStatement) Idx0() file.Idx
```

#### func (*BlockStatement) Idx1

```go
func (self *BlockStatement) Idx1() file.Idx
```

#### type BooleanLiteral

```go
type BooleanLiteral struct {
	Idx     file.Idx
	Literal string
	Value   bool
}
```


#### func (*BooleanLiteral) Idx0

```go
func (self *BooleanLiteral) Idx0() file.Idx
```

#### func (*BooleanLiteral) Idx1

```go
func (self *BooleanLiteral) Idx1() file.Idx
```

#### type BracketExpression

```go
type BracketExpression struct {
	Left         Expression
	Member       Expression
	LeftBracket  file.Idx
	RightBracket file.Idx
}
```


#### func (*BracketExpression) Idx0

```go
func (self *BracketExpression) Idx0() file.Idx
```

#### func (*BracketExpression) Idx1

```go
func (self *BracketExpression) Idx1() file.Idx
```

#### type BranchStatement

```go
type BranchStatement struct {
	Idx   file.Idx
	Token token.Token
	Label *Identifier
}
```


#### func (*BranchStatement) Idx0

```go
func (self *BranchStatement) Idx0() file.Idx
```

#### func (*BranchStatement) Idx1

```go
func (self *BranchStatement) Idx1() file.Idx
```

#### type CallExpression

```go
type CallExpression struct {
	Callee           Expression
	LeftParenthesis  file.Idx
	ArgumentList     []Expression
	RightParenthesis file.Idx
}
```


#### func (*CallExpression) Idx0

```go
func (self *CallExpression) Idx0() file.Idx
```

#### func (*CallExpression) Idx1

```go
func (self *CallExpression) Idx1() file.Idx
```

#### type CaseStatement

```go
type CaseStatement struct {
	Case       file.Idx
	Test       Expression
	Consequent []Statement
}
```


#### func (*CaseStatement) Idx0

```go
func (self *CaseStatement) Idx0() file.Idx
```

#### func (*CaseStatement) Idx1

```go
func (self *CaseStatement) Idx1() file.Idx
```

#### type CatchStatement

```go
type CatchStatement struct {
	Catch     file.Idx
	Parameter *Identifier
	Body      Statement
}
```


#### func (*CatchStatement) Idx0

```go
func (self *CatchStatement) Idx0() file.Idx
```

#### func (*CatchStatement) Idx1

```go
func (self *CatchStatement) Idx1() file.Idx
```

#### type ConditionalExpression

```go
type ConditionalExpression struct {
	Test       Expression
	Consequent Expression
	Alternate  Expression
}
```


#### func (*ConditionalExpression) Idx0

```go
func (self *ConditionalExpression) Idx0() file.Idx
```

#### func (*ConditionalExpression) Idx1

```go
func (self *ConditionalExpression) Idx1() file.Idx
```

#### type DebuggerStatement

```go
type DebuggerStatement struct {
	Debugger file.Idx
}
```


#### func (*DebuggerStatement) Idx0

```go
func (self *DebuggerStatement) Idx0() file.Idx
```

#### func (*DebuggerStatement) Idx1

```go
func (self *DebuggerStatement) Idx1() file.Idx
```

#### type Declaration

```go
type Declaration interface {
	// contains filtered or unexported methods
}
```

All declaration nodes implement the Declaration interface.

#### type DoWhileStatement

```go
type DoWhileStatement struct {
	Do   file.Idx
	Test Expression
	Body Statement
}
```


#### func (*DoWhileStatement) Idx0

```go
func (self *DoWhileStatement) Idx0() file.Idx
```

#### func (*DoWhileStatement) Idx1

```go
func (self *DoWhileStatement) Idx1() file.Idx
```

#### type DotExpression

```go
type DotExpression struct {
	Left       Expression
	Identifier Identifier
}
```


#### func (*DotExpression) Idx0

```go
func (self *DotExpression) Idx0() file.Idx
```

#### func (*DotExpression) Idx1

```go
func (self *DotExpression) Idx1() file.Idx
```

#### type EmptyStatement

```go
type EmptyStatement struct {
	Semicolon file.Idx
}
```


#### func (*EmptyStatement) Idx0

```go
func (self *EmptyStatement) Idx0() file.Idx
```

#### func (*EmptyStatement) Idx1

```go
func (self *EmptyStatement) Idx1() file.Idx
```

#### type Expression

```go
type Expression interface {
	Node
	// contains filtered or unexported methods
}
```

All expression nodes implement the Expression interface.

#### type ExpressionStatement

```go
type ExpressionStatement struct {
	Expression Expression
}
```


#### func (*ExpressionStatement) Idx0

```go
func (self *ExpressionStatement) Idx0() file.Idx
```

#### func (*ExpressionStatement) Idx1

```go
func (self *ExpressionStatement) Idx1() file.Idx
```

#### type ForInStatement

```go
type ForInStatement struct {
	For    file.Idx
	Into   Expression
	Source Expression
	Body   Statement
}
```


#### func (*ForInStatement) Idx0

```go
func (self *ForInStatement) Idx0() file.Idx
```

#### func (*ForInStatement) Idx1

```go
func (self *ForInStatement) Idx1() file.Idx
```

#### type ForStatement

```go
type ForStatement struct {
	For         file.Idx
	Initializer Expression
	Update      Expression
	Test        Expression
	Body        Statement
}
```


#### func (*ForStatement) Idx0

```go
func (self *ForStatement) Idx0() file.Idx
```

#### func (*ForStatement) Idx1

```go
func (self *ForStatement) Idx1() file.Idx
```

#### type FunctionDeclaration

```go
type FunctionDeclaration struct {
	Function *FunctionLiteral
}
```


#### type FunctionLiteral

```go
type FunctionLiteral struct {
	Function      file.Idx
	Name          *Identifier
	ParameterList *ParameterList
	Body          Statement
	Source        string

	DeclarationList []Declaration
}
```


#### func (*FunctionLiteral) Idx0

```go
func (self *FunctionLiteral) Idx0() file.Idx
```

#### func (*FunctionLiteral) Idx1

```go
func (self *FunctionLiteral) Idx1() file.Idx
```

#### type Identifier

```go
type Identifier struct {
	Name string
	Idx  file.Idx
}
```


#### func (*Identifier) Idx0

```go
func (self *Identifier) Idx0() file.Idx
```

#### func (*Identifier) Idx1

```go
func (self *Identifier) Idx1() file.Idx
```

#### type IfStatement

```go
type IfStatement struct {
	If         file.Idx
	Test       Expression
	Consequent Statement
	Alternate  Statement
}
```


#### func (*IfStatement) Idx0

```go
func (self *IfStatement) Idx0() file.Idx
```

#### func (*IfStatement) Idx1

```go
func (self *IfStatement) Idx1() file.Idx
```

#### type LabelledStatement

```go
type LabelledStatement struct {
	Label     *Identifier
	Colon     file.Idx
	Statement Statement
}
```


#### func (*LabelledStatement) Idx0

```go
func (self *LabelledStatement) Idx0() file.Idx
```

#### func (*LabelledStatement) Idx1

```go
func (self *LabelledStatement) Idx1() file.Idx
```

#### type NewExpression

```go
type NewExpression struct {
	New              file.Idx
	Callee           Expression
	LeftParenthesis  file.Idx
	ArgumentList     []Expression
	RightParenthesis file.Idx
}
```


#### func (*NewExpression) Idx0

```go
func (self *NewExpression) Idx0() file.Idx
```

#### func (*NewExpression) Idx1

```go
func (self *NewExpression) Idx1() file.Idx
```

#### type Node

```go
type Node interface {
	Idx0() file.Idx // The index of the first character belonging to the node
	Idx1() file.Idx // The index of the first character immediately after the node
}
```

All nodes implement the Node interface.

#### type NullLiteral

```go
type NullLiteral struct {
	Idx     file.Idx
	Literal string
}
```


#### func (*NullLiteral) Idx0

```go
func (self *NullLiteral) Idx0() file.Idx
```

#### func (*NullLiteral) Idx1

```go
func (self *NullLiteral) Idx1() file.Idx
```

#### type NumberLiteral

```go
type NumberLiteral struct {
	Idx     file.Idx
	Literal string
	Value   interface{}
}
```


#### func (*NumberLiteral) Idx0

```go
func (self *NumberLiteral) Idx0() file.Idx
```

#### func (*NumberLiteral) Idx1

```go
func (self *NumberLiteral) Idx1() file.Idx
```

#### type ObjectLiteral

```go
type ObjectLiteral struct {
	LeftBrace  file.Idx
	RightBrace file.Idx
	Value      []Property
}
```


#### func (*ObjectLiteral) Idx0

```go
func (self *ObjectLiteral) Idx0() file.Idx
```

#### func (*ObjectLiteral) Idx1

```go
func (self *ObjectLiteral) Idx1() file.Idx
```

#### type ParameterList

```go
type ParameterList struct {
	Opening file.Idx
	List    []*Identifier
	Closing file.Idx
}
```


#### type Program

```go
type Program struct {
	Body []Statement

	DeclarationList []Declaration

	File *file.File
}
```


#### func (*Program) Idx0

```go
func (self *Program) Idx0() file.Idx
```

#### func (*Program) Idx1

```go
func (self *Program) Idx1() file.Idx
```

#### type Property

```go
type Property struct {
	Key   string
	Kind  string
	Value Expression
}
```


#### type RegExpLiteral

```go
type RegExpLiteral struct {
	Idx     file.Idx
	Literal string
	Pattern string
	Flags   string
	Value   string
}
```


#### func (*RegExpLiteral) Idx0

```go
func (self *RegExpLiteral) Idx0() file.Idx
```

#### func (*RegExpLiteral) Idx1

```go
func (self *RegExpLiteral) Idx1() file.Idx
```

#### type ReturnStatement

```go
type ReturnStatement struct {
	Return   file.Idx
	Argument Expression
}
```


#### func (*ReturnStatement) Idx0

```go
func (self *ReturnStatement) Idx0() file.Idx
```

#### func (*ReturnStatement) Idx1

```go
func (self *ReturnStatement) Idx1() file.Idx
```

#### type SequenceExpression

```go
type SequenceExpression struct {
	Sequence []Expression
}
```


#### func (*SequenceExpression) Idx0

```go
func (self *SequenceExpression) Idx0() file.Idx
```

#### func (*SequenceExpression) Idx1

```go
func (self *SequenceExpression) Idx1() file.Idx
```

#### type Statement

```go
type Statement interface {
	Node
	// contains filtered or unexported methods
}
```

All statement nodes implement the Statement interface.

#### type StringLiteral

```go
type StringLiteral struct {
	Idx     file.Idx
	Literal string
	Value   string
}
```


#### func (*StringLiteral) Idx0

```go
func (self *StringLiteral) Idx0() file.Idx
```

#### func (*StringLiteral) Idx1

```go
func (self *StringLiteral) Idx1() file.Idx
```

#### type SwitchStatement

```go
type SwitchStatement struct {
	Switch       file.Idx
	Discriminant Expression
	Default      int
	Body         []*CaseStatement
}
```


#### func (*SwitchStatement) Idx0

```go
func (self *SwitchStatement) Idx0() file.Idx
```

#### func (*SwitchStatement) Idx1

```go
func (self *SwitchStatement) Idx1() file.Idx
```

#### type ThisExpression

```go
type ThisExpression struct {
	Idx file.Idx
}
```


#### func (*ThisExpression) Idx0

```go
func (self *ThisExpression) Idx0() file.Idx
```

#### func (*ThisExpression) Idx1

```go
func (self *ThisExpression) Idx1() file.Idx
```

#### type ThrowStatement

```go
type ThrowStatement struct {
	Throw    file.Idx
	Argument Expression
}
```


#### func (*ThrowStatement) Idx0

```go
func (self *ThrowStatement) Idx0() file.Idx
```

#### func (*ThrowStatement) Idx1

```go
func (self *ThrowStatement) Idx1() file.Idx
```

#### type TryStatement

```go
type TryStatement struct {
	Try     file.Idx
	Body    Statement
	Catch   *CatchStatement
	Finally Statement
}
```


#### func (*TryStatement) Idx0

```go
func (self *TryStatement) Idx0() file.Idx
```

#### func (*TryStatement) Idx1

```go
func (self *TryStatement) Idx1() file.Idx
```

#### type UnaryExpression

```go
type UnaryExpression struct {
	Operator token.Token
	Idx      file.Idx // If a prefix operation
	Operand  Expression
	Postfix  bool
}
```


#### func (*UnaryExpression) Idx0

```go
func (self *UnaryExpression) Idx0() file.Idx
```

#### func (*UnaryExpression) Idx1

```go
func (self *UnaryExpression) Idx1() file.Idx
```

#### type VariableDeclaration

```go
type VariableDeclaration struct {
	Var  file.Idx
	List []*VariableExpression
}
```


#### type VariableExpression

```go
type VariableExpression struct {
	Name        string
	Idx         file.Idx
	Initializer Expression
}
```


#### func (*VariableExpression) Idx0

```go
func (self *VariableExpression) Idx0() file.Idx
```

#### func (*VariableExpression) Idx1

```go
func (self *VariableExpression) Idx1() file.Idx
```

#### type VariableStatement

```go
type VariableStatement struct {
	Var  file.Idx
	List []Expression
}
```


#### func (*VariableStatement) Idx0

```go
func (self *VariableStatement) Idx0() file.Idx
```

#### func (*VariableStatement) Idx1

```go
func (self *VariableStatement) Idx1() file.Idx
```

#### type WhileStatement

```go
type WhileStatement struct {
	While file.Idx
	Test  Expression
	Body  Statement
}
```


#### func (*WhileStatement) Idx0

```go
func (self *WhileStatement) Idx0() file.Idx
```

#### func (*WhileStatement) Idx1

```go
func (self *WhileStatement) Idx1() file.Idx
```

#### type WithStatement

```go
type WithStatement struct {
	With   file.Idx
	Object Expression
	Body   Statement
}
```


#### func (*WithStatement) Idx0

```go
func (self *WithStatement) Idx0() file.Idx
```

#### func (*WithStatement) Idx1

```go
func (self *WithStatement) Idx1() file.Idx
```

--
**godocdown** http://github.com/robertkrimen/godocdown
