package editor

import (
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"boutdecode/etui/internal/i18n"
)

// Range désigne une occurrence sur une seule ligne : [Start, End) en runes.
type Range struct{ Line, Start, End int }

type searchState struct {
	mode     int // 0 fermé, 1 recherche, 2 recherche + remplacement
	query    []rune
	repl     []rune
	field    int // 0 = recherche, 1 = remplacement
	caseSens bool
	matches  []Range
	idx      int // occurrence courante, -1 si aucune
	dirty    bool
	notice   string
}

func (s *searchState) active() bool { return s.mode != 0 }

func (s *searchState) barHeight() int {
	switch s.mode {
	case 1:
		return 1
	case 2:
		return 2
	}
	return 0
}

func (s *searchState) paste(e *Editor, text string) {
	text = strings.ReplaceAll(normalize(text), "\n", " ")
	if s.field == 1 {
		s.repl = append(s.repl, []rune(text)...)
		return
	}
	s.query = append(s.query, []rune(text)...)
	e.searchRecompute()
	e.searchJumpFrom(e.searchFrom())
}

func fold(r rune, caseSens bool) rune {
	if caseSens {
		return r
	}
	return unicode.ToLower(r)
}

// findAll liste toutes les occurrences de query, chevauchements compris.
func findAll(lines [][]rune, query []rune, caseSens bool) []Range {
	if len(query) == 0 {
		return nil
	}
	q := make([]rune, len(query))
	for i, r := range query {
		q[i] = fold(r, caseSens)
	}
	var out []Range
	for li, l := range lines {
		for i := 0; i+len(q) <= len(l); i++ {
			ok := true
			for j := range q {
				if fold(l[i+j], caseSens) != q[j] {
					ok = false
					break
				}
			}
			if ok {
				out = append(out, Range{li, i, i + len(q)})
			}
		}
	}
	return out
}

// Matches renvoie les occurrences courantes (vide si la recherche est fermée).
func (e *Editor) Matches() []Range {
	if !e.search.active() {
		return nil
	}
	return e.search.matches
}

// SearchInfo : numéro de l'occurrence courante (1-based, 0 si aucune) et total.
func (e *Editor) SearchInfo() (cur, total int) {
	return e.search.idx + 1, len(e.search.matches)
}

// SearchActive indique si la barre de recherche est ouverte.
func (e *Editor) SearchActive() bool { return e.search.active() }

// TakeNotice renvoie et efface le dernier message informatif de l'éditeur.
func (e *Editor) TakeNotice() string {
	n := e.search.notice
	e.search.notice = ""
	return n
}

func (e *Editor) searchRecompute() {
	s := &e.search
	s.matches = findAll(e.buf.lines, s.query, s.caseSens)
	s.dirty = false
	if len(s.matches) == 0 {
		s.idx = -1
	} else if s.idx >= len(s.matches) {
		s.idx = len(s.matches) - 1
	}
}

// refreshSearch recalcule les occurrences si le texte a changé.
func (e *Editor) refreshSearch() {
	if e.search.active() && e.search.dirty {
		e.searchRecompute()
	}
}

func (e *Editor) openSearch(replace bool) {
	s := &e.search
	wasOpen := s.active()
	s.mode = 1
	if replace {
		s.mode = 2
	}
	if t := e.SelectedText(); t != "" && !strings.Contains(t, "\n") && !wasOpen {
		s.query = []rune(t)
	}
	if !replace {
		s.field = 0
	}
	s.idx = -1
	e.searchRecompute()
	e.searchJumpFrom(e.searchFrom())
	e.scrollToCursor()
}

func (e *Editor) closeSearch() {
	e.search.mode = 0
	e.search.field = 0
	e.scrollToCursor()
}

// searchFrom : position à partir de laquelle chercher l'occurrence suivante.
func (e *Editor) searchFrom() Pos {
	if a, _, ok := e.Selection(); ok {
		return a
	}
	return e.cur
}

// searchJumpFrom saute à la première occurrence commençant en p ou après (circulaire).
func (e *Editor) searchJumpFrom(p Pos) {
	s := &e.search
	if len(s.matches) == 0 {
		s.idx = -1
		return
	}
	for i, m := range s.matches {
		if !(Pos{m.Line, m.Start}).Less(p) {
			e.searchJump(i)
			return
		}
	}
	e.searchJump(0)
}

func (e *Editor) searchJump(i int) {
	s := &e.search
	s.idx = i
	m := s.matches[i]
	e.buf.breakGroup()
	e.anchor, e.cur, e.sel = Pos{m.Line, m.Start}, Pos{m.Line, m.End}, true
	e.wantX = e.displayCol(m.Line, m.End)
	e.scrollToCursor()
}

// searchStep passe à l'occurrence suivante (dir=1) ou précédente (dir=-1), circulaire.
func (e *Editor) searchStep(dir int) {
	e.refreshSearch()
	s := &e.search
	n := len(s.matches)
	if n == 0 {
		s.notice = i18n.T(i18n.SearchNoMatch)
		return
	}
	if s.idx < 0 {
		if dir > 0 {
			e.searchJumpFrom(e.searchFrom())
		} else {
			// dernière occurrence avant le curseur
			p := e.searchFrom()
			j := n - 1
			for i := n - 1; i >= 0; i-- {
				if (Pos{s.matches[i].Line, s.matches[i].Start}).Less(p) {
					j = i
					break
				}
			}
			e.searchJump(j)
		}
		return
	}
	e.searchJump(((s.idx+dir)%n + n) % n)
}

// replaceCurrent remplace l'occurrence courante puis passe à la suivante.
func (e *Editor) replaceCurrent() {
	e.refreshSearch()
	s := &e.search
	if s.idx < 0 || s.idx >= len(s.matches) {
		e.searchStep(1)
		return
	}
	m := s.matches[s.idx]
	a, z := Pos{m.Line, m.Start}, Pos{m.Line, m.End}
	end := e.buf.replace(a, z, string(s.repl), kindOther, e.cur)
	e.cur = end
	e.afterEdit()
	e.searchRecompute()
	s.idx = -1
	e.searchJumpFrom(end)
}

func (e *Editor) replaceAll() {
	e.refreshSearch()
	s := &e.search
	n := len(s.matches)
	if n == 0 {
		s.notice = i18n.T(i18n.SearchNoMatch)
		return
	}
	before := e.cur
	for i := n - 1; i >= 0; i-- { // de la fin vers le début : les positions restent valides
		m := s.matches[i]
		e.buf.replace(Pos{m.Line, m.Start}, Pos{m.Line, m.End}, string(s.repl), kindOther, before)
	}
	e.cur = e.buf.clamp(before)
	e.afterEdit()
	e.searchRecompute()
	s.notice = i18n.T(i18n.SearchReplaced, n)
}

func (e *Editor) searchKey(m tea.KeyPressMsg) {
	s := &e.search
	field := &s.query
	if s.field == 1 {
		field = &s.repl
	}
	ks := m.Keystroke()
	switch ks {
	case "esc", "ctrl+c":
		e.closeSearch()
	case "enter":
		if s.field == 1 {
			e.replaceCurrent()
		} else {
			e.searchStep(1)
		}
	case "shift+enter", "up", "shift+f3":
		e.searchStep(-1)
	case "down", "f3":
		e.searchStep(1)
	case "tab", "shift+tab":
		if s.mode == 2 {
			s.field = 1 - s.field
		}
	case "backspace":
		if len(*field) > 0 {
			*field = (*field)[:len(*field)-1]
			if s.field == 0 {
				e.searchQueryChanged()
			}
		}
	case "ctrl+u":
		*field = nil
		if s.field == 0 {
			e.searchQueryChanged()
		}
	case "alt+c":
		s.caseSens = !s.caseSens
		e.searchQueryChanged()
	case "alt+a":
		if s.mode == 2 {
			e.replaceAll()
		}
	case "ctrl+f":
		e.openSearch(false)
	case "ctrl+r":
		e.openSearch(true)
		s.field = 1
	default:
		if m.Text != "" && m.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			*field = append(*field, []rune(m.Text)...)
			if s.field == 0 {
				e.searchQueryChanged()
			}
		}
	}
}

// searchQueryChanged met à jour les occurrences et saute à la plus proche (recherche incrémentale).
func (e *Editor) searchQueryChanged() {
	e.searchRecompute()
	e.search.idx = -1
	e.searchJumpFrom(e.searchFrom())
}
