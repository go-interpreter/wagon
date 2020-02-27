// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wast

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-interpreter/wagon/wasm"
)

type Scanner struct {
	file  string
	inBuf *bytes.Buffer

	ch    rune
	eof   bool
	token *Token

	offset int

	Line   int
	Column int

	Errors []error
}

func NewScanner(path string) *Scanner {
	var s Scanner

	s.file = path
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		s.raise(err)
		return &s
	}

	s.inBuf = bytes.NewBuffer(buf)

	s.eof = false
	s.Line = 1
	s.Column = 1
	s.offset = 0

	return &s
}

const (
	eofRune = -1
	errRune = -2
)

func (s *Scanner) peek() rune {
	if s.eof {
		return eofRune
	}

	r, _, err := s.inBuf.ReadRune()
	defer s.inBuf.UnreadRune() // rewind

	switch {
	case err == io.EOF:
		return eofRune
	case err != nil:
		s.raise(err)
		return errRune
	}

	return r
}

func (s *Scanner) next() rune {
	if s.eof {
		return eofRune
	}

	r, n, err := s.inBuf.ReadRune()
	switch {
	case err == io.EOF:
		s.eof = true
		s.ch = eofRune
		s.offset += n
		s.Column++
		return eofRune
	case err != nil:
		s.raise(err)
		return errRune
	}

	if r == '\n' {
		s.Column = 0
		s.Line++
	}

	s.offset += n
	s.Column++
	s.ch = r

	return r
}

func (s *Scanner) match(r rune) bool {
	if s.peek() == r {
		s.next()
		return true
	}
	return false
}

func (s *Scanner) matchIf(f func(rune) bool) bool {
	if f(s.peek()) {
		s.next()
		return true
	}
	return false
}

func (s *Scanner) Next() (token *Token) {
	s.token = &Token{
		Line:   s.Line,
		Column: s.Column,
	}
	token = s.token

	if s.match(eofRune) {
		token.Kind = EOF
		s.next()
		return
	}

	switch {
	case s.matchIf(isSpace): // ignore spaces
	case s.match('('):
		if s.match(';') {
			s.scanBlockComment()
			return s.Next()
		}
		token.Text = "("
		token.Kind = LPAR
		return
	case s.match(')'):
		token.Text = ")"
		token.Kind = RPAR
		return
	case s.match(';'):
		if s.match(';') {
			s.scanLineComment()
			return s.Next()
		}
		s.errorf("unexpected character ';'")
	case s.match('$'): // names/vars
		token.Kind = VAR
		token.Text = "$"
		s.scanVar()
		return
	case s.match('"'):
		s.scanString()
		return
	case s.matchIf(isReserved):
		s.scanReserved()
		return
	case s.matchIf(isUtf8):
		s.errorf("malformed operator")
		s.next()
	default:
		s.errorf("malformed UTF-8 encoding")
		s.next()
	}

	return s.Next()
}

func (s *Scanner) scanString() {
	s.token.Kind = STRING
	s.token.Text = ""
	for !s.eof {
		switch {
		case s.eof || s.match('\n'):
			s.errorf("unclosed string literal")
			return
		case s.matchIf(unicode.IsControl):
			s.errorf("illegal control character in string literal")
		case s.match('"'):
			return
		case s.match('\\'):
			s.scanEscape()
		// case s.matchIf(isStringRune):
		default:
			s.next()
			s.token.Text += string(s.ch)
		}
	}
}

func (s *Scanner) scanEscape() bool {
	// escape slash is already matched
	switch s.next() {
	case 'n':
		s.token.Text += "\n"
	case 'r':
		s.token.Text += "\r"
	case 't':
		s.token.Text += "\t"
	case '\\':
		s.token.Text += "\\"
	case '\'':
		s.token.Text += "'"
	case '"':
		s.token.Text += "\""
	case 'u': // unicode
		if !s.match('{') {
			s.errorf("missing opening '{' in unicode escape sequence")
			s.token.Text += "\\u"
			return false
		}

		esc := ""
		for s.matchIf(isHexDigit) {
			esc += string(s.ch)
		}

		switch {
		case len(esc) == 0 && s.match('}'):
			s.errorf("empty unicode escape sequence")
			s.token.Text += "\\u{}"
			return false
		case len(esc) == 0 && !s.match('}'):
			rtext := safeRune(s.peek())
			s.errorf("unexpected character in unicode escape sequence '%s'", rtext)
			s.token.Text += "\\u" + rtext
			return false
		case len(esc) > 0 && !s.match('}'):
			s.errorf("missing closing '}' in unicode escape sequence")
			s.token.Text += "\\u{" + esc
			return false
		}

		n, err := strconv.ParseInt(esc, 16, 0)
		if err != nil {
			s.raise(err)
			s.token.Text += "\\u{" + esc + "}"
			return false
		}

		u := make([]byte, 4)
		utf8.EncodeRune(u, rune(n))
		s.token.Text += string(bytes.Trim(u, "\x00")) // remove NULL bytes
	default: // hexadecimal
		if !isHexDigit(s.ch) {
			rtext := safeRune(s.ch)
			s.errorf("unexpected character in hexadecimal escape sequence '%s'", rtext)
			s.token.Text += "\\" + rtext
			return false
		}

		esc := string(s.ch)
		if !s.matchIf(isHexDigit) {
			rtext := safeRune(s.peek())
			s.errorf("unexpected character in hexadecimal escape sequence '%s'", rtext)
			s.token.Text += "\\" + esc + rtext
			return false
		}
		esc += string(s.ch)

		n, err := strconv.ParseInt(esc, 16, 0)
		if err != nil {
			s.raise(err)
			s.token.Text += "\\" + esc
			return false
		}
		s.token.Text += string(n)
	}
	return true
}

func (s *Scanner) scanVar() {
	if s.match('_') || s.matchIf(isLetter) || s.matchIf(isDigit) || s.matchIf(isSymbol) {
		s.token.Text += string(s.ch)
		s.scanVar()
	}

	if len(s.token.Text) == 1 {
		s.errorf("empty $-name")
	}
}

func (s *Scanner) scanReserved() {
	s.token.Text = string(s.ch)
	for s.matchIf(isReserved) {
		s.token.Text += string(s.ch)

		if isType(s.token.Text) && s.match('.') {
			s.scanTypedReserved(s.token.Text)
			return
		}
	}

	// isolated type token 'i32', 'f64', ...
	if isType(s.token.Text) {
		s.token.Kind = VALUE_TYPE
		s.token.Data = valueTypeOf(s.token.Text)
		return
	}

	// Basic instruction / reserved word
	if k, ok := tokenKindOf[s.token.Text]; ok {
		s.token.Kind = k
	}
}

func (s *Scanner) scanTypedReserved(t string) {
	var instr string

	s.token.Text += string(s.ch) // '.'
	for s.matchIf(isReserved) {
		instr += string(s.ch)
	}
	s.token.Text += instr

	s.token.Data = valueTypeOf(t)
	if k, ok := typedKindOf[instr]; ok {
		s.token.Kind = k
		return
	}

	s.errorf("unkown operator")
}

func (s *Scanner) scanLineComment() {
	for s.next() != '\n' {
	}
}

func (s *Scanner) scanBlockComment() {
	for depth := 1; depth > 0; {
		switch {
		case s.eof:
			s.errorf("unclosed comment")
			return
		case s.match('(') && s.match(';'):
			depth++
		case s.match(';') && s.match(')'):
			depth--
		default:
			s.next()
		}
	}
}

const (
	scanErrPrefix  = "error: "
	scanWarnPrefix = "warning: "
)

// errorf generates a new scanner error appended to the scanner's Errors field
func (s *Scanner) errorf(fmtStr string, args ...interface{}) {
	pfx := fmt.Sprintf("%s ~ line %d, column %d\n  => ", s.file, s.token.Line, s.token.Column)
	err := fmt.Errorf(scanErrPrefix+pfx+fmtStr+"\n", args...)
	s.Errors = append(s.Errors, err)
}

// raise directly promote any error to a printable Scanner error
func (s *Scanner) raise(err error) {
	err2 := fmt.Errorf(scanErrPrefix+"%s\n  %s\n", s.file, err.Error())
	s.Errors = append(s.Errors, err2)
}

func safeRune(r rune) string {
	switch r {
	case '\n':
		return "\\n"
	case '\r':
		return "\\r"
	case '\t':
		return "\\t"
	case '\'':
		return "\\'"
	case '"':
		return "\\\""
	default:
		if unicode.IsPrint(r) {
			return string(r)
		}
		return fmt.Sprintf("\\%x", r)
	}
}

func isUtf8(r rune) bool {
	return utf8.ValidRune(r)
}

func isReserved(r rune) bool {
	return !isSpace(r) && strings.IndexRune("\"();", r) < 0
}

func isSpace(r rune) bool {
	switch r {
	case ' ', '\n', '\r', '\t':
		return true
	default:
		return false
	}
}

func isSymbol(r rune) bool {
	return strings.IndexRune("+-*/\\^~=<>!?@#$%&|:`.'", r) > 0
}

func isSign(r rune) bool {
	return r == '-' || r == '+'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isHexDigit(r rune) bool {
	if isDigit(r) {
		return true
	}
	return (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func isLowerLetter(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func isUpperLetter(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isLetter(r rune) bool {
	return isLowerLetter(r) || isUpperLetter(r)
}

func isType(s string) bool {
	switch s {
	case "i32", "i64", "f32", "f64":
		return true
	default:
		return false
	}
}

func isNum(s string) bool {
	if len(s) == 0 || !isDigit(rune(s[0])) {
		return false
	}

	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			i++
			if i >= len(s) {
				return false
			}
		}
		if !isDigit(rune(s[i])) {
			return false
		}
	}
	return true
}

func isNat(s string) bool {
	if strings.HasPrefix(s, "0x") {
		return isHexNum(s[2:])
	}
	return isNat(s)
}

func isInt(s string) bool {
	if len(s) == 0 || !isSign(rune(s[0])) {
		return false
	}
	return isNat(s[1:])
}

func isFloat(s string) bool {
	if len(s) == 0 {
		return false
	}

	switch s[0] {
	case '+', '-':
		s = s[1:] // strip the string from the sign
	}

	switch {
	case s == "inf":
		return true
	case s == "nan":
		return true
	case strings.HasPrefix(s, "nan:0x"):
		return isHexNum(s[6:]) // len("nan:0x") == 6
	}

	if strings.HasPrefix(s, "0x") {
		period := strings.IndexRune(s[2:], '.')
		if period <= 1 {
			// there must be an hexnum between the '0x' and the '.'
			return false
		}

		units := s[2 : 2+period]
		if 2+period == len(s) {
			return isHexNum(units)
		}

		decimals := s[2+period+1:]
		return isHexNum(units) && isHexNum(decimals)
	}

	return false
}

func isHexNum(s string) bool {
	if len(s) == 0 || !isHexDigit(rune(s[0])) {
		return false
	}

	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			i++
			if i >= len(s) {
				return false
			}
		}
		if !isHexDigit(rune(s[i])) {
			return false
		}
	}
	return true
}

func digitValue(r rune) int {
	switch {
	case isDigit(r):
		return int(r) - '0'
	case r >= 'a' && r <= 'f':
		return int(r) - 'a' + 10
	case r >= 'A' && r <= 'F':
		return int(r) - 'A' + 10
	}
	return 16 // max
}

func valueTypeOf(s string) wasm.ValueType {
	switch s {
	case "i32":
		return wasm.ValueTypeI32
	case "i64":
		return wasm.ValueTypeI64
	case "f32":
		return wasm.ValueTypeF32
	case "f64":
		return wasm.ValueTypeF64
	}
	return 0 // TODO find a suitable error ValueType value
}
