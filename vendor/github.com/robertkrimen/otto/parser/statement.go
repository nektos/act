package parser

import (
	"github.com/robertkrimen/otto/ast"
	"github.com/robertkrimen/otto/token"
)

func (self *_parser) parseBlockStatement() *ast.BlockStatement {
	node := &ast.BlockStatement{}

	// Find comments before the leading brace
	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(node, self.comments.FetchAll(), ast.LEADING)
		self.comments.Unset()
	}

	node.LeftBrace = self.expect(token.LEFT_BRACE)
	node.List = self.parseStatementList()

	if self.mode&StoreComments != 0 {
		self.comments.Unset()
		self.comments.CommentMap.AddComments(node, self.comments.FetchAll(), ast.FINAL)
		self.comments.AfterBlock()
	}

	node.RightBrace = self.expect(token.RIGHT_BRACE)

	// Find comments after the trailing brace
	if self.mode&StoreComments != 0 {
		self.comments.ResetLineBreak()
		self.comments.CommentMap.AddComments(node, self.comments.Fetch(), ast.TRAILING)
	}

	return node
}

func (self *_parser) parseEmptyStatement() ast.Statement {
	idx := self.expect(token.SEMICOLON)
	return &ast.EmptyStatement{Semicolon: idx}
}

func (self *_parser) parseStatementList() (list []ast.Statement) {
	for self.token != token.RIGHT_BRACE && self.token != token.EOF {
		statement := self.parseStatement()
		list = append(list, statement)
	}

	return
}

func (self *_parser) parseStatement() ast.Statement {

	if self.token == token.EOF {
		self.errorUnexpectedToken(self.token)
		return &ast.BadStatement{From: self.idx, To: self.idx + 1}
	}

	if self.mode&StoreComments != 0 {
		self.comments.ResetLineBreak()
	}

	switch self.token {
	case token.SEMICOLON:
		return self.parseEmptyStatement()
	case token.LEFT_BRACE:
		return self.parseBlockStatement()
	case token.IF:
		return self.parseIfStatement()
	case token.DO:
		statement := self.parseDoWhileStatement()
		self.comments.PostProcessNode(statement)
		return statement
	case token.WHILE:
		return self.parseWhileStatement()
	case token.FOR:
		return self.parseForOrForInStatement()
	case token.BREAK:
		return self.parseBreakStatement()
	case token.CONTINUE:
		return self.parseContinueStatement()
	case token.DEBUGGER:
		return self.parseDebuggerStatement()
	case token.WITH:
		return self.parseWithStatement()
	case token.VAR:
		return self.parseVariableStatement()
	case token.FUNCTION:
		return self.parseFunctionStatement()
	case token.SWITCH:
		return self.parseSwitchStatement()
	case token.RETURN:
		return self.parseReturnStatement()
	case token.THROW:
		return self.parseThrowStatement()
	case token.TRY:
		return self.parseTryStatement()
	}

	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
	}

	expression := self.parseExpression()

	if identifier, isIdentifier := expression.(*ast.Identifier); isIdentifier && self.token == token.COLON {
		// LabelledStatement
		colon := self.idx
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next() // :

		label := identifier.Name
		for _, value := range self.scope.labels {
			if label == value {
				self.error(identifier.Idx0(), "Label '%s' already exists", label)
			}
		}
		var labelComments []*ast.Comment
		if self.mode&StoreComments != 0 {
			labelComments = self.comments.FetchAll()
		}
		self.scope.labels = append(self.scope.labels, label) // Push the label
		statement := self.parseStatement()
		self.scope.labels = self.scope.labels[:len(self.scope.labels)-1] // Pop the label
		exp := &ast.LabelledStatement{
			Label:     identifier,
			Colon:     colon,
			Statement: statement,
		}
		if self.mode&StoreComments != 0 {
			self.comments.CommentMap.AddComments(exp, labelComments, ast.LEADING)
		}

		return exp
	}

	self.optionalSemicolon()

	statement := &ast.ExpressionStatement{
		Expression: expression,
	}

	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(statement, comments, ast.LEADING)
	}
	return statement
}

func (self *_parser) parseTryStatement() ast.Statement {
	var tryComments []*ast.Comment
	if self.mode&StoreComments != 0 {
		tryComments = self.comments.FetchAll()
	}
	node := &ast.TryStatement{
		Try:  self.expect(token.TRY),
		Body: self.parseBlockStatement(),
	}
	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(node, tryComments, ast.LEADING)
		self.comments.CommentMap.AddComments(node.Body, self.comments.FetchAll(), ast.TRAILING)
	}

	if self.token == token.CATCH {
		catch := self.idx
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()
		self.expect(token.LEFT_PARENTHESIS)
		if self.token != token.IDENTIFIER {
			self.expect(token.IDENTIFIER)
			self.nextStatement()
			return &ast.BadStatement{From: catch, To: self.idx}
		} else {
			identifier := self.parseIdentifier()
			self.expect(token.RIGHT_PARENTHESIS)
			node.Catch = &ast.CatchStatement{
				Catch:     catch,
				Parameter: identifier,
				Body:      self.parseBlockStatement(),
			}

			if self.mode&StoreComments != 0 {
				self.comments.CommentMap.AddComments(node.Catch.Body, self.comments.FetchAll(), ast.TRAILING)
			}
		}
	}

	if self.token == token.FINALLY {
		if self.mode&StoreComments != 0 {
			self.comments.Unset()
		}
		self.next()
		if self.mode&StoreComments != 0 {
			tryComments = self.comments.FetchAll()
		}

		node.Finally = self.parseBlockStatement()

		if self.mode&StoreComments != 0 {
			self.comments.CommentMap.AddComments(node.Finally, tryComments, ast.LEADING)
		}
	}

	if node.Catch == nil && node.Finally == nil {
		self.error(node.Try, "Missing catch or finally after try")
		return &ast.BadStatement{From: node.Try, To: node.Body.Idx1()}
	}

	return node
}

func (self *_parser) parseFunctionParameterList() *ast.ParameterList {
	opening := self.expect(token.LEFT_PARENTHESIS)
	if self.mode&StoreComments != 0 {
		self.comments.Unset()
	}
	var list []*ast.Identifier
	for self.token != token.RIGHT_PARENTHESIS && self.token != token.EOF {
		if self.token != token.IDENTIFIER {
			self.expect(token.IDENTIFIER)
		} else {
			identifier := self.parseIdentifier()
			list = append(list, identifier)
		}
		if self.token != token.RIGHT_PARENTHESIS {
			if self.mode&StoreComments != 0 {
				self.comments.Unset()
			}
			self.expect(token.COMMA)
		}
	}
	closing := self.expect(token.RIGHT_PARENTHESIS)

	return &ast.ParameterList{
		Opening: opening,
		List:    list,
		Closing: closing,
	}
}

func (self *_parser) parseParameterList() (list []string) {
	for self.token != token.EOF {
		if self.token != token.IDENTIFIER {
			self.expect(token.IDENTIFIER)
		}
		list = append(list, self.literal)
		self.next()
		if self.token != token.EOF {
			self.expect(token.COMMA)
		}
	}
	return
}

func (self *_parser) parseFunctionStatement() *ast.FunctionStatement {
	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
	}
	function := &ast.FunctionStatement{
		Function: self.parseFunction(true),
	}
	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(function, comments, ast.LEADING)
	}

	return function
}

func (self *_parser) parseFunction(declaration bool) *ast.FunctionLiteral {

	node := &ast.FunctionLiteral{
		Function: self.expect(token.FUNCTION),
	}

	var name *ast.Identifier
	if self.token == token.IDENTIFIER {
		name = self.parseIdentifier()
		if declaration {
			self.scope.declare(&ast.FunctionDeclaration{
				Function: node,
			})
		}
	} else if declaration {
		// Use expect error handling
		self.expect(token.IDENTIFIER)
	}
	if self.mode&StoreComments != 0 {
		self.comments.Unset()
	}
	node.Name = name
	node.ParameterList = self.parseFunctionParameterList()
	self.parseFunctionBlock(node)
	node.Source = self.slice(node.Idx0(), node.Idx1())

	return node
}

func (self *_parser) parseFunctionBlock(node *ast.FunctionLiteral) {
	{
		self.openScope()
		inFunction := self.scope.inFunction
		self.scope.inFunction = true
		defer func() {
			self.scope.inFunction = inFunction
			self.closeScope()
		}()
		node.Body = self.parseBlockStatement()
		node.DeclarationList = self.scope.declarationList
	}
}

func (self *_parser) parseDebuggerStatement() ast.Statement {
	idx := self.expect(token.DEBUGGER)

	node := &ast.DebuggerStatement{
		Debugger: idx,
	}
	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(node, self.comments.FetchAll(), ast.TRAILING)
	}

	self.semicolon()
	return node
}

func (self *_parser) parseReturnStatement() ast.Statement {
	idx := self.expect(token.RETURN)
	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
	}

	if !self.scope.inFunction {
		self.error(idx, "Illegal return statement")
		self.nextStatement()
		return &ast.BadStatement{From: idx, To: self.idx}
	}

	node := &ast.ReturnStatement{
		Return: idx,
	}

	if !self.implicitSemicolon && self.token != token.SEMICOLON && self.token != token.RIGHT_BRACE && self.token != token.EOF {
		node.Argument = self.parseExpression()
	}
	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(node, comments, ast.LEADING)
	}

	self.semicolon()

	return node
}

func (self *_parser) parseThrowStatement() ast.Statement {
	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
	}
	idx := self.expect(token.THROW)

	if self.implicitSemicolon {
		if self.chr == -1 { // Hackish
			self.error(idx, "Unexpected end of input")
		} else {
			self.error(idx, "Illegal newline after throw")
		}
		self.nextStatement()
		return &ast.BadStatement{From: idx, To: self.idx}
	}

	node := &ast.ThrowStatement{
		Throw:    self.idx,
		Argument: self.parseExpression(),
	}
	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(node, comments, ast.LEADING)
	}

	self.semicolon()

	return node
}

func (self *_parser) parseSwitchStatement() ast.Statement {
	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
	}
	self.expect(token.SWITCH)
	if self.mode&StoreComments != 0 {
		comments = append(comments, self.comments.FetchAll()...)
	}
	self.expect(token.LEFT_PARENTHESIS)
	node := &ast.SwitchStatement{
		Discriminant: self.parseExpression(),
		Default:      -1,
	}
	self.expect(token.RIGHT_PARENTHESIS)
	if self.mode&StoreComments != 0 {
		comments = append(comments, self.comments.FetchAll()...)
	}

	self.expect(token.LEFT_BRACE)

	inSwitch := self.scope.inSwitch
	self.scope.inSwitch = true
	defer func() {
		self.scope.inSwitch = inSwitch
	}()

	for index := 0; self.token != token.EOF; index++ {
		if self.token == token.RIGHT_BRACE {
			self.next()
			break
		}

		clause := self.parseCaseStatement()
		if clause.Test == nil {
			if node.Default != -1 {
				self.error(clause.Case, "Already saw a default in switch")
			}
			node.Default = index
		}
		node.Body = append(node.Body, clause)
	}

	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(node, comments, ast.LEADING)
	}

	return node
}

func (self *_parser) parseWithStatement() ast.Statement {
	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
	}
	self.expect(token.WITH)
	var withComments []*ast.Comment
	if self.mode&StoreComments != 0 {
		withComments = self.comments.FetchAll()
	}

	self.expect(token.LEFT_PARENTHESIS)

	node := &ast.WithStatement{
		Object: self.parseExpression(),
	}
	self.expect(token.RIGHT_PARENTHESIS)

	if self.mode&StoreComments != 0 {
		//comments = append(comments, self.comments.FetchAll()...)
		self.comments.CommentMap.AddComments(node, comments, ast.LEADING)
		self.comments.CommentMap.AddComments(node, withComments, ast.WITH)
	}

	node.Body = self.parseStatement()

	return node
}

func (self *_parser) parseCaseStatement() *ast.CaseStatement {
	node := &ast.CaseStatement{
		Case: self.idx,
	}

	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
		self.comments.Unset()
	}

	if self.token == token.DEFAULT {
		self.next()
	} else {
		self.expect(token.CASE)
		node.Test = self.parseExpression()
	}

	if self.mode&StoreComments != 0 {
		self.comments.Unset()
	}
	self.expect(token.COLON)

	for {
		if self.token == token.EOF ||
			self.token == token.RIGHT_BRACE ||
			self.token == token.CASE ||
			self.token == token.DEFAULT {
			break
		}
		consequent := self.parseStatement()
		node.Consequent = append(node.Consequent, consequent)
	}

	// Link the comments to the case statement
	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(node, comments, ast.LEADING)
	}

	return node
}

func (self *_parser) parseIterationStatement() ast.Statement {
	inIteration := self.scope.inIteration
	self.scope.inIteration = true
	defer func() {
		self.scope.inIteration = inIteration
	}()
	return self.parseStatement()
}

func (self *_parser) parseForIn(into ast.Expression) *ast.ForInStatement {

	// Already have consumed "<into> in"

	source := self.parseExpression()
	self.expect(token.RIGHT_PARENTHESIS)
	body := self.parseIterationStatement()

	forin := &ast.ForInStatement{
		Into:   into,
		Source: source,
		Body:   body,
	}

	return forin
}

func (self *_parser) parseFor(initializer ast.Expression) *ast.ForStatement {

	// Already have consumed "<initializer> ;"

	var test, update ast.Expression

	if self.token != token.SEMICOLON {
		test = self.parseExpression()
	}
	if self.mode&StoreComments != 0 {
		self.comments.Unset()
	}
	self.expect(token.SEMICOLON)

	if self.token != token.RIGHT_PARENTHESIS {
		update = self.parseExpression()
	}
	self.expect(token.RIGHT_PARENTHESIS)
	body := self.parseIterationStatement()

	forstatement := &ast.ForStatement{
		Initializer: initializer,
		Test:        test,
		Update:      update,
		Body:        body,
	}

	return forstatement
}

func (self *_parser) parseForOrForInStatement() ast.Statement {
	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
	}
	idx := self.expect(token.FOR)
	var forComments []*ast.Comment
	if self.mode&StoreComments != 0 {
		forComments = self.comments.FetchAll()
	}
	self.expect(token.LEFT_PARENTHESIS)

	var left []ast.Expression

	forIn := false
	if self.token != token.SEMICOLON {

		allowIn := self.scope.allowIn
		self.scope.allowIn = false
		if self.token == token.VAR {
			var_ := self.idx
			var varComments []*ast.Comment
			if self.mode&StoreComments != 0 {
				varComments = self.comments.FetchAll()
				self.comments.Unset()
			}
			self.next()
			list := self.parseVariableDeclarationList(var_)
			if len(list) == 1 && self.token == token.IN {
				if self.mode&StoreComments != 0 {
					self.comments.Unset()
				}
				self.next() // in
				forIn = true
				left = []ast.Expression{list[0]} // There is only one declaration
			} else {
				left = list
			}
			if self.mode&StoreComments != 0 {
				self.comments.CommentMap.AddComments(left[0], varComments, ast.LEADING)
			}
		} else {
			left = append(left, self.parseExpression())
			if self.token == token.IN {
				self.next()
				forIn = true
			}
		}
		self.scope.allowIn = allowIn
	}

	if forIn {
		switch left[0].(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression, *ast.VariableExpression:
			// These are all acceptable
		default:
			self.error(idx, "Invalid left-hand side in for-in")
			self.nextStatement()
			return &ast.BadStatement{From: idx, To: self.idx}
		}

		forin := self.parseForIn(left[0])
		if self.mode&StoreComments != 0 {
			self.comments.CommentMap.AddComments(forin, comments, ast.LEADING)
			self.comments.CommentMap.AddComments(forin, forComments, ast.FOR)
		}
		return forin
	}

	if self.mode&StoreComments != 0 {
		self.comments.Unset()
	}
	self.expect(token.SEMICOLON)
	initializer := &ast.SequenceExpression{Sequence: left}
	forstatement := self.parseFor(initializer)
	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(forstatement, comments, ast.LEADING)
		self.comments.CommentMap.AddComments(forstatement, forComments, ast.FOR)
	}
	return forstatement
}

func (self *_parser) parseVariableStatement() *ast.VariableStatement {
	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
	}
	idx := self.expect(token.VAR)

	list := self.parseVariableDeclarationList(idx)

	statement := &ast.VariableStatement{
		Var:  idx,
		List: list,
	}
	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(statement, comments, ast.LEADING)
		self.comments.Unset()
	}
	self.semicolon()

	return statement
}

func (self *_parser) parseDoWhileStatement() ast.Statement {
	inIteration := self.scope.inIteration
	self.scope.inIteration = true
	defer func() {
		self.scope.inIteration = inIteration
	}()

	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
	}
	self.expect(token.DO)
	var doComments []*ast.Comment
	if self.mode&StoreComments != 0 {
		doComments = self.comments.FetchAll()
	}

	node := &ast.DoWhileStatement{}
	if self.token == token.LEFT_BRACE {
		node.Body = self.parseBlockStatement()
	} else {
		node.Body = self.parseStatement()
	}

	self.expect(token.WHILE)
	var whileComments []*ast.Comment
	if self.mode&StoreComments != 0 {
		whileComments = self.comments.FetchAll()
	}
	self.expect(token.LEFT_PARENTHESIS)
	node.Test = self.parseExpression()
	self.expect(token.RIGHT_PARENTHESIS)

	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(node, comments, ast.LEADING)
		self.comments.CommentMap.AddComments(node, doComments, ast.DO)
		self.comments.CommentMap.AddComments(node, whileComments, ast.WHILE)
	}

	return node
}

func (self *_parser) parseWhileStatement() ast.Statement {
	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
	}
	self.expect(token.WHILE)

	var whileComments []*ast.Comment
	if self.mode&StoreComments != 0 {
		whileComments = self.comments.FetchAll()
	}

	self.expect(token.LEFT_PARENTHESIS)
	node := &ast.WhileStatement{
		Test: self.parseExpression(),
	}
	self.expect(token.RIGHT_PARENTHESIS)
	node.Body = self.parseIterationStatement()

	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(node, comments, ast.LEADING)
		self.comments.CommentMap.AddComments(node, whileComments, ast.WHILE)
	}

	return node
}

func (self *_parser) parseIfStatement() ast.Statement {
	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
	}
	self.expect(token.IF)
	var ifComments []*ast.Comment
	if self.mode&StoreComments != 0 {
		ifComments = self.comments.FetchAll()
	}

	self.expect(token.LEFT_PARENTHESIS)
	node := &ast.IfStatement{
		If:   self.idx,
		Test: self.parseExpression(),
	}
	self.expect(token.RIGHT_PARENTHESIS)
	if self.token == token.LEFT_BRACE {
		node.Consequent = self.parseBlockStatement()
	} else {
		node.Consequent = self.parseStatement()
	}

	if self.token == token.ELSE {
		self.next()
		node.Alternate = self.parseStatement()
	}

	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(node, comments, ast.LEADING)
		self.comments.CommentMap.AddComments(node, ifComments, ast.IF)
	}

	return node
}

func (self *_parser) parseSourceElement() ast.Statement {
	statement := self.parseStatement()
	//self.comments.Unset()
	return statement
}

func (self *_parser) parseSourceElements() []ast.Statement {
	body := []ast.Statement(nil)

	for {
		if self.token != token.STRING {
			break
		}
		body = append(body, self.parseSourceElement())
	}

	for self.token != token.EOF {
		body = append(body, self.parseSourceElement())
	}

	return body
}

func (self *_parser) parseProgram() *ast.Program {
	self.openScope()
	defer self.closeScope()
	return &ast.Program{
		Body:            self.parseSourceElements(),
		DeclarationList: self.scope.declarationList,
		File:            self.file,
	}
}

func (self *_parser) parseBreakStatement() ast.Statement {
	var comments []*ast.Comment
	if self.mode&StoreComments != 0 {
		comments = self.comments.FetchAll()
	}
	idx := self.expect(token.BREAK)
	semicolon := self.implicitSemicolon
	if self.token == token.SEMICOLON {
		semicolon = true
		self.next()
	}

	if semicolon || self.token == token.RIGHT_BRACE {
		self.implicitSemicolon = false
		if !self.scope.inIteration && !self.scope.inSwitch {
			goto illegal
		}
		breakStatement := &ast.BranchStatement{
			Idx:   idx,
			Token: token.BREAK,
		}

		if self.mode&StoreComments != 0 {
			self.comments.CommentMap.AddComments(breakStatement, comments, ast.LEADING)
			self.comments.CommentMap.AddComments(breakStatement, self.comments.FetchAll(), ast.TRAILING)
		}

		return breakStatement
	}

	if self.token == token.IDENTIFIER {
		identifier := self.parseIdentifier()
		if !self.scope.hasLabel(identifier.Name) {
			self.error(idx, "Undefined label '%s'", identifier.Name)
			return &ast.BadStatement{From: idx, To: identifier.Idx1()}
		}
		self.semicolon()
		breakStatement := &ast.BranchStatement{
			Idx:   idx,
			Token: token.BREAK,
			Label: identifier,
		}
		if self.mode&StoreComments != 0 {
			self.comments.CommentMap.AddComments(breakStatement, comments, ast.LEADING)
		}

		return breakStatement
	}

	self.expect(token.IDENTIFIER)

illegal:
	self.error(idx, "Illegal break statement")
	self.nextStatement()
	return &ast.BadStatement{From: idx, To: self.idx}
}

func (self *_parser) parseContinueStatement() ast.Statement {
	idx := self.expect(token.CONTINUE)
	semicolon := self.implicitSemicolon
	if self.token == token.SEMICOLON {
		semicolon = true
		self.next()
	}

	if semicolon || self.token == token.RIGHT_BRACE {
		self.implicitSemicolon = false
		if !self.scope.inIteration {
			goto illegal
		}
		return &ast.BranchStatement{
			Idx:   idx,
			Token: token.CONTINUE,
		}
	}

	if self.token == token.IDENTIFIER {
		identifier := self.parseIdentifier()
		if !self.scope.hasLabel(identifier.Name) {
			self.error(idx, "Undefined label '%s'", identifier.Name)
			return &ast.BadStatement{From: idx, To: identifier.Idx1()}
		}
		if !self.scope.inIteration {
			goto illegal
		}
		self.semicolon()
		return &ast.BranchStatement{
			Idx:   idx,
			Token: token.CONTINUE,
			Label: identifier,
		}
	}

	self.expect(token.IDENTIFIER)

illegal:
	self.error(idx, "Illegal continue statement")
	self.nextStatement()
	return &ast.BadStatement{From: idx, To: self.idx}
}

// Find the next statement after an error (recover)
func (self *_parser) nextStatement() {
	for {
		switch self.token {
		case token.BREAK, token.CONTINUE,
			token.FOR, token.IF, token.RETURN, token.SWITCH,
			token.VAR, token.DO, token.TRY, token.WITH,
			token.WHILE, token.THROW, token.CATCH, token.FINALLY:
			// Return only if parser made some progress since last
			// sync or if it has not reached 10 next calls without
			// progress. Otherwise consume at least one token to
			// avoid an endless parser loop
			if self.idx == self.recover.idx && self.recover.count < 10 {
				self.recover.count++
				return
			}
			if self.idx > self.recover.idx {
				self.recover.idx = self.idx
				self.recover.count = 0
				return
			}
			// Reaching here indicates a parser bug, likely an
			// incorrect token list in this function, but it only
			// leads to skipping of possibly correct code if a
			// previous error is present, and thus is preferred
			// over a non-terminating parse.
		case token.EOF:
			return
		}
		self.next()
	}
}
