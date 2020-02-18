package otto

import (
	"fmt"
	"runtime"

	"github.com/robertkrimen/otto/token"
)

func (self *_runtime) cmpl_evaluate_nodeStatement(node _nodeStatement) Value {
	// Allow interpreter interruption
	// If the Interrupt channel is nil, then
	// we avoid runtime.Gosched() overhead (if any)
	// FIXME: Test this
	if self.otto.Interrupt != nil {
		runtime.Gosched()
		select {
		case value := <-self.otto.Interrupt:
			value()
		default:
		}
	}

	switch node := node.(type) {

	case *_nodeBlockStatement:
		labels := self.labels
		self.labels = nil

		value := self.cmpl_evaluate_nodeStatementList(node.list)
		switch value.kind {
		case valueResult:
			switch value.evaluateBreak(labels) {
			case resultBreak:
				return emptyValue
			}
		}
		return value

	case *_nodeBranchStatement:
		target := node.label
		switch node.branch { // FIXME Maybe node.kind? node.operator?
		case token.BREAK:
			return toValue(newBreakResult(target))
		case token.CONTINUE:
			return toValue(newContinueResult(target))
		}

	case *_nodeDebuggerStatement:
		if self.debugger != nil {
			self.debugger(self.otto)
		}
		return emptyValue // Nothing happens.

	case *_nodeDoWhileStatement:
		return self.cmpl_evaluate_nodeDoWhileStatement(node)

	case *_nodeEmptyStatement:
		return emptyValue

	case *_nodeExpressionStatement:
		return self.cmpl_evaluate_nodeExpression(node.expression)

	case *_nodeForInStatement:
		return self.cmpl_evaluate_nodeForInStatement(node)

	case *_nodeForStatement:
		return self.cmpl_evaluate_nodeForStatement(node)

	case *_nodeIfStatement:
		return self.cmpl_evaluate_nodeIfStatement(node)

	case *_nodeLabelledStatement:
		self.labels = append(self.labels, node.label)
		defer func() {
			if len(self.labels) > 0 {
				self.labels = self.labels[:len(self.labels)-1] // Pop the label
			} else {
				self.labels = nil
			}
		}()
		return self.cmpl_evaluate_nodeStatement(node.statement)

	case *_nodeReturnStatement:
		if node.argument != nil {
			return toValue(newReturnResult(self.cmpl_evaluate_nodeExpression(node.argument).resolve()))
		}
		return toValue(newReturnResult(Value{}))

	case *_nodeSwitchStatement:
		return self.cmpl_evaluate_nodeSwitchStatement(node)

	case *_nodeThrowStatement:
		value := self.cmpl_evaluate_nodeExpression(node.argument).resolve()
		panic(newException(value))

	case *_nodeTryStatement:
		return self.cmpl_evaluate_nodeTryStatement(node)

	case *_nodeVariableStatement:
		// Variables are already defined, this is initialization only
		for _, variable := range node.list {
			self.cmpl_evaluate_nodeVariableExpression(variable.(*_nodeVariableExpression))
		}
		return emptyValue

	case *_nodeWhileStatement:
		return self.cmpl_evaluate_nodeWhileStatement(node)

	case *_nodeWithStatement:
		return self.cmpl_evaluate_nodeWithStatement(node)

	}

	panic(fmt.Errorf("Here be dragons: evaluate_nodeStatement(%T)", node))
}

func (self *_runtime) cmpl_evaluate_nodeStatementList(list []_nodeStatement) Value {
	var result Value
	for _, node := range list {
		value := self.cmpl_evaluate_nodeStatement(node)
		switch value.kind {
		case valueResult:
			return value
		case valueEmpty:
		default:
			// We have getValue here to (for example) trigger a
			// ReferenceError (of the not defined variety)
			// Not sure if this is the best way to error out early
			// for such errors or if there is a better way
			// TODO Do we still need this?
			result = value.resolve()
		}
	}
	return result
}

func (self *_runtime) cmpl_evaluate_nodeDoWhileStatement(node *_nodeDoWhileStatement) Value {

	labels := append(self.labels, "")
	self.labels = nil

	test := node.test

	result := emptyValue
resultBreak:
	for {
		for _, node := range node.body {
			value := self.cmpl_evaluate_nodeStatement(node)
			switch value.kind {
			case valueResult:
				switch value.evaluateBreakContinue(labels) {
				case resultReturn:
					return value
				case resultBreak:
					break resultBreak
				case resultContinue:
					goto resultContinue
				}
			case valueEmpty:
			default:
				result = value
			}
		}
	resultContinue:
		if !self.cmpl_evaluate_nodeExpression(test).resolve().bool() {
			// Stahp: do ... while (false)
			break
		}
	}
	return result
}

func (self *_runtime) cmpl_evaluate_nodeForInStatement(node *_nodeForInStatement) Value {

	labels := append(self.labels, "")
	self.labels = nil

	source := self.cmpl_evaluate_nodeExpression(node.source)
	sourceValue := source.resolve()

	switch sourceValue.kind {
	case valueUndefined, valueNull:
		return emptyValue
	}

	sourceObject := self.toObject(sourceValue)

	into := node.into
	body := node.body

	result := emptyValue
	object := sourceObject
	for object != nil {
		enumerateValue := emptyValue
		object.enumerate(false, func(name string) bool {
			into := self.cmpl_evaluate_nodeExpression(into)
			// In the case of: for (var abc in def) ...
			if into.reference() == nil {
				identifier := into.string()
				// TODO Should be true or false (strictness) depending on context
				into = toValue(getIdentifierReference(self, self.scope.lexical, identifier, false, -1))
			}
			self.putValue(into.reference(), toValue_string(name))
			for _, node := range body {
				value := self.cmpl_evaluate_nodeStatement(node)
				switch value.kind {
				case valueResult:
					switch value.evaluateBreakContinue(labels) {
					case resultReturn:
						enumerateValue = value
						return false
					case resultBreak:
						object = nil
						return false
					case resultContinue:
						return true
					}
				case valueEmpty:
				default:
					enumerateValue = value
				}
			}
			return true
		})
		if object == nil {
			break
		}
		object = object.prototype
		if !enumerateValue.isEmpty() {
			result = enumerateValue
		}
	}
	return result
}

func (self *_runtime) cmpl_evaluate_nodeForStatement(node *_nodeForStatement) Value {

	labels := append(self.labels, "")
	self.labels = nil

	initializer := node.initializer
	test := node.test
	update := node.update
	body := node.body

	if initializer != nil {
		initialResult := self.cmpl_evaluate_nodeExpression(initializer)
		initialResult.resolve() // Side-effect trigger
	}

	result := emptyValue
resultBreak:
	for {
		if test != nil {
			testResult := self.cmpl_evaluate_nodeExpression(test)
			testResultValue := testResult.resolve()
			if testResultValue.bool() == false {
				break
			}
		}
		for _, node := range body {
			value := self.cmpl_evaluate_nodeStatement(node)
			switch value.kind {
			case valueResult:
				switch value.evaluateBreakContinue(labels) {
				case resultReturn:
					return value
				case resultBreak:
					break resultBreak
				case resultContinue:
					goto resultContinue
				}
			case valueEmpty:
			default:
				result = value
			}
		}
	resultContinue:
		if update != nil {
			updateResult := self.cmpl_evaluate_nodeExpression(update)
			updateResult.resolve() // Side-effect trigger
		}
	}
	return result
}

func (self *_runtime) cmpl_evaluate_nodeIfStatement(node *_nodeIfStatement) Value {
	test := self.cmpl_evaluate_nodeExpression(node.test)
	testValue := test.resolve()
	if testValue.bool() {
		return self.cmpl_evaluate_nodeStatement(node.consequent)
	} else if node.alternate != nil {
		return self.cmpl_evaluate_nodeStatement(node.alternate)
	}

	return emptyValue
}

func (self *_runtime) cmpl_evaluate_nodeSwitchStatement(node *_nodeSwitchStatement) Value {

	labels := append(self.labels, "")
	self.labels = nil

	discriminantResult := self.cmpl_evaluate_nodeExpression(node.discriminant)
	target := node.default_

	for index, clause := range node.body {
		test := clause.test
		if test != nil {
			if self.calculateComparison(token.STRICT_EQUAL, discriminantResult, self.cmpl_evaluate_nodeExpression(test)) {
				target = index
				break
			}
		}
	}

	result := emptyValue
	if target != -1 {
		for _, clause := range node.body[target:] {
			for _, statement := range clause.consequent {
				value := self.cmpl_evaluate_nodeStatement(statement)
				switch value.kind {
				case valueResult:
					switch value.evaluateBreak(labels) {
					case resultReturn:
						return value
					case resultBreak:
						return emptyValue
					}
				case valueEmpty:
				default:
					result = value
				}
			}
		}
	}

	return result
}

func (self *_runtime) cmpl_evaluate_nodeTryStatement(node *_nodeTryStatement) Value {
	tryCatchValue, exception := self.tryCatchEvaluate(func() Value {
		return self.cmpl_evaluate_nodeStatement(node.body)
	})

	if exception && node.catch != nil {
		outer := self.scope.lexical
		self.scope.lexical = self.newDeclarationStash(outer)
		defer func() {
			self.scope.lexical = outer
		}()
		// TODO If necessary, convert TypeError<runtime> => TypeError
		// That, is, such errors can be thrown despite not being JavaScript "native"
		// strict = false
		self.scope.lexical.setValue(node.catch.parameter, tryCatchValue, false)

		// FIXME node.CatchParameter
		// FIXME node.Catch
		tryCatchValue, exception = self.tryCatchEvaluate(func() Value {
			return self.cmpl_evaluate_nodeStatement(node.catch.body)
		})
	}

	if node.finally != nil {
		finallyValue := self.cmpl_evaluate_nodeStatement(node.finally)
		if finallyValue.kind == valueResult {
			return finallyValue
		}
	}

	if exception {
		panic(newException(tryCatchValue))
	}

	return tryCatchValue
}

func (self *_runtime) cmpl_evaluate_nodeWhileStatement(node *_nodeWhileStatement) Value {

	test := node.test
	body := node.body
	labels := append(self.labels, "")
	self.labels = nil

	result := emptyValue
resultBreakContinue:
	for {
		if !self.cmpl_evaluate_nodeExpression(test).resolve().bool() {
			// Stahp: while (false) ...
			break
		}
		for _, node := range body {
			value := self.cmpl_evaluate_nodeStatement(node)
			switch value.kind {
			case valueResult:
				switch value.evaluateBreakContinue(labels) {
				case resultReturn:
					return value
				case resultBreak:
					break resultBreakContinue
				case resultContinue:
					continue resultBreakContinue
				}
			case valueEmpty:
			default:
				result = value
			}
		}
	}
	return result
}

func (self *_runtime) cmpl_evaluate_nodeWithStatement(node *_nodeWithStatement) Value {
	object := self.cmpl_evaluate_nodeExpression(node.object)
	outer := self.scope.lexical
	lexical := self.newObjectStash(self.toObject(object.resolve()), outer)
	self.scope.lexical = lexical
	defer func() {
		self.scope.lexical = outer
	}()

	return self.cmpl_evaluate_nodeStatement(node.body)
}
