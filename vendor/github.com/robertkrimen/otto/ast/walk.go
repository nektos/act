package ast

import "fmt"

// Visitor Enter method is invoked for each node encountered by Walk.
// If the result visitor w is not nil, Walk visits each of the children
// of node with the visitor v, followed by a call of the Exit method.
type Visitor interface {
	Enter(n Node) (v Visitor)
	Exit(n Node)
}

// Walk traverses an AST in depth-first order: It starts by calling
// v.Enter(node); node must not be nil. If the visitor v returned by
// v.Enter(node) is not nil, Walk is invoked recursively with visitor
// v for each of the non-nil children of node, followed by a call
// of v.Exit(node).
func Walk(v Visitor, n Node) {
	if n == nil {
		return
	}
	if v = v.Enter(n); v == nil {
		return
	}

	defer v.Exit(n)

	switch n := n.(type) {
	case *ArrayLiteral:
		if n != nil {
			for _, ex := range n.Value {
				Walk(v, ex)
			}
		}
	case *AssignExpression:
		if n != nil {
			Walk(v, n.Left)
			Walk(v, n.Right)
		}
	case *BadExpression:
	case *BinaryExpression:
		if n != nil {
			Walk(v, n.Left)
			Walk(v, n.Right)
		}
	case *BlockStatement:
		if n != nil {
			for _, s := range n.List {
				Walk(v, s)
			}
		}
	case *BooleanLiteral:
	case *BracketExpression:
		if n != nil {
			Walk(v, n.Left)
			Walk(v, n.Member)
		}
	case *BranchStatement:
		if n != nil {
			Walk(v, n.Label)
		}
	case *CallExpression:
		if n != nil {
			Walk(v, n.Callee)
			for _, a := range n.ArgumentList {
				Walk(v, a)
			}
		}
	case *CaseStatement:
		if n != nil {
			Walk(v, n.Test)
			for _, c := range n.Consequent {
				Walk(v, c)
			}
		}
	case *CatchStatement:
		if n != nil {
			Walk(v, n.Parameter)
			Walk(v, n.Body)
		}
	case *ConditionalExpression:
		if n != nil {
			Walk(v, n.Test)
			Walk(v, n.Consequent)
			Walk(v, n.Alternate)
		}
	case *DebuggerStatement:
	case *DoWhileStatement:
		if n != nil {
			Walk(v, n.Test)
			Walk(v, n.Body)
		}
	case *DotExpression:
		if n != nil {
			Walk(v, n.Left)
		}
	case *EmptyExpression:
	case *EmptyStatement:
	case *ExpressionStatement:
		if n != nil {
			Walk(v, n.Expression)
		}
	case *ForInStatement:
		if n != nil {
			Walk(v, n.Into)
			Walk(v, n.Source)
			Walk(v, n.Body)
		}
	case *ForStatement:
		if n != nil {
			Walk(v, n.Initializer)
			Walk(v, n.Update)
			Walk(v, n.Test)
			Walk(v, n.Body)
		}
	case *FunctionLiteral:
		if n != nil {
			Walk(v, n.Name)
			for _, p := range n.ParameterList.List {
				Walk(v, p)
			}
			Walk(v, n.Body)
		}
	case *FunctionStatement:
		if n != nil {
			Walk(v, n.Function)
		}
	case *Identifier:
	case *IfStatement:
		if n != nil {
			Walk(v, n.Test)
			Walk(v, n.Consequent)
			Walk(v, n.Alternate)
		}
	case *LabelledStatement:
		if n != nil {
			Walk(v, n.Statement)
		}
	case *NewExpression:
		if n != nil {
			Walk(v, n.Callee)
			for _, a := range n.ArgumentList {
				Walk(v, a)
			}
		}
	case *NullLiteral:
	case *NumberLiteral:
	case *ObjectLiteral:
		if n != nil {
			for _, p := range n.Value {
				Walk(v, p.Value)
			}
		}
	case *Program:
		if n != nil {
			for _, b := range n.Body {
				Walk(v, b)
			}
		}
	case *RegExpLiteral:
	case *ReturnStatement:
		if n != nil {
			Walk(v, n.Argument)
		}
	case *SequenceExpression:
		if n != nil {
			for _, e := range n.Sequence {
				Walk(v, e)
			}
		}
	case *StringLiteral:
	case *SwitchStatement:
		if n != nil {
			Walk(v, n.Discriminant)
			for _, c := range n.Body {
				Walk(v, c)
			}
		}
	case *ThisExpression:
	case *ThrowStatement:
		if n != nil {
			Walk(v, n.Argument)
		}
	case *TryStatement:
		if n != nil {
			Walk(v, n.Body)
			Walk(v, n.Catch)
			Walk(v, n.Finally)
		}
	case *UnaryExpression:
		if n != nil {
			Walk(v, n.Operand)
		}
	case *VariableExpression:
		if n != nil {
			Walk(v, n.Initializer)
		}
	case *VariableStatement:
		if n != nil {
			for _, e := range n.List {
				Walk(v, e)
			}
		}
	case *WhileStatement:
		if n != nil {
			Walk(v, n.Test)
			Walk(v, n.Body)
		}
	case *WithStatement:
		if n != nil {
			Walk(v, n.Object)
			Walk(v, n.Body)
		}
	default:
		panic(fmt.Sprintf("Walk: unexpected node type %T", n))
	}
}
