package ast

import (
	"fmt"
	"github.com/robertkrimen/otto/file"
)

// CommentPosition determines where the comment is in a given context
type CommentPosition int

const (
	_        CommentPosition = iota
	LEADING                  // Before the pertinent expression
	TRAILING                 // After the pertinent expression
	KEY                      // Before a key in an object
	COLON                    // After a colon in a field declaration
	FINAL                    // Final comments in a block, not belonging to a specific expression or the comment after a trailing , in an array or object literal
	IF                       // After an if keyword
	WHILE                    // After a while keyword
	DO                       // After do keyword
	FOR                      // After a for keyword
	WITH                     // After a with keyword
	TBD
)

// Comment contains the data of the comment
type Comment struct {
	Begin    file.Idx
	Text     string
	Position CommentPosition
}

// NewComment creates a new comment
func NewComment(text string, idx file.Idx) *Comment {
	comment := &Comment{
		Begin:    idx,
		Text:     text,
		Position: TBD,
	}

	return comment
}

// String returns a stringified version of the position
func (cp CommentPosition) String() string {
	switch cp {
	case LEADING:
		return "Leading"
	case TRAILING:
		return "Trailing"
	case KEY:
		return "Key"
	case COLON:
		return "Colon"
	case FINAL:
		return "Final"
	case IF:
		return "If"
	case WHILE:
		return "While"
	case DO:
		return "Do"
	case FOR:
		return "For"
	case WITH:
		return "With"
	default:
		return "???"
	}
}

// String returns a stringified version of the comment
func (c Comment) String() string {
	return fmt.Sprintf("Comment: %v", c.Text)
}

// Comments defines the current view of comments from the parser
type Comments struct {
	// CommentMap is a reference to the parser comment map
	CommentMap CommentMap
	// Comments lists the comments scanned, not linked to a node yet
	Comments []*Comment
	// future lists the comments after a line break during a sequence of comments
	future []*Comment
	// Current is node for which comments are linked to
	Current Expression

	// wasLineBreak determines if a line break occured while scanning for comments
	wasLineBreak bool
	// primary determines whether or not processing a primary expression
	primary bool
	// afterBlock determines whether or not being after a block statement
	afterBlock bool
}

func NewComments() *Comments {
	comments := &Comments{
		CommentMap: CommentMap{},
	}

	return comments
}

func (c *Comments) String() string {
	return fmt.Sprintf("NODE: %v, Comments: %v, Future: %v(LINEBREAK:%v)", c.Current, len(c.Comments), len(c.future), c.wasLineBreak)
}

// FetchAll returns all the currently scanned comments,
// including those from the next line
func (c *Comments) FetchAll() []*Comment {
	defer func() {
		c.Comments = nil
		c.future = nil
	}()

	return append(c.Comments, c.future...)
}

// Fetch returns all the currently scanned comments
func (c *Comments) Fetch() []*Comment {
	defer func() {
		c.Comments = nil
	}()

	return c.Comments
}

// ResetLineBreak marks the beginning of a new statement
func (c *Comments) ResetLineBreak() {
	c.wasLineBreak = false
}

// MarkPrimary will mark the context as processing a primary expression
func (c *Comments) MarkPrimary() {
	c.primary = true
	c.wasLineBreak = false
}

// AfterBlock will mark the context as being after a block.
func (c *Comments) AfterBlock() {
	c.afterBlock = true
}

// AddComment adds a comment to the view.
// Depending on the context, comments are added normally or as post line break.
func (c *Comments) AddComment(comment *Comment) {
	if c.primary {
		if !c.wasLineBreak {
			c.Comments = append(c.Comments, comment)
		} else {
			c.future = append(c.future, comment)
		}
	} else {
		if !c.wasLineBreak || (c.Current == nil && !c.afterBlock) {
			c.Comments = append(c.Comments, comment)
		} else {
			c.future = append(c.future, comment)
		}
	}
}

// MarkComments will mark the found comments as the given position.
func (c *Comments) MarkComments(position CommentPosition) {
	for _, comment := range c.Comments {
		if comment.Position == TBD {
			comment.Position = position
		}
	}
	for _, c := range c.future {
		if c.Position == TBD {
			c.Position = position
		}
	}
}

// Unset the current node and apply the comments to the current expression.
// Resets context variables.
func (c *Comments) Unset() {
	if c.Current != nil {
		c.applyComments(c.Current, c.Current, TRAILING)
		c.Current = nil
	}
	c.wasLineBreak = false
	c.primary = false
	c.afterBlock = false
}

// SetExpression sets the current expression.
// It is applied the found comments, unless the previous expression has not been unset.
// It is skipped if the node is already set or if it is a part of the previous node.
func (c *Comments) SetExpression(node Expression) {
	// Skipping same node
	if c.Current == node {
		return
	}
	if c.Current != nil && c.Current.Idx1() == node.Idx1() {
		c.Current = node
		return
	}
	previous := c.Current
	c.Current = node

	// Apply the found comments and futures to the node and the previous.
	c.applyComments(node, previous, TRAILING)
}

// PostProcessNode applies all found comments to the given node
func (c *Comments) PostProcessNode(node Node) {
	c.applyComments(node, nil, TRAILING)
}

// applyComments applies both the comments and the future comments to the given node and the previous one,
// based on the context.
func (c *Comments) applyComments(node, previous Node, position CommentPosition) {
	if previous != nil {
		c.CommentMap.AddComments(previous, c.Comments, position)
		c.Comments = nil
	} else {
		c.CommentMap.AddComments(node, c.Comments, position)
		c.Comments = nil
	}
	// Only apply the future comments to the node if the previous is set.
	// This is for detecting end of line comments and which node comments on the following lines belongs to
	if previous != nil {
		c.CommentMap.AddComments(node, c.future, position)
		c.future = nil
	}
}

// AtLineBreak will mark a line break
func (c *Comments) AtLineBreak() {
	c.wasLineBreak = true
}

// CommentMap is the data structure where all found comments are stored
type CommentMap map[Node][]*Comment

// AddComment adds a single comment to the map
func (cm CommentMap) AddComment(node Node, comment *Comment) {
	list := cm[node]
	list = append(list, comment)

	cm[node] = list
}

// AddComments adds a slice of comments, given a node and an updated position
func (cm CommentMap) AddComments(node Node, comments []*Comment, position CommentPosition) {
	for _, comment := range comments {
		if comment.Position == TBD {
			comment.Position = position
		}
		cm.AddComment(node, comment)
	}
}

// Size returns the size of the map
func (cm CommentMap) Size() int {
	size := 0
	for _, comments := range cm {
		size += len(comments)
	}

	return size
}

// MoveComments moves comments with a given position from a node to another
func (cm CommentMap) MoveComments(from, to Node, position CommentPosition) {
	for i, c := range cm[from] {
		if c.Position == position {
			cm.AddComment(to, c)

			// Remove the comment from the "from" slice
			cm[from][i] = cm[from][len(cm[from])-1]
			cm[from][len(cm[from])-1] = nil
			cm[from] = cm[from][:len(cm[from])-1]
		}
	}
}
