package parser

import (
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	itemText          itemType = iota // Line of text
	itemBlockQuote
	itemUl
	itemOl
	itemCode
	itemHr
	itemSetTextHeader
	itemH1
	itemH2
	itemH3
	itemH4
	itemH5
	itemH6
	itemEOF
	itemNewLine
	itemHardNewLine
	itemError
)

const eof = -1

const (
	br             delim = "\r\n"
	hardBr               = "  " + br
	ul0 = "-"
	ul1 = "+"
	ul2 = "*"
	hr1 = "*"
	hr2 = "-"
	ol                   = "1."
	atxHeader            = "#"
	setTextHeader1       = "="
	setTextHeader2       = "-"
	link                 = "["
	img                  = "!["
)

const inlineChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890!@#$%^&*()_-[]{};':\",./>? "

type itemType int
type delim string

type item struct {
	typ itemType
	val string
}

func (i item) String() string {
	switch {
	case i.typ == itemEOF:
		return "EOF"
	case i.typ == itemError:
		return i.val
	case i.typ == itemHardNewLine:
		return "Hard return"
	case i.typ == itemNewLine:
		return "Soft return"
	case i.typ == itemText:
		return fmt.Sprintf("Text: %q", i.val)
	case i.typ == itemUl:
		return "UL Item: " + i.val
	case i.typ >= itemH1 && i.typ < itemH6:
		return fmt.Sprintf("Header H%v", i.typ-itemH1+1)
		// case len(i.val) > 10:
		// 	return fmt.Sprintf("%.10q...", i.val)
	}
	return fmt.Sprintf("%q", i.val)
}

type lexer struct {
	name  string
	input string
	start int
	pos   int
	width int
	items chan item
}

// run starts the lexing process
func (l *lexer) run() {
	for state := lexText; state != nil; {
		state = state(l)
	}
	close(l.items)
}

// emit sends an item out on the items channel and resets pos & start
func (l *lexer) emit(t itemType) {
	l.items <- item{t, l.input[l.start:l.pos]}
	l.start = l.pos
}

// next returns the next rune in the input string and moves pos forward
func (l *lexer) next() rune {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	var r rune
	r, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return r
}

// nextNTimes runs next n times
func (l *lexer) nextNTimes(n int) []rune {
	res := make([]rune, n)
	for i := 0; i < n; i++ {
		res[i] = l.next()
	}
	return res
}

// ignore skips over the substr between l.start & l.pos
func (l *lexer) ignore() {
	l.start = l.pos
}

// ignoreNext ignores the next n runes
func (l *lexer) ignoreNext(n int) {
	l.nextNTimes(n)
	l.ignore()
}

// ignoreRun ignores all the following successive occurrences of r
func (l *lexer) ignoreRun(r rune) {
	for l.accept(string(r)) {
	}
	l.ignore()
}

// backup moves the pos cursor one step back
// WARNING: only safe to run once in between runs of next()
func (l *lexer) backup() {
	l.pos -= l.width
}

// backupNSpaces backs up n times
// WARNING: only safe to run when you are certain the previous n characters are identical
func (l *lexer) backupNSpaces(n int) {
	l.pos -= n * l.width
}

// peek returns the next rune without altering the state of the lexer
func (l *lexer) peek() rune {
	defer l.backup()
	return l.next()
}

// accept absorbs one rune from the valid string into the current item
func (l *lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

// acceptRun accepts successive characters as long as they are in the valid string
func (l *lexer) acceptRun(valid string) int {
	n := 0
	for strings.IndexRune(valid, rune(l.next())) >= 0 {
		n++
	}
	l.backup()
	return n
}

func (l *lexer) acceptUntilNewLine() {
	for ; (!hp(l.input[l.pos:], br) && l.peek() != eof); l.next() { }
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{itemError, fmt.Sprintf(format, args...)}
	return nil
}

// lex provisions the whole lexing scheme and passes back references
// to the lexer instance and items channel
func lex(name, input string) (*lexer, chan item) {
	l := &lexer{
		name:  name,
		input: input,
		items: make(chan item),
	}
	go l.run()
	return l, l.items
}

type stateFn func(*lexer) stateFn

// hp is a shorthand for strings.HasPrefix that accepts a delim param
func hp(s string, d delim) bool {
	return strings.HasPrefix(s, string(d))
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

// isEndOfLine reports whether r is an end-of-line character.
func isEndOfLine(r rune) bool {
	return r == '\r' || r == '\n'
}

// isAlphaNumeric reports whether r is an alphabetic, digit, or underscore.
func isAlphaNumeric(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// ============================================================ //
// ========================= STATES =========================== //
// ============================================================ //

func lexText(l *lexer) stateFn {
	/* What are we looking at right now? */
	s := l.input[l.pos:]
	if hp(s, atxHeader) {
		return lexAtxHeader
	} else if hp(s, ul0) || hp(s, ul2) {
		return lexHr
	} else if hp(s, ul1) && s[1] == ' ' {
		l.acceptRun(" " + string(ul1))
		return lexUl
	} else if hp(s, ol) && s[2] == ' ' {
		return lexOl
	}
	l.acceptUntilNewLine()
	lexTextNewLine(l)
	// Cursor now immediately after newline
	/* What were we just looking at? */
	l.acceptRun(" ") // Ignore leading spaces
	s = l.input[l.pos:] // Start checking line contents
	if hp(s, setTextHeader1) || hp(s, setTextHeader2) { // Previous line was setTextheader
		l.acceptRun(string(setTextHeader1) + string(setTextHeader2) + " ") // Accept all ='s, -'s and trailing spaces
		if !hp(l.input[l.pos:], br) {	// settext header stuff has trailing chars
			l.acceptUntilNewLine()
			lexTextNewLine(l)
			return lexText
		}
		// valid settext header declaration
		l.emit(itemSetTextHeader)
		l.nextNTimes(len(br))
		l.ignore()
		l.emit(itemNewLine)
	}
	return lexText
}

// lexTextNewLine lexes the newline at the end of text, emitting the correct line ending type
// cursor should be directly before "\r\n" when called
// cursor is moved to the start of the next line
func lexTextNewLine(l *lexer) {
	if (l.pos + len(br)) > len(l.input) {
		l.emit(itemEOF)
		os.Exit(0)
	}
	if l.input[l.pos - 2:l.pos + len(br)] == string(hardBr) {
		l.backupNSpaces(2)
		if (l.pos > l.start) {
			l.emit(itemText)
		}
		l.nextNTimes(len(hardBr))
		l.ignore()	// Ignore literal \r\n chars
		l.emit(itemHardNewLine)
	} else {
		if l.pos > l.start {
			l.emit(itemText)
		}
		l.nextNTimes(len(br))
		l.ignore()
		l.emit(itemNewLine)
	}
}

func lexSetTextHeader(l *lexer) stateFn {
	fmt.Println("Entered lexSetTextHeader")
	return nil
}

func lexAtxHeader(l *lexer) stateFn {
	var typ itemType
	n := l.acceptRun("#") // Find which level of header this is
	if l.peek() != ' ' {
		l.acceptUntilNewLine()
		lexTextNewLine(l)
		return lexText
	}
	l.acceptRun(" ")
	switch n { // Map to item type
	case 0:
		typ = itemError
	case 1:
		typ = itemH1
	case 2:
		typ = itemH2
	case 3:
		typ = itemH3
	case 4:
		typ = itemH4
	case 5:
		typ = itemH5
	case 6:
		fallthrough
	default:
		typ = itemH6
	}
	if typ == itemError {
		return l.errorf("Expected \"#\" at start of ATX header") // Send error & exit
	}
	l.ignore()
	l.emit(typ)
	return lexText
}

func lexHr(l *lexer) stateFn {
	hrChar := l.input[l.pos:l.pos+1] // '-' or '*'
	for !hp(l.input[l.pos:], br) {
		if !l.accept(hrChar) {
			l.ignore()
			return lexUl
		}
		l.acceptRun(" ")		
	}
	l.nextNTimes(len(br))
	l.ignore()
	l.emit(itemHr)
	return lexText
}

func lexUl(l *lexer) stateFn {
	l.emit(itemUl)
	return lexText
}

func lexOl(l *lexer) stateFn {
	return nil
}
// ============================================================ //
// ======================= END STATES ========================= //
// ============================================================ //
