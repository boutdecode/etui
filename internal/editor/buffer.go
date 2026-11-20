package editor

import (
	"strings"
	"unicode"
)

// Pos est une position dans le document : ligne et colonne (index de rune).
type Pos struct{ Line, Col int }

func (p Pos) Less(q Pos) bool {
	return p.Line < q.Line || (p.Line == q.Line && p.Col < q.Col)
}

// editKind sert à regrouper les frappes consécutives dans l'historique.
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
	before   Pos // curseur avant l'édition
	after    Pos // curseur après l'édition
	kind     editKind
}

// buffer : lignes de runes + historique d'annulation.
type buffer struct {
	lines [][]rune

	undo, redo []edit
	lastKind   editKind
	savedLen   int // taille de la pile d'annulation à l'enregistrement, -1 si inatteignable
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

// clamp ramène p dans le document.
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

// textRange renvoie le texte entre a et b (a <= b).
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

// endOf calcule la position de fin d'un texte inséré en start.
func endOf(start Pos, s string) Pos {
	n := strings.Count(s, "\n")
	if n == 0 {
		return Pos{start.Line, start.Col + len([]rune(s))}
	}
	last := s[strings.LastIndexByte(s, '\n')+1:]
	return Pos{start.Line + n, len([]rune(last))}
}

// apply remplace [a,z) par ins sans toucher à l'historique ; renvoie la fin de l'insertion.
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

// replace remplace [a,z) par ins en enregistrant l'édition. Renvoie la fin de l'insertion.
// before est la position du curseur avant l'opération.
func (b *buffer) replace(a, z Pos, ins string, kind editKind, before Pos) Pos {
	del := b.textRange(a, z)
	end := b.apply(a, z, ins)
	e := edit{start: a, del: del, ins: ins, before: before, after: end, kind: kind}
	if b.savedLen > len(b.undo) { // le point d'enregistrement est écrasé
		b.savedLen = -1
	}
	b.redo = nil
	if !b.merge(e) {
		b.undo = append(b.undo, e)
	}
	b.lastKind = kind
	return end
}

// merge fusionne e dans la dernière édition si elles forment un même groupe de frappe.
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
		// un espace après un mot ouvre un nouveau groupe, pour un undo plus naturel
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

// breakGroup interrompt le regroupement (déplacement du curseur, etc.).
func (b *buffer) breakGroup() { b.lastKind = kindOther }

// undoEdit annule la dernière édition ; renvoie la position du curseur.
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

// wordLeft : début du mot précédent.
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

// wordRight : fin du mot suivant.
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

// wordAt renvoie les bornes du mot (ou de la suite de non-mots) sous p.
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
