package parser

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	itemParagraph          itemType = iota // New paragraph
	itemParagraphContinued                 // Paragraph continued after a new line
	itemBlockQuote
	itemUl
	itemOl
	itemCode
	itemHr
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
	case i.typ == itemParagraph:
		return fmt.Sprintf("New paragraph: %q", i.val)
	case i.typ == itemParagraphContinued:
		return fmt.Sprintf("Continued: %q", i.val)
	case i.typ >= itemH1 && i.typ < itemH6:
		return fmt.Sprintf("Header H%v: %q", i.typ-itemH1+1, i.val)
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
	for state := lexBlock; state != nil; {
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
	for ; !hp(l.input[l.pos:], br); l.next() { }
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

func lexBlock(l *lexer) stateFn {
	for {
		s := l.input[l.pos:]
		if hp(s, hardBr) {
			l.nextNTimes(len(hardBr))
			l.emit(itemHardNewLine)
		} else if hp(s, ul0) || hp(s, ul1) || hp(s, ul2) {
				fmt.Println("entering lex ul...")
				return lexUl
		} else if isAlphaNumeric(rune(l.peek())) {
			fmt.Println("entering lex paragraph...")
			return lexNewParagraph
		} else if hp(s, atxHeader) {
			fmt.Println("entering lex atx header...")
			return lexAtxHeader
		} else if l.pos > len(l.input) {
			break
		}
	}
	if l.pos > l.start {
		l.emit(itemParagraph)
	}
	l.emit(itemEOF)
	return nil
}

func lexNewParagraph(l *lexer) stateFn {
	return lexParagraph(l, itemParagraph)
}

func lexParagraphContinued(l *lexer) stateFn {
	return lexParagraph(l, itemParagraphContinued)
}

func lg(s ...string) {
	// fmt.Println(s)
}

func lexParagraph(l *lexer, typ itemType) stateFn {
	if l.accept(string(setTextHeader1)) || l.accept(string(setTextHeader2)) {
		return lexSetTextHeader // They are trying to
	}
	for {
		n := l.acceptRun(inlineChars)
		if n == 0 { // No more characters to absorb (might be unnecessary)
			if hp(l.input[l.pos-2:], hardBr) {
				l.emit(typ)             // Emit either para or continued para
				l.nextNTimes(len(br))   // Absorb the newline
				l.emit(itemHardNewLine) // Emit hard new line, now pos is at beginning of next line
				l.acceptRun(" ")
				l.ignore() // Ignore leading spaces
				if l.accept(string(setTextHeader1)) || l.accept(string(setTextHeader2)) {
					lg("Matched setTextHeaders")
					l.backup() // Reset for next state
					return lexSetTextHeader
				} else if l.accept(string(br)) {
					lg("Matched second newLine")
					return lexBlock // Another newline triggers a new block
				} else if isAlphaNumeric(l.peek()) {
					lg("Matched continued para with:", string(l.peek()))
					return lexParagraphContinued // Means this is a hard break within the same paragraph
				}
			} else if hp(l.input[l.pos:], br) {
				// fmt.Printf("Found soft new line with following char: %q\n", l.peek())
				l.emit(typ)
				l.nextNTimes(len(br))
				l.emit(itemNewLine)
				l.acceptRun(" ")
				if hp(l.input[l.pos:], br) {
					l.emit(typ)
					l.nextNTimes(len(br))
					l.ignore()
					return lexBlock
				} else if isAlphaNumeric(l.peek()) {
					continue
				} else {
					return lexBlock
				}
			}
		}
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
		return lexNewParagraph
	}
	l.ignoreNext(1)
	switch n { // Map to item type
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
		typ = itemH6
	case 0:
		typ = itemError
	default:
		typ = itemH6
	}
	if typ == itemError {
		return l.errorf("Expected \"#\" at start of ATX header") // Send error & exit
	}
	l.ignore()
	l.acceptRun(inlineChars)
	l.emit(typ)
	if hp(l.input[l.pos:], br) {
		l.ignoreNext(len(br))
	} else {
		return l.errorf("Expected newline at end of ATX header")
	}
	return lexBlock
}

func lexUl(l *lexer) stateFn {
	fmt.Println(string(l.next()))
	if l.peek() != ' ' {
		return lexNewParagraph
	}
	for !hp(l.input[l.pos:], br) && l.peek() != eof {
		l.next()
	}
	l.emit(itemUl)
	l.nextNTimes(len(br))
	l.ignore()
	return lexBlock
}
// ============================================================ //
// ======================= END STATES ========================= //
// ============================================================ //
