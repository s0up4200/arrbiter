package filter

import (
	"fmt"
)

// TokenType represents the type of a token in the filter expression
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenAND
	TokenOR
	TokenNOT
	TokenLParen
	TokenRParen
	TokenField
	TokenOperator
	TokenValue
)

// Token represents a lexical token
type Token struct {
	Type  TokenType
	Value string
}

// FieldType represents the type of field being filtered
type FieldType string

const (
	FieldTag            FieldType = "tag"
	FieldAddedBefore    FieldType = "added_before"
	FieldAddedAfter     FieldType = "added_after"
	FieldImportedBefore FieldType = "imported_before"
	FieldImportedAfter  FieldType = "imported_after"
	FieldWatched        FieldType = "watched"
	FieldWatchCount     FieldType = "watch_count"
	FieldWatchedBy      FieldType = "watched_by"
	FieldWatchCountBy   FieldType = "watch_count_by"
)

// Operator represents a comparison operator
type Operator string

const (
	OpEquals       Operator = ":"
	OpNotEquals    Operator = "!:"
	OpGreaterThan  Operator = ">"
	OpLessThan     Operator = "<"
	OpGreaterEqual Operator = ">="
	OpLessEqual    Operator = "<="
)

// Filter represents a filter expression
type Filter interface {
	Evaluate(movie interface{}) bool
	String() string
}

// AndFilter represents an AND operation between filters
type AndFilter struct {
	Left  Filter
	Right Filter
}

func (f *AndFilter) Evaluate(movie interface{}) bool {
	return f.Left.Evaluate(movie) && f.Right.Evaluate(movie)
}

func (f *AndFilter) String() string {
	return fmt.Sprintf("(%s AND %s)", f.Left.String(), f.Right.String())
}

// OrFilter represents an OR operation between filters
type OrFilter struct {
	Left  Filter
	Right Filter
}

func (f *OrFilter) Evaluate(movie interface{}) bool {
	return f.Left.Evaluate(movie) || f.Right.Evaluate(movie)
}

func (f *OrFilter) String() string {
	return fmt.Sprintf("(%s OR %s)", f.Left.String(), f.Right.String())
}

// NotFilter represents a NOT operation on a filter
type NotFilter struct {
	Filter Filter
}

func (f *NotFilter) Evaluate(movie interface{}) bool {
	return !f.Filter.Evaluate(movie)
}

func (f *NotFilter) String() string {
	return fmt.Sprintf("NOT %s", f.Filter.String())
}

// FieldFilter represents a field comparison filter
type FieldFilter struct {
	Field    FieldType
	Operator Operator
	Value    interface{}
	// For user-specific filters that need both username and value
	Username string
	IntValue int
}

func (f *FieldFilter) String() string {
	return fmt.Sprintf("%s%s%v", f.Field, f.Operator, f.Value)
}

// ParseError represents an error during parsing
type ParseError struct {
	Message  string
	Position int
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at position %d: %s", e.Position, e.Message)
}
