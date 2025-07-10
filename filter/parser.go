package filter

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Parser parses filter expressions
type Parser struct {
	input      string
	position   int
	tokens     []Token
	currentIdx int // Renamed to avoid conflict with current() method
}

// NewParser creates a new parser
func NewParser(input string) *Parser {
	return &Parser{
		input: input,
	}
}

// Parse parses the input and returns a Filter
func (p *Parser) Parse() (Filter, error) {
	if err := p.tokenize(); err != nil {
		return nil, err
	}

	return p.parseExpression()
}

// tokenize breaks the input into tokens
func (p *Parser) tokenize() error {
	p.tokens = []Token{}
	p.position = 0

	for p.position < len(p.input) {
		p.skipWhitespace()

		if p.position >= len(p.input) {
			break
		}

		// Check for operators and keywords
		switch {
		case p.consumeKeyword("AND"):
			p.tokens = append(p.tokens, Token{Type: TokenAND, Value: "AND"})
		case p.consumeKeyword("OR"):
			p.tokens = append(p.tokens, Token{Type: TokenOR, Value: "OR"})
		case p.consumeKeyword("NOT"):
			p.tokens = append(p.tokens, Token{Type: TokenNOT, Value: "NOT"})
		case p.current() == '(':
			p.tokens = append(p.tokens, Token{Type: TokenLParen, Value: "("})
			p.position++
		case p.current() == ')':
			p.tokens = append(p.tokens, Token{Type: TokenRParen, Value: ")"})
			p.position++
		case p.isFieldStart():
			field, err := p.consumeField()
			if err != nil {
				return err
			}
			p.tokens = append(p.tokens, Token{Type: TokenField, Value: field})

			// Expect operator
			p.skipWhitespace()
			op, err := p.consumeOperator()
			if err != nil {
				return err
			}
			p.tokens = append(p.tokens, Token{Type: TokenOperator, Value: op})

			// Expect value
			p.skipWhitespace()
			value, err := p.consumeValue()
			if err != nil {
				return err
			}
			p.tokens = append(p.tokens, Token{Type: TokenValue, Value: value})

			// Special handling for watch_count_by which needs a second operator and value
			if field == "watch_count_by" {
				// Look for comparison operator and number
				p.skipWhitespace()
				if p.position < len(p.input) {
					// Check if there's a comparison operator
					savePos := p.position
					compOp, err := p.consumeOperator()
					if err == nil {
						p.tokens = append(p.tokens, Token{Type: TokenOperator, Value: compOp})

						// Get the number value
						p.skipWhitespace()
						numValue, err := p.consumeValue()
						if err == nil {
							p.tokens = append(p.tokens, Token{Type: TokenValue, Value: numValue})
						} else {
							// Restore position if we can't parse the number
							p.position = savePos
						}
					} else {
						// Restore position if no comparison operator
						p.position = savePos
					}
				}
			}
		default:
			return &ParseError{
				Message:  fmt.Sprintf("unexpected character: %c", p.current()),
				Position: p.position,
			}
		}
	}

	p.tokens = append(p.tokens, Token{Type: TokenEOF})
	return nil
}

// Helper methods for tokenization
func (p *Parser) current() byte {
	if p.position >= len(p.input) {
		return 0
	}
	return p.input[p.position]
}

func (p *Parser) peek(offset int) byte {
	pos := p.position + offset
	if pos >= len(p.input) {
		return 0
	}
	return p.input[pos]
}

func (p *Parser) skipWhitespace() {
	for p.position < len(p.input) && unicode.IsSpace(rune(p.input[p.position])) {
		p.position++
	}
}

func (p *Parser) consumeKeyword(keyword string) bool {
	if p.position+len(keyword) > len(p.input) {
		return false
	}

	if strings.ToUpper(p.input[p.position:p.position+len(keyword)]) == keyword {
		// Check that the keyword is followed by whitespace or special char
		nextPos := p.position + len(keyword)
		if nextPos >= len(p.input) || unicode.IsSpace(rune(p.input[nextPos])) ||
			p.input[nextPos] == '(' || p.input[nextPos] == ')' {
			p.position += len(keyword)
			return true
		}
	}
	return false
}

func (p *Parser) isFieldStart() bool {
	// Fields start with a letter
	return unicode.IsLetter(rune(p.current()))
}

func (p *Parser) consumeField() (string, error) {
	start := p.position
	for p.position < len(p.input) && (unicode.IsLetter(rune(p.current())) || p.current() == '_') {
		p.position++
	}

	if start == p.position {
		return "", &ParseError{
			Message:  "expected field name",
			Position: p.position,
		}
	}

	return p.input[start:p.position], nil
}

func (p *Parser) consumeOperator() (string, error) {
	// Check two-character operators first
	if p.position+1 < len(p.input) {
		twoChar := p.input[p.position : p.position+2]
		switch twoChar {
		case "!:", ">=", "<=":
			p.position += 2
			return twoChar, nil
		}
	}

	// Check single-character operators
	switch p.current() {
	case ':', '>', '<':
		op := string(p.current())
		p.position++
		return op, nil
	}

	return "", &ParseError{
		Message:  "expected operator (:, !:, >, <, >=, <=)",
		Position: p.position,
	}
}

func (p *Parser) consumeValue() (string, error) {
	// Handle quoted values
	if p.current() == '"' {
		p.position++ // Skip opening quote
		start := p.position
		for p.position < len(p.input) && p.current() != '"' {
			if p.current() == '\\' && p.peek(1) == '"' {
				p.position += 2 // Skip escaped quote
			} else {
				p.position++
			}
		}
		if p.current() != '"' {
			return "", &ParseError{
				Message:  "unterminated string",
				Position: start,
			}
		}
		value := p.input[start:p.position]
		p.position++ // Skip closing quote
		// Unescape quotes
		return strings.ReplaceAll(value, `\"`, `"`), nil
	}

	// Handle unquoted values
	start := p.position
	for p.position < len(p.input) && !unicode.IsSpace(rune(p.current())) &&
		p.current() != '(' && p.current() != ')' {
		p.position++
	}

	if start == p.position {
		return "", &ParseError{
			Message:  "expected value",
			Position: p.position,
		}
	}

	return p.input[start:p.position], nil
}

// Parsing methods
func (p *Parser) parseExpression() (Filter, error) {
	return p.parseOr()
}

func (p *Parser) parseOr() (Filter, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.currentIdx < len(p.tokens) && p.tokens[p.currentIdx].Type == TokenOR {
		p.currentIdx++ // consume OR
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &OrFilter{Left: left, Right: right}
	}

	return left, nil
}

func (p *Parser) parseAnd() (Filter, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}

	for p.currentIdx < len(p.tokens) && p.tokens[p.currentIdx].Type == TokenAND {
		p.currentIdx++ // consume AND
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &AndFilter{Left: left, Right: right}
	}

	return left, nil
}

func (p *Parser) parseNot() (Filter, error) {
	if p.currentIdx < len(p.tokens) && p.tokens[p.currentIdx].Type == TokenNOT {
		p.currentIdx++ // consume NOT
		filter, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &NotFilter{Filter: filter}, nil
	}

	return p.parsePrimary()
}

func (p *Parser) parsePrimary() (Filter, error) {
	// Handle parentheses
	if p.currentIdx < len(p.tokens) && p.tokens[p.currentIdx].Type == TokenLParen {
		p.currentIdx++ // consume (
		filter, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.currentIdx >= len(p.tokens) || p.tokens[p.currentIdx].Type != TokenRParen {
			return nil, &ParseError{Message: "expected closing parenthesis"}
		}
		p.currentIdx++ // consume )
		return filter, nil
	}

	// Handle field filters
	if p.currentIdx < len(p.tokens) && p.tokens[p.currentIdx].Type == TokenField {
		field := p.tokens[p.currentIdx].Value
		p.currentIdx++

		if p.currentIdx >= len(p.tokens) || p.tokens[p.currentIdx].Type != TokenOperator {
			return nil, &ParseError{Message: "expected operator after field"}
		}
		op := p.tokens[p.currentIdx].Value
		p.currentIdx++

		if p.currentIdx >= len(p.tokens) || p.tokens[p.currentIdx].Type != TokenValue {
			return nil, &ParseError{Message: "expected value after operator"}
		}
		value := p.tokens[p.currentIdx].Value
		p.currentIdx++

		// Special handling for watch_count_by
		if field == "watch_count_by" {
			// Check if there are more tokens for the comparison
			if p.currentIdx < len(p.tokens)-1 && p.tokens[p.currentIdx].Type == TokenOperator {
				compOp := p.tokens[p.currentIdx].Value
				p.currentIdx++

				if p.currentIdx < len(p.tokens) && p.tokens[p.currentIdx].Type == TokenValue {
					countStr := p.tokens[p.currentIdx].Value
					p.currentIdx++

					// Parse the count
					count, err := strconv.Atoi(countStr)
					if err != nil {
						return nil, &ParseError{Message: fmt.Sprintf("invalid count for watch_count_by: %s", countStr)}
					}

					// Create a special filter that includes both username and count
					return &FieldFilter{
						Field:    FieldType("watch_count_by"),
						Operator: Operator(compOp), // The comparison operator (>, <, etc)
						Value:    value,            // The username
						IntValue: count,            // The count to compare against
					}, nil
				}
			}
			// If no comparison operator, default to checking if user has watched at all
			return &FieldFilter{
				Field:    FieldType("watch_count_by"),
				Operator: Operator(op),
				Value:    value,
				IntValue: 0, // Default: has the user watched at all
			}, nil
		}

		return createFieldFilter(field, op, value)
	}

	if p.currentIdx < len(p.tokens) && p.tokens[p.currentIdx].Type == TokenEOF {
		return nil, &ParseError{Message: "unexpected end of expression"}
	}

	return nil, &ParseError{Message: "expected field filter or parenthesized expression"}
}

// createFieldFilter creates a field filter from the parsed values
func createFieldFilter(field, op, value string) (Filter, error) {
	fieldType := FieldType(field)
	operator := Operator(op)

	// Validate field type
	switch fieldType {
	case FieldTag:
		return &FieldFilter{
			Field:    fieldType,
			Operator: operator,
			Value:    value,
		}, nil
	case FieldAddedBefore, FieldAddedAfter, FieldImportedBefore, FieldImportedAfter:
		// Parse date
		date, err := time.Parse("2006-01-02", value)
		if err != nil {
			return nil, &ParseError{Message: fmt.Sprintf("invalid date format: %s (expected YYYY-MM-DD)", value)}
		}
		return &FieldFilter{
			Field:    fieldType,
			Operator: operator,
			Value:    date,
		}, nil
	case FieldWatched:
		// Parse boolean
		boolVal := strings.ToLower(value) == "true" || value == "1" || strings.ToLower(value) == "yes"
		return &FieldFilter{
			Field:    fieldType,
			Operator: operator,
			Value:    boolVal,
		}, nil
	case FieldWatchCount:
		// Parse integer
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return nil, &ParseError{Message: fmt.Sprintf("invalid number: %s", value)}
		}
		return &FieldFilter{
			Field:    fieldType,
			Operator: operator,
			Value:    intVal,
		}, nil
	case FieldWatchedBy:
		// Username is stored as string
		return &FieldFilter{
			Field:    fieldType,
			Operator: operator,
			Value:    value,
		}, nil
	case FieldWatchCountBy:
		// For watch_count_by, we store the username in Value
		// The actual comparison will be handled in parsePrimary
		return &FieldFilter{
			Field:    fieldType,
			Operator: operator,
			Value:    value, // Username
		}, nil
	default:
		return nil, &ParseError{Message: fmt.Sprintf("unknown field: %s", field)}
	}
}
