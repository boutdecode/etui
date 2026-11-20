package editor

import (
	"strings"
	"unicode"
)

// Pos is a position in the document: line and column (rune index).
type Pos struct{ Line, Col int }

func (p Pos) Less(q Pos) bool {
	return p.Line < q.Line || (p.Line == q.Line && p.Col < q.Col)
}

// editKind is used to group consecutive keystrokes in the history.
type editKind int

const (
	kindOther editKind = iota
	kindType
	kindBackspace
	kindDelete
)

type edit struct {
	start    Pos
	del, ins string
	before   Pos // cursor before the edit
	after    Pos // cursor after the edit
	kind     editKind
}

// buffer: lines of runes + undo history.
type buffer struct {
	lines [][]rune

	undo, redo []edit
	lastKind   editKind
	savedLen   int // size of the undo stack at save time, -1 if unreachable
}

func newBuffer(text string) *buffer {
	b := &buffer{}
	b.setText(text)
	return b
}

func (b *buffer) setText(text string) {
	b.lines = splitLines(text)
	b.undo, b.redo = nil, nil
	b.lastKind = kindOther
	b.savedLen = 0
}

func splitLines(text string) [][]rune {
	parts := strings.Split(text, "\n")
	out := make([][]rune, len(parts))
	for i, p := range parts {
		out[i] = []rune(p)
	}
	return out
}

func (b *buffer) text() string {
	var sb strings.Builder
	for i, l := range b.lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(string(l))
	}
	return sb.String()
}

func (b *buffer) modified() bool { return b.savedLen != len(b.undo) }

func (b *buffer) markSaved() {
	b.savedLen = len(b.undo)
	b.lastKind = kindOther
}

func (b *buffer) lineLen(i int) int { return len(b.lines[i]) }

func (b *buffer) end() Pos {
	n := len(b.lines) - 1
	return Pos{n, len(b.lines[n])}
}

// clamp brings p back inside the document.
func (b *buffer) clamp(p Pos) Pos {
	if p.Line < 0 {
		return Pos{}
	}
	if p.Line >= len(b.lines) {
		return b.end()
	}
	if p.Col < 0 {
		p.Col = 0
	}
	if p.Col > len(b.lines[p.Line]) {
		p.Col = len(b.lines[p.Line])
	}
	return p
}

// textRange returns the text between a and b (a <= b).
func (b *buffer) textRange(a, z Pos) string {
	if a.Line == z.Line {
		return string(b.lines[a.Line][a.Col:z.Col])
	}
	var sb strings.Builder
	sb.WriteString(string(b.lines[a.Line][a.Col:]))
	for l := a.Line + 1; l < z.Line; l++ {
		sb.WriteByte('\n')
		sb.WriteString(string(b.lines[l]))
	}
	sb.WriteByte('\n')
	sb.WriteString(string(b.lines[z.Line][:z.Col]))
	return sb.String()
}

// endOf computes the end position of a text inserted at start.
func endOf(start Pos, s string) Pos {
	n := strings.Count(s, "\n")
	if n == 0 {
		return Pos{start.Line, start.Col + len([]rune(s))}
	}
	last := s[strings.LastIndexByte(s, '\n')+1:]
	return Pos{start.Line + n, len([]rune(last))}
}

// apply replaces [a,z) with ins without touching the history; it returns the end of the insertion.
func (b *buffer) apply(a, z Pos, ins string) Pos {
	insLines := splitLines(ins)
	head := b.lines[a.Line][:a.Col]
	tail := b.lines[z.Line][z.Col:]
	var repl [][]rune
	var end Pos
	if len(insLines) == 1 {
		l := make([]rune, 0, len(head)+len(insLines[0])+len(tail))
		l = append(append(append(l, head...), insLines[0]...), tail...)
		repl = [][]rune{l}
		end = Pos{a.Line, len(head) + len(insLines[0])}
	} else {
		first := append(append(make([]rune, 0, len(head)+len(insLines[0])), head...), insLines[0]...)
		lastIns := insLines[len(insLines)-1]
		last := append(append(make([]rune, 0, len(lastIns)+len(tail)), lastIns...), tail...)
		repl = append(repl, first)
		repl = append(repl, insLines[1:len(insLines)-1]...)
		repl = append(repl, last)
		end = Pos{a.Line + len(insLines) - 1, len(lastIns)}
	}
	out := make([][]rune, 0, len(b.lines)-(z.Line-a.Line)+len(repl)-1)
	out = append(out, b.lines[:a.Line]...)
	out = append(out, repl...)
	out = append(out, b.lines[z.Line+1:]...)
	b.lines = out
	return end
}

// replace replaces [a,z) with ins and records the edit. It returns the end of the insertion.
// before is the cursor position before the operation.
func (b *buffer) replace(a, z Pos, ins string, kind editKind, before Pos) Pos {
	del := b.textRange(a, z)
	end := b.apply(a, z, ins)
	e := edit{start: a, del: del, ins: ins, before: before, after: end, kind: kind}
	if b.savedLen > len(b.undo) { // the save point is overwritten
		b.savedLen = -1
	}
	b.redo = nil
	if !b.merge(e) {
		b.undo = append(b.undo, e)
	}
	b.lastKind = kind
	return end
}

// merge merges e into the last edit if they form a single typing group.
func (b *buffer) merge(e edit) bool {
	n := len(b.undo)
	if n == 0 || e.kind == kindOther || e.kind != b.lastKind || b.savedLen == n {
		return false
	}
	last := &b.undo[n-1]
	if last.kind != e.kind {
		return false
	}
	switch e.kind {
	case kindType:
		if e.del != "" || strings.Contains(e.ins, "\n") ||
			e.start != endOf(last.start, last.ins) {
			return false
		}
		// a space after a word starts a new group, for a more natural undo
		if r := []rune(e.ins); len(r) == 1 && unicode.IsSpace(r[0]) {
			return false
		}
		last.ins += e.ins
		last.after = e.after
		return true
	case kindBackspace:
		if last.ins != "" || e.ins != "" || endOf(e.start, e.del) != last.start {
			return false
		}
		last.start = e.start
		last.del = e.del + last.del
		last.after = e.after
		return true
	case kindDelete:
		if last.ins != "" || e.ins != "" || e.start != last.start {
			return false
		}
		last.del += e.del
		last.after = e.after
		return true
	}
	return false
}

// breakGroup interrupts grouping (cursor movement, etc.).
func (b *buffer) breakGroup() { b.lastKind = kindOther }

// undoEdit undoes the last edit; it returns the cursor position.
func (b *buffer) undoEdit() (Pos, bool) {
	n := len(b.undo)
	if n == 0 {
		return Pos{}, false
	}
	e := b.undo[n-1]
	b.undo = b.undo[:n-1]
	b.apply(e.start, endOf(e.start, e.ins), e.del)
	b.redo = append(b.redo, e)
	b.lastKind = kindOther
	return e.before, true
}

func (b *buffer) redoEdit() (Pos, bool) {
	n := len(b.redo)
	if n == 0 {
		return Pos{}, false
	}
	e := b.redo[n-1]
	b.redo = b.redo[:n-1]
	b.apply(e.start, endOf(e.start, e.del), e.ins)
	b.undo = append(b.undo, e)
	b.lastKind = kindOther
	return e.after, true
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// wordLeft: start of the previous word.
func (b *buffer) wordLeft(p Pos) Pos {
	if p.Col == 0 {
		if p.Line == 0 {
			return p
		}
		return Pos{p.Line - 1, len(b.lines[p.Line-1])}
	}
	l := b.lines[p.Line]
	c := p.Col
	for c > 0 && !isWordRune(l[c-1]) {
		c--
	}
	for c > 0 && isWordRune(l[c-1]) {
		c--
	}
	return Pos{p.Line, c}
}

// wordRight: end of the next word.
func (b *buffer) wordRight(p Pos) Pos {
	l := b.lines[p.Line]
	if p.Col >= len(l) {
		if p.Line == len(b.lines)-1 {
			return p
		}
		return Pos{p.Line + 1, 0}
	}
	c := p.Col
	for c < len(l) && !isWordRune(l[c]) {
		c++
	}
	for c < len(l) && isWordRune(l[c]) {
		c++
	}
	return Pos{p.Line, c}
}

// wordAt returns the bounds of the word (or run of non-word characters) under p.
func (b *buffer) wordAt(p Pos) (Pos, Pos) {
	l := b.lines[p.Line]
	if len(l) == 0 {
		return p, p
	}
	c := p.Col
	if c >= len(l) {
		c = len(l) - 1
	}
	in := isWordRune(l[c])
	s, e := c, c+1
	for s > 0 && isWordRune(l[s-1]) == in {
		s--
	}
	for e < len(l) && isWordRune(l[e]) == in {
		e++
	}
	return Pos{p.Line, s}, Pos{p.Line, e}
}
