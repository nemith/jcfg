package jcfg

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type token struct {
	typ tokenType // type of token
	pos int       // starting position of item in input
	val string    // value of the token
}

func (t token) String() string {
	return fmt.Sprintf("(%s, '%s')", t.typ, t.val)
}

//go:generate stringer -type=tokenType -output=token_string.go
type tokenType int

const (
	tokenError        tokenType = iota // error occurred with the value is the text of the error
	tokenEOF                           // end of the file
	tokenKeyword                       // Keyword can be the start of a value or of a section
	tokenValue                         // Value starting after a keyword but before the end
	tokenValueString                   // Quoted Value
	tokenEndStatement                  // End of the statement usually ended with a ';'
	tokenSectionStart                  // Start of a section '{'
	tokenSectionEnd                    // End of a section '}'
	tokenLineComment                   // Line comment starting with // until the end of a line (Not technically used in Junos)
	tokenHashComment                   // Line comment starting with a # or ## until the end of a line
	tokenBlockComment                  // Multiline capaible comment starting with /* and ending with */
	tokenModifier                      // Modifier at the start of a statement (e.g 'deactivate:')
	tokenListStart                     // Start of a list '['
	tokenListEnd                       // End of a list ']'
)

const (
	eof = -1
)

type stateFn func(*lexer) stateFn

type lexer struct {
	name   string
	input  string
	start  int
	pos    int
	width  int
	tokens chan token
}

func (l *lexer) emit(t tokenType) {
	l.tokens <- token{t, l.start, l.input[l.start:l.pos]}
	l.start = l.pos
}

func (l *lexer) next() rune {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = w
	l.pos += l.width
	return r
}

func (l *lexer) ignore() {
	l.start = l.pos
}

func (l *lexer) backup() {
	l.pos -= l.width
}

func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

func (l *lexer) skipSpace() {
	r := l.next()
	for unicode.IsSpace(r) {
		r = l.next()
	}
	l.backup()
	l.ignore()
}

func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.tokens <- token{tokenError, l.start, fmt.Sprintf(format, args...)}
	return nil
}

// lineNumber reports which line we're on. Doing it this way
// means we don't have to worry about peek double counting.
func (l *lexer) lineNumber(pos int) int {
	return 1 + strings.Count(l.input[:pos], "\n")
}

// columnNumber reports which column in the current line we're on.
func (l *lexer) columnNumber(pos int) int {
	n := strings.LastIndex(l.input[:pos], "\n")
	if n == -1 {
		n = 0
	}
	return int(pos) - n
}

// nextToken returns the next token from the input.
func (l *lexer) nextToken() token {
	token := <-l.tokens
	//	l.lastPos = token.pos
	return token
}

func lex(name, input string) *lexer {
	l := &lexer{
		name:   name,
		input:  input,
		tokens: make(chan token),
	}
	go l.run()
	return l
}

func (l *lexer) run() {
	for state := lexInsideSection; state != nil; {
		state = state(l)
	}
	close(l.tokens)
}

const (
	lineComment       = "//"
	hashComment       = "#"
	leftBlockComment  = "/*"
	rightBlockComment = "*/"
)

func lexInsideSection(l *lexer) stateFn {
	for {
		if strings.HasPrefix(l.input[l.pos:], lineComment) {
			return lexLineComment
		}

		if strings.HasPrefix(l.input[l.pos:], leftBlockComment) {
			return lexBlockComment
		}

		switch r := l.next(); {
		case r == eof:
			l.emit(tokenEOF)
			return nil
		case r == '#':
			l.backup()
			return lexHashComment
		case r == '}':
			l.emit(tokenSectionEnd)
		case isAlphaNumeric(r):
			l.backup()
			return lexStatement
		case unicode.IsSpace(r):
			l.ignore()
			continue
		default:
			l.errorf("Invalid statement: %s", string(r))
		}
	}

	// Reached EOF
	l.emit(tokenEOF)
	return nil
}

func lexStatement(l *lexer) stateFn {
	switch r := l.next(); {
	case r == ';' || r == eof:
		l.emit(tokenKeyword)
		return lexEndStatement
	case unicode.IsSpace(r):
		for unicode.IsSpace(l.peek()) {
			l.next()
		}
		l.ignore()
	case isAlphaNumeric(r):
		return lexKeyword
	}
	return lexStatement
}

func lexKeyword(l *lexer) stateFn {
	for isAlphaNumeric(l.peek()) {
		l.next()
	}
	if l.peek() == ':' {
		l.emit(tokenModifier)
		l.ignore()
		return lexStatement
	}
	l.emit(tokenKeyword)
	return lexValues
}

func lexValues(l *lexer) stateFn {
	switch r := l.next(); {
	case r == '"':
		return lexQuote
	case r == '{':
		return lexSectionStart
	case r == ';' || r == '\n' || r == eof:
		return lexEndStatement
	case r == '/':
		if l.next() != '/' {
			return l.errorf("invalid input (missing second / for comment)")
		}
		l.backup()
		l.backup()
		l.emit(tokenEndStatement)
		return lexLineComment
	case r == '#':
		fmt.Printf("Are we getting here?")
		l.backup()
		l.emit(tokenEndStatement)
		return lexHashComment
	case unicode.IsSpace(r):
		for unicode.IsSpace(l.peek()) {
			l.next()
		}
		l.ignore()
	case isAlphaNumeric(r):
		return lexValue

	default:
		l.errorf("invalid shit yo.")
	}
	return lexValues
}

func lexValue(l *lexer) stateFn {
	for isAlphaNumeric(l.peek()) {
		l.next()
	}
	l.emit(tokenValue)
	return lexValues
}

func lexModifier(l *lexer) stateFn {
	l.emit(tokenModifier)
	l.skipSpace()
	return lexStatement(l)
}

func lexEndStatement(l *lexer) stateFn {
	l.emit(tokenEndStatement)
	return lexInsideSection
}

func lexSectionStart(l *lexer) stateFn {
	l.emit(tokenSectionStart)
	return lexInsideSection
}

// lexQuote scans a quoted string.
func lexQuote(l *lexer) stateFn {
Loop:
	for {
		switch l.next() {
		case '\\':
			if r := l.next(); r != eof {
				break
			}
			fallthrough
		case eof:
			return l.errorf("unterminated quoted string")
		case '"':
			break Loop
		}
	}
	l.emit(tokenValue)
	return lexValues
}

func lexHashComment(l *lexer) stateFn {
	for {
		r := l.next()
		if r == '\n' || r == eof {
			break
		}
	}
	l.emit(tokenHashComment)
	return lexInsideSection
}

func lexLineComment(l *lexer) stateFn {
	for {
		r := l.next()
		if r == '\n' || r == eof {
			break
		}
	}
	l.emit(tokenLineComment)
	return lexInsideSection
}

func lexBlockComment(l *lexer) stateFn {
	i := strings.Index(l.input[l.pos:], rightBlockComment)
	if i < 0 {
		return l.errorf("unclosed comment")
	}
	l.pos += (i + len(rightBlockComment))
	l.emit(tokenBlockComment)
	l.ignore()
	return lexInsideSection
}

func isAlphaNumeric(r rune) bool {
	//	if strings.IndexRune("!#$%&|*+-/:<=>?@^_~", r) >= 0 {
	//		return true
	//	}
	return r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
