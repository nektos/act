package parser

import (
	"regexp"

	"github.com/robertkrimen/otto/ast"
	"github.com/robertkrimen/otto/file"
	"github.com/robertkrimen/otto/token"
)

func (self *_parser) parseIdentifier() *ast.Identifier {
	literal := self.literal
	idx := self.idx
	if self.mode&StoreComments != 0 {
		self.comments.MarkComments(ast.LEADING)
	}
	self.next()
	exp := &ast.Identifier{
		Name: literal,
		Idx:  idx,
	}

	if self.mode&StoreComments != 0 {
		self.comments.SetExpression(exp)
	}

	return exp
}

func (self *_parser) parsePrimaryExpression() ast.Expression {
	literal := self.literal
	idx := self.idx
	switch self.token {
	case token.IDENTIFIER:
		self.next()
		if len(literal) > 1 {
			tkn, strict := token.IsKeyword(literal)
			if tkn == token.KEYWORD {
				if !strict {
					self.error(idx, "Unexpected reserved word")
				}
			}
		}
		return &ast.Identifier{
			Name: literal,
			Idx:  idx,
		}
	case token.NULL:
		self.next()
		return &ast.NullLiteral{
			Idx:     idx,
			Literal: literal,
		}
	case token.BOOLEAN:
		self.next()
		value := false
		switch literal {
		case "true":
			value = true
		case "false":
			value = false
		default:
			self.error(idx, "Illegal boolean literal")
		}
		return &ast.BooleanLiteral{
			Idx:     idx,
			Literal: literal,
			Value:   value,
		}
	case token.STRING:
		self.next()
		value, err := parseStringLiteral(literal[1 : len(literal)-1])
		if err != nil {
			self.error(idx, err.Error())
		}
		return &ast.StringLiteral{
			Idx:     idx,
			Literal: literal,
			Value:   value,
		}
	case token.NUMBER:
		self.next()
		value, err := parseNumberLiteral(literal)
		if err != nil {
			self.error(idx, err.Error())
			value = 0
		}
		return &ast.NumberLiteral{
			Idx:     idx,
			Literal: literal,
			Value:   value,
		}
	case token.SLASH, token.QUOTIENT_ASSIGN:
		return self.parseRegExpLiteral()
	case token.LEFT_BRACE:
		return self.parseObjectLiteral()
	case token.LEFT_BRACKET:
		return self.parseArrayLiteral()
	case token.LEFT_PARENTHESIS:
		self.expect(token.LEFT_PARENTHESIS)
		expression := self.parseExpression()
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.expect(token.RIGHT_PARENTHESIS)
		return expression
	case token.THIS:
		self.next()
		return &ast.ThisExpression{
			Idx: idx,
		}
	case token.FUNCTION:
		return self.parseFunction(false)
	}

	self.errorUnexpectedToken(self.token)
	self.nextStatement()
	return &ast.BadExpression{From: idx, To: self.idx}
}

func (self *_parser) parseRegExpLiteral() *ast.RegExpLiteral {

	offset := self.chrOffset - 1 // Opening slash already gotten
	if self.token == token.QUOTIENT_ASSIGN {
		offset -= 1 // =
	}
	idx := self.idxOf(offset)

	pattern, err := self.scanString(offset)
	endOffset := self.chrOffset

	self.next()
	if err == nil {
		pattern = pattern[1 : len(pattern)-1]
	}

	flags := ""
	if self.token == token.IDENTIFIER { // gim

		flags = self.literal
		self.next()
		endOffset = self.chrOffset - 1
	}

	var value string
	// TODO 15.10
	{
		// Test during parsing that this is a valid regular expression
		// Sorry, (?=) and (?!) are invalid (for now)
		pattern, err := TransformRegExp(pattern)
		if err != nil {
			if pattern == "" || self.mode&IgnoreRegExpErrors == 0 {
				self.error(idx, "Invalid regular expression: %s", err.Error())
			}
		} else {
			_, err = regexp.Compile(pattern)
			if err != nil {
				// We should not get here, ParseRegExp should catch any errors
				self.error(idx, "Invalid regular expression: %s", err.Error()[22:]) // Skip redundant "parse regexp error"
			} else {
				value = pattern
			}
		}
	}

	literal := self.str[offset:endOffset]

	return &ast.RegExpLiteral{
		Idx:     idx,
		Literal: literal,
		Pattern: pattern,
		Flags:   flags,
		Value:   value,
	}
}

func (self *_parser) parseVariableDeclaration(declarationList *[]*ast.VariableExpression) ast.Expression {

	if self.token != token.IDENTIFIER {
		idx := self.expect(token.IDENTIFIER)
		self.nextStatement()
		return &ast.BadExpression{From: idx, To: self.idx}
	}

	literal := self.literal
	idx := self.idx
	self.next()
	node := &ast.VariableExpression{
		Name: literal,
		Idx:  idx,
	}
	if self.mode&StoreComments != 0 {
		self.comments.SetExpression(node)
	}

	if declarationList != nil {
		*declarationList = append(*declarationList, node)
	}

	if self.token == token.ASSIGN {
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()
		node.Initializer = self.parseAssignmentExpression()
	}

	return node
}

func (self *_parser) parseVariableDeclarationList(var_ file.Idx) []ast.Expression {

	var declarationList []*ast.VariableExpression // Avoid bad expressions
	var list []ast.Expression

	for {
		if self.mode&StoreComments != 0 {
			self.comments.MarkComments(ast.LEADING)
		}
		decl := self.parseVariableDeclaration(&declarationList)
		list = append(list, decl)
		if self.token != token.COMMA {
			break
		}
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()
	}

	self.scope.declare(&ast.VariableDeclaration{
		Var:  var_,
		List: declarationList,
	})

	return list
}

func (self *_parser) parseObjectPropertyKey() (string, string) {
	idx, tkn, literal := self.idx, self.token, self.literal
	value := ""
	if self.mode&StoreComments != 0 {
		self.comments.MarkComments(ast.KEY)
	}
	self.next()

	switch tkn {
	case token.IDENTIFIER:
		value = literal
	case token.NUMBER:
		var err error
		_, err = parseNumberLiteral(literal)
		if err != nil {
			self.error(idx, err.Error())
		} else {
			value = literal
		}
	case token.STRING:
		var err error
		value, err = parseStringLiteral(literal[1 : len(literal)-1])
		if err != nil {
			self.error(idx, err.Error())
		}
	default:
		// null, false, class, etc.
		if matchIdentifier.MatchString(literal) {
			value = literal
		}
	}
	return literal, value
}

func (self *_parser) parseObjectProperty() ast.Property {
	literal, value := self.parseObjectPropertyKey()
	if literal == "get" && self.token != token.COLON {
		idx := self.idx
		_, value := self.parseObjectPropertyKey()
		parameterList := self.parseFunctionParameterList()

		node := &ast.FunctionLiteral{
			Function:      idx,
			ParameterList: parameterList,
		}
		self.parseFunctionBlock(node)
		return ast.Property{
			Key:   value,
			Kind:  "get",
			Value: node,
		}
	} else if literal == "set" && self.token != token.COLON {
		idx := self.idx
		_, value := self.parseObjectPropertyKey()
		parameterList := self.parseFunctionParameterList()

		node := &ast.FunctionLiteral{
			Function:      idx,
			ParameterList: parameterList,
		}
		self.parseFunctionBlock(node)
		return ast.Property{
			Key:   value,
			Kind:  "set",
			Value: node,
		}
	}

	if self.mode&StoreComments != 0 {
		self.comments.MarkComments(ast.COLON)
	}
	self.expect(token.COLON)

	exp := ast.Property{
		Key:   value,
		Kind:  "value",
		Value: self.parseAssignmentExpression(),
	}

	if self.mode&StoreComments != 0 {
		self.comments.SetExpression(exp.Value)
	}
	return exp
}

func (self *_parser) parseObjectLiteral() ast.Expression {
	var value []ast.Property
	idx0 := self.expect(token.LEFT_BRACE)
	for self.token != token.RIGHT_BRACE && self.token != token.EOF {
		value = append(value, self.parseObjectProperty())
		if self.token == token.COMMA {
			if self.mode&StoreComments != 0 {
				self.comments.Unset()
			}
			self.next()
			continue
		}
	}
	if self.mode&StoreComments != 0 {
		self.comments.MarkComments(ast.FINAL)
	}
	idx1 := self.expect(token.RIGHT_BRACE)

	return &ast.ObjectLiteral{
		LeftBrace:  idx0,
		RightBrace: idx1,
		Value:      value,
	}
}

func (self *_parser) parseArrayLiteral() ast.Expression {
	idx0 := self.expect(token.LEFT_BRACKET)
	var value []ast.Expression
	for self.token != token.RIGHT_BRACKET && self.token != token.EOF {
		if self.token == token.COMMA {
			// This kind of comment requires a special empty expression node.
			empty := &ast.EmptyExpression{self.idx, self.idx}

			if self.mode&StoreComments != 0 {
				self.comments.SetExpression(empty)
				self.comments.Unset()
			}
			value = append(value, empty)
			self.next()
			continue
		}

		exp := self.parseAssignmentExpression()

		value = append(value, exp)
		if self.token != token.RIGHT_BRACKET {
			if self.mode&StoreComments != 0 {
				self.comments.Unset()
			}
			self.expect(token.COMMA)
		}
	}
	if self.mode&StoreComments != 0 {
		self.comments.MarkComments(ast.FINAL)
	}
	idx1 := self.expect(token.RIGHT_BRACKET)

	return &ast.ArrayLiteral{
		LeftBracket:  idx0,
		RightBracket: idx1,
		Value:        value,
	}
}

func (self *_parser) parseArgumentList() (argumentList []ast.Expression, idx0, idx1 file.Idx) {
	if self.mode&StoreComments != 0 {
		self.comments.Unset()
	}
	idx0 = self.expect(token.LEFT_PARENTHESIS)
	if self.token != token.RIGHT_PARENTHESIS {
		for {
			exp := self.parseAssignmentExpression()
			if self.mode&StoreComments != 0 {
				self.comments.SetExpression(exp)
			}
			argumentList = append(argumentList, exp)
			if self.token != token.COMMA {
				break
			}
			if self.mode&StoreComments != 0 {
				self.comments.Unset()
			}
			self.next()
		}
	}
	if self.mode&StoreComments != 0 {
		self.comments.Unset()
	}
	idx1 = self.expect(token.RIGHT_PARENTHESIS)
	return
}

func (self *_parser) parseCallExpression(left ast.Expression) ast.Expression {
	argumentList, idx0, idx1 := self.parseArgumentList()
	exp := &ast.CallExpression{
		Callee:           left,
		LeftParenthesis:  idx0,
		ArgumentList:     argumentList,
		RightParenthesis: idx1,
	}

	if self.mode&StoreComments != 0 {
		self.comments.SetExpression(exp)
	}
	return exp
}

func (self *_parser) parseDotMember(left ast.Expression) ast.Expression {
	period := self.expect(token.PERIOD)

	literal := self.literal
	idx := self.idx

	if !matchIdentifier.MatchString(literal) {
		self.expect(token.IDENTIFIER)
		self.nextStatement()
		return &ast.BadExpression{From: period, To: self.idx}
	}

	self.next()

	return &ast.DotExpression{
		Left: left,
		Identifier: &ast.Identifier{
			Idx:  idx,
			Name: literal,
		},
	}
}

func (self *_parser) parseBracketMember(left ast.Expression) ast.Expression {
	idx0 := self.expect(token.LEFT_BRACKET)
	member := self.parseExpression()
	idx1 := self.expect(token.RIGHT_BRACKET)
	return &ast.BracketExpression{
		LeftBracket:  idx0,
		Left:         left,
		Member:       member,
		RightBracket: idx1,
	}
}

func (self *_parser) parseNewExpression() ast.Expression {
	idx := self.expect(token.NEW)
	callee := self.parseLeftHandSideExpression()
	node := &ast.NewExpression{
		New:    idx,
		Callee: callee,
	}
	if self.token == token.LEFT_PARENTHESIS {
		argumentList, idx0, idx1 := self.parseArgumentList()
		node.ArgumentList = argumentList
		node.LeftParenthesis = idx0
		node.RightParenthesis = idx1
	}

	if self.mode&StoreComments != 0 {
		self.comments.SetExpression(node)
	}

	return node
}

func (self *_parser) parseLeftHandSideExpression() ast.Expression {

	var left ast.Expression
	if self.token == token.NEW {
		left = self.parseNewExpression()
	} else {
		if self.mode&StoreComments != 0 {
			self.comments.MarkComments(ast.LEADING)
			self.comments.MarkPrimary()
		}
		left = self.parsePrimaryExpression()
	}

	if self.mode&StoreComments != 0 {
		self.comments.SetExpression(left)
	}

	for {
		if self.token == token.PERIOD {
			left = self.parseDotMember(left)
		} else if self.token == token.LEFT_BRACKET {
			left = self.parseBracketMember(left)
		} else {
			break
		}
	}

	return left
}

func (self *_parser) parseLeftHandSideExpressionAllowCall() ast.Expression {

	allowIn := self.scope.allowIn
	self.scope.allowIn = true
	defer func() {
		self.scope.allowIn = allowIn
	}()

	var left ast.Expression
	if self.token == token.NEW {
		var newComments []*ast.Comment
		if self.mode&StoreComments != 0 {
			newComments = self.comments.FetchAll()
			self.comments.MarkComments(ast.LEADING)
			self.comments.MarkPrimary()
		}
		left = self.parseNewExpression()
		if self.mode&StoreComments != 0 {
			self.comments.CommentMap.AddComments(left, newComments, ast.LEADING)
		}
	} else {
		if self.mode&StoreComments != 0 {
			self.comments.MarkComments(ast.LEADING)
			self.comments.MarkPrimary()
		}
		left = self.parsePrimaryExpression()
	}

	if self.mode&StoreComments != 0 {
		self.comments.SetExpression(left)
	}

	for {
		if self.token == token.PERIOD {
			left = self.parseDotMember(left)
		} else if self.token == token.LEFT_BRACKET {
			left = self.parseBracketMember(left)
		} else if self.token == token.LEFT_PARENTHESIS {
			left = self.parseCallExpression(left)
		} else {
			break
		}
	}

	return left
}

func (self *_parser) parsePostfixExpression() ast.Expression {
	operand := self.parseLeftHandSideExpressionAllowCall()

	switch self.token {
	case token.INCREMENT, token.DECREMENT:
		// Make sure there is no line terminator here
		if self.implicitSemicolon {
			break
		}
		tkn := self.token
		idx := self.idx
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()
		switch operand.(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression:
		default:
			self.error(idx, "Invalid left-hand side in assignment")
			self.nextStatement()
			return &ast.BadExpression{From: idx, To: self.idx}
		}
		exp := &ast.UnaryExpression{
			Operator: tkn,
			Idx:      idx,
			Operand:  operand,
			Postfix:  true,
		}

		if self.mode&StoreComments != 0 {
			self.comments.SetExpression(exp)
		}

		return exp
	}

	return operand
}

func (self *_parser) parseUnaryExpression() ast.Expression {

	switch self.token {
	case token.PLUS, token.MINUS, token.NOT, token.BITWISE_NOT:
		fallthrough
	case token.DELETE, token.VOID, token.TYPEOF:
		tkn := self.token
		idx := self.idx
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()

		return &ast.UnaryExpression{
			Operator: tkn,
			Idx:      idx,
			Operand:  self.parseUnaryExpression(),
		}
	case token.INCREMENT, token.DECREMENT:
		tkn := self.token
		idx := self.idx
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()
		operand := self.parseUnaryExpression()
		switch operand.(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression:
		default:
			self.error(idx, "Invalid left-hand side in assignment")
			self.nextStatement()
			return &ast.BadExpression{From: idx, To: self.idx}
		}
		return &ast.UnaryExpression{
			Operator: tkn,
			Idx:      idx,
			Operand:  operand,
		}
	}

	return self.parsePostfixExpression()
}

func (self *_parser) parseMultiplicativeExpression() ast.Expression {
	next := self.parseUnaryExpression
	left := next()

	for self.token == token.MULTIPLY || self.token == token.SLASH ||
		self.token == token.REMAINDER {
		tkn := self.token
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()

		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseAdditiveExpression() ast.Expression {
	next := self.parseMultiplicativeExpression
	left := next()

	for self.token == token.PLUS || self.token == token.MINUS {
		tkn := self.token
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()

		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseShiftExpression() ast.Expression {
	next := self.parseAdditiveExpression
	left := next()

	for self.token == token.SHIFT_LEFT || self.token == token.SHIFT_RIGHT ||
		self.token == token.UNSIGNED_SHIFT_RIGHT {
		tkn := self.token
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()

		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseRelationalExpression() ast.Expression {
	next := self.parseShiftExpression
	left := next()

	allowIn := self.scope.allowIn
	self.scope.allowIn = true
	defer func() {
		self.scope.allowIn = allowIn
	}()

	switch self.token {
	case token.LESS, token.LESS_OR_EQUAL, token.GREATER, token.GREATER_OR_EQUAL:
		tkn := self.token
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()

		exp := &ast.BinaryExpression{
			Operator:   tkn,
			Left:       left,
			Right:      self.parseRelationalExpression(),
			Comparison: true,
		}
		return exp
	case token.INSTANCEOF:
		tkn := self.token
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()

		exp := &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    self.parseRelationalExpression(),
		}
		return exp
	case token.IN:
		if !allowIn {
			return left
		}
		tkn := self.token
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()

		exp := &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    self.parseRelationalExpression(),
		}
		return exp
	}

	return left
}

func (self *_parser) parseEqualityExpression() ast.Expression {
	next := self.parseRelationalExpression
	left := next()

	for self.token == token.EQUAL || self.token == token.NOT_EQUAL ||
		self.token == token.STRICT_EQUAL || self.token == token.STRICT_NOT_EQUAL {
		tkn := self.token
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()

		left = &ast.BinaryExpression{
			Operator:   tkn,
			Left:       left,
			Right:      next(),
			Comparison: true,
		}
	}

	return left
}

func (self *_parser) parseBitwiseAndExpression() ast.Expression {
	next := self.parseEqualityExpression
	left := next()

	for self.token == token.AND {
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		tkn := self.token
		self.next()

		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseBitwiseExclusiveOrExpression() ast.Expression {
	next := self.parseBitwiseAndExpression
	left := next()

	for self.token == token.EXCLUSIVE_OR {
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		tkn := self.token
		self.next()

		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseBitwiseOrExpression() ast.Expression {
	next := self.parseBitwiseExclusiveOrExpression
	left := next()

	for self.token == token.OR {
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		tkn := self.token
		self.next()

		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseLogicalAndExpression() ast.Expression {
	next := self.parseBitwiseOrExpression
	left := next()

	for self.token == token.LOGICAL_AND {
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		tkn := self.token
		self.next()

		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseLogicalOrExpression() ast.Expression {
	next := self.parseLogicalAndExpression
	left := next()

	for self.token == token.LOGICAL_OR {
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		tkn := self.token
		self.next()

		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    next(),
		}
	}

	return left
}

func (self *_parser) parseConditionlExpression() ast.Expression {
	left := self.parseLogicalOrExpression()

	if self.token == token.QUESTION_MARK {
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()

		consequent := self.parseAssignmentExpression()
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.expect(token.COLON)
		exp := &ast.ConditionalExpression{
			Test:       left,
			Consequent: consequent,
			Alternate:  self.parseAssignmentExpression(),
		}

		return exp
	}

	return left
}

func (self *_parser) parseAssignmentExpression() ast.Expression {
	left := self.parseConditionlExpression()
	var operator token.Token
	switch self.token {
	case token.ASSIGN:
		operator = self.token
	case token.ADD_ASSIGN:
		operator = token.PLUS
	case token.SUBTRACT_ASSIGN:
		operator = token.MINUS
	case token.MULTIPLY_ASSIGN:
		operator = token.MULTIPLY
	case token.QUOTIENT_ASSIGN:
		operator = token.SLASH
	case token.REMAINDER_ASSIGN:
		operator = token.REMAINDER
	case token.AND_ASSIGN:
		operator = token.AND
	case token.AND_NOT_ASSIGN:
		operator = token.AND_NOT
	case token.OR_ASSIGN:
		operator = token.OR
	case token.EXCLUSIVE_OR_ASSIGN:
		operator = token.EXCLUSIVE_OR
	case token.SHIFT_LEFT_ASSIGN:
		operator = token.SHIFT_LEFT
	case token.SHIFT_RIGHT_ASSIGN:
		operator = token.SHIFT_RIGHT
	case token.UNSIGNED_SHIFT_RIGHT_ASSIGN:
		operator = token.UNSIGNED_SHIFT_RIGHT
	}

	if operator != 0 {
		idx := self.idx
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()
		switch left.(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression:
		default:
			self.error(left.Idx0(), "Invalid left-hand side in assignment")
			self.nextStatement()
			return &ast.BadExpression{From: idx, To: self.idx}
		}

		exp := &ast.AssignExpression{
			Left:     left,
			Operator: operator,
			Right:    self.parseAssignmentExpression(),
		}

		if self.mode&StoreComments != 0 {
			self.comments.SetExpression(exp)
		}

		return exp
	}

	return left
}

func (self *_parser) parseExpression() ast.Expression {
	next := self.parseAssignmentExpression
	left := next()

	if self.token == token.COMMA {
		sequence := []ast.Expression{left}
		for {
			if self.token != token.COMMA {
				break
			}
			self.next()
			sequence = append(sequence, next())
		}
		return &ast.SequenceExpression{
			Sequence: sequence,
		}
	}

	return left
}
