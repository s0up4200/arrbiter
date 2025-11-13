package filter

import (
	"fmt"
)

// Error types for filter operations
type (
	// CompilationError indicates a filter expression could not be compiled
	CompilationError struct {
		Expression string
		Reason     string
		Position   int // -1 if position is unknown
		Err        error
	}

	// EvaluationError indicates a filter could not be evaluated
	EvaluationError struct {
		FilterName string
		MovieTitle string
		Reason     string
		Err        error
	}
)

func (e *CompilationError) Error() string {
	if e.Position >= 0 {
		return fmt.Sprintf("compilation error at position %d in '%s': %s", e.Position, e.Expression, e.Reason)
	}
	return fmt.Sprintf("compilation error in '%s': %s", e.Expression, e.Reason)
}

func (e *CompilationError) Unwrap() error {
	return e.Err
}

func (e *EvaluationError) Error() string {
	return fmt.Sprintf("evaluation error for filter '%s' on movie '%s': %s", e.FilterName, e.MovieTitle, e.Reason)
}

func (e *EvaluationError) Unwrap() error {
	return e.Err
}
