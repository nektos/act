package otto

import (
	"strconv"
)

func (self *_runtime) cmpl_evaluate_nodeProgram(node *_nodeProgram, eval bool) Value {
	if !eval {
		self.enterGlobalScope()
		defer func() {
			self.leaveScope()
		}()
	}
	self.cmpl_functionDeclaration(node.functionList)
	self.cmpl_variableDeclaration(node.varList)
	self.scope.frame.file = node.file
	return self.cmpl_evaluate_nodeStatementList(node.body)
}

func (self *_runtime) cmpl_call_nodeFunction(function *_object, stash *_fnStash, node *_nodeFunctionLiteral, this Value, argumentList []Value) Value {

	indexOfParameterName := make([]string, len(argumentList))
	// function(abc, def, ghi)
	// indexOfParameterName[0] = "abc"
	// indexOfParameterName[1] = "def"
	// indexOfParameterName[2] = "ghi"
	// ...

	argumentsFound := false
	for index, name := range node.parameterList {
		if name == "arguments" {
			argumentsFound = true
		}
		value := Value{}
		if index < len(argumentList) {
			value = argumentList[index]
			indexOfParameterName[index] = name
		}
		// strict = false
		self.scope.lexical.setValue(name, value, false)
	}

	if !argumentsFound {
		arguments := self.newArgumentsObject(indexOfParameterName, stash, len(argumentList))
		arguments.defineProperty("callee", toValue_object(function), 0101, false)
		stash.arguments = arguments
		// strict = false
		self.scope.lexical.setValue("arguments", toValue_object(arguments), false)
		for index, _ := range argumentList {
			if index < len(node.parameterList) {
				continue
			}
			indexAsString := strconv.FormatInt(int64(index), 10)
			arguments.defineProperty(indexAsString, argumentList[index], 0111, false)
		}
	}

	self.cmpl_functionDeclaration(node.functionList)
	self.cmpl_variableDeclaration(node.varList)

	result := self.cmpl_evaluate_nodeStatement(node.body)
	if result.kind == valueResult {
		return result
	}

	return Value{}
}

func (self *_runtime) cmpl_functionDeclaration(list []*_nodeFunctionLiteral) {
	executionContext := self.scope
	eval := executionContext.eval
	stash := executionContext.variable

	for _, function := range list {
		name := function.name
		value := self.cmpl_evaluate_nodeExpression(function)
		if !stash.hasBinding(name) {
			stash.createBinding(name, eval == true, value)
		} else {
			// TODO 10.5.5.e
			stash.setBinding(name, value, false) // TODO strict
		}
	}
}

func (self *_runtime) cmpl_variableDeclaration(list []string) {
	executionContext := self.scope
	eval := executionContext.eval
	stash := executionContext.variable

	for _, name := range list {
		if !stash.hasBinding(name) {
			stash.createBinding(name, eval == true, Value{}) // TODO strict?
		}
	}
}
