package editor

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/mattn/go-runewidth"

	"boutdecode/etui/internal/theme"
)

// CursorPos est la position du curseur dans la zone rendue (relative à l'éditeur).
type CursorPos struct {
	X, Y    int
	Visible bool
}

// cell est une cellule d'affichage issue d'une rune (une tabulation donne plusieurs cellules).
type cell struct {
	s  string
	w  int
	ri int // index de la rune source
}

func expand(line []rune) []cell {
	cells := make([]cell, 0, len(line))
	x := 0
	for i, r := range line {
		if r == '\t' {
			w := TabWidth - x%TabWidth
			for k := 0; k < w; k++ {
				cells = append(cells, cell{" ", 1, i})
			}
			x += w
			continue
		}
		w := runewidth.RuneWidth(r)
		if r < 0x20 || r == 0x7f { // caractère de contrôle : affiché visiblement
			cells = append(cells, cell{"·", 1, i})
			x++
			continue
		}
		cells = append(cells, cell{string(r), w, i})
		x += w
	}
	return cells
}

// attr regroupe ce qui détermine le style d'une cellule.
// Ordre de superposition (défini ici, une seule fois) :
//
//  1. coloration syntaxique (premier plan)
//  2. ligne courante (fond)
//  3. sélection (fond)
//  4. occurrences de recherche (fond + premier plan), l'occurrence courante primant
type attr struct {
	tt      chroma.TokenType
	curLine bool
	sel     bool
	match   int // 0 aucune, 1 occurrence, 2 occurrence courante
}

func styleFor(th theme.Theme, a attr, cache map[attr]lipgloss.Style) lipgloss.Style {
	if st, ok := cache[a]; ok {
		return st
	}
	st := th.TokenStyle(a.tt)
	if a.curLine {
		st = st.Background(th.CurLineBg)
	}
	if a.sel {
		st = st.Background(th.SelBg)
	}
	switch a.match {
	case 1:
		st = st.Background(th.MatchBg).Foreground(th.MatchFg)
	case 2:
		st = st.Background(th.CurMatchBg).Foreground(th.MatchFg).Bold(true)
	}
	cache[a] = st
	return st
}

// View rend l'éditeur : zone de texte (marge + contenu) puis barre de recherche éventuelle.
// Chaque ligne fait exactement e.width colonnes.
func (e *Editor) View(th theme.Theme, focused bool) (string, CursorPos) {
	w, th_ := e.width, e.textHeight()
	if w <= 0 || e.height <= 0 {
		return "", CursorPos{}
	}
	gw := e.GutterWidth()
	tw := e.textWidth()
	digits := gw - 2
	cache := map[attr]lipgloss.Style{}

	selA, selZ, hasSel := e.Selection()
	matchIdx := map[int][]int{}
	if e.search.active() {
		for i, m := range e.search.matches {
			if m.Line >= e.top && m.Line < e.top+th_ {
				matchIdx[m.Line] = append(matchIdx[m.Line], i)
			}
		}
	}

	var out []string
	for row := 0; row < th_; row++ {
		ln := e.top + row
		if ln >= len(e.buf.lines) {
			g := lipgloss.NewStyle().Foreground(th.LineNum).Render(fmt.Sprintf("%*s ", digits+1, "~"))
			out = append(out, g+strings.Repeat(" ", tw))
			continue
		}
		isCur := ln == e.cur.Line
		numStyle := lipgloss.NewStyle().Foreground(th.LineNum)
		if isCur {
			numStyle = lipgloss.NewStyle().Foreground(th.LineNumCur).Bold(true)
		}
		gutter := numStyle.Render(fmt.Sprintf("%*d ", digits+1, ln+1))

		line := e.buf.lines[ln]
		var toks []chroma.TokenType
		if e.tokens != nil && ln < len(e.tokens) && len(e.tokens[ln]) == len(line) {
			toks = e.tokens[ln]
		}
		cells := expand(line)
		if hasSel && ln >= selA.Line && ln < selZ.Line { // la sélection englobe le retour à la ligne
			cells = append(cells, cell{" ", 1, len(line)})
		}

		attrAt := func(ri int) attr {
			a := attr{tt: chroma.Text, curLine: isCur && focused && !e.sel}
			if ri < len(toks) {
				a.tt = toks[ri]
			}
			if hasSel {
				p := Pos{ln, ri}
				if !p.Less(selA) && p.Less(selZ) {
					a.sel = true
				}
			}
			for _, mi := range matchIdx[ln] {
				m := e.search.matches[mi]
				if ri >= m.Start && ri < m.End {
					a.match = 1
					if mi == e.search.idx {
						a.match = 2
					}
				}
			}
			return a
		}

		var sb strings.Builder
		var run strings.Builder
		var runAttr attr
		have := false
		flush := func() {
			if have {
				sb.WriteString(styleFor(th, runAttr, cache).Render(run.String()))
				run.Reset()
				have = false
			}
		}
		emit := func(s string, a attr) {
			if have && a != runAttr {
				flush()
			}
			runAttr, have = a, true
			run.WriteString(s)
		}

		x, used := 0, 0
		for _, c := range cells {
			x0 := x
			x += c.w
			if x0 < e.leftCol {
				if x > e.leftCol { // caractère large coupé à gauche
					emit(" ", attrAt(c.ri))
					used++
				}
				continue
			}
			if x > e.leftCol+tw {
				if x0 < e.leftCol+tw { // coupé à droite
					emit(" ", attrAt(c.ri))
					used++
				}
				break
			}
			emit(c.s, attrAt(c.ri))
			used += c.w
		}
		flush()
		if used < tw {
			pad := attr{tt: chroma.Text, curLine: isCur && focused && !e.sel}
			sb.WriteString(styleFor(th, pad, cache).Render(strings.Repeat(" ", tw-used)))
		}
		out = append(out, gutter+sb.String())
	}

	var cur CursorPos
	if focused && !e.search.active() {
		cx := gw + e.displayCol(e.cur.Line, e.cur.Col) - e.leftCol
		cy := e.cur.Line - e.top
		if cx >= gw && cx < w && cy >= 0 && cy < th_ {
			cur = CursorPos{cx, cy, true}
		}
	}

	if e.search.active() {
		bar, bx, by := e.searchBar(th, w)
		out = append(out, bar...)
		if focused {
			cur = CursorPos{bx, th_ + by, true}
		}
	}
	return strings.Join(out, "\n"), cur
}

func fit(s string, w int) string {
	s = runewidth.Truncate(s, w, "…")
	if pad := w - runewidth.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// searchBar rend la barre de recherche et renvoie la position du curseur dans celle-ci.
func (e *Editor) searchBar(th theme.Theme, w int) ([]string, int, int) {
	s := &e.search
	st := lipgloss.NewStyle().Background(th.StatusBg).Foreground(th.StatusFg)
	acc := st.Bold(true)

	cur, total := e.SearchInfo()
	info := "0/0"
	if total > 0 {
		info = fmt.Sprintf("%d/%d", max(cur, 0), total)
	}
	cs := "aA"
	if s.caseSens {
		cs = "Aa"
	}
	label1 := " Rechercher : "
	right := fmt.Sprintf(" %s  [%s]  Entrée↓ Maj+Entrée↑ Alt+C casse Échap ", info, cs)
	room := w - runewidth.StringWidth(label1) - runewidth.StringWidth(right)
	q := string(s.query)
	if room < 1 {
		right, room = "", w-runewidth.StringWidth(label1)
	}
	if room < 1 {
		room = 1
	}
	qv := runewidth.TruncateLeft(q, max(0, runewidth.StringWidth(q)-room+1), "")
	line1 := acc.Render(label1) + st.Render(fit(qv, room)+right)
	if lw := lipgloss.Width(line1); lw < w {
		line1 += st.Render(strings.Repeat(" ", w-lw))
	}
	bars := []string{line1}
	cx, cy := runewidth.StringWidth(label1)+runewidth.StringWidth(qv), 0

	if s.mode == 2 {
		label2 := " Remplacer  : "
		hint := " Tab champ · Entrée remplace · Alt+A tout "
		room2 := w - runewidth.StringWidth(label2) - runewidth.StringWidth(hint)
		if room2 < 1 {
			hint, room2 = "", w-runewidth.StringWidth(label2)
		}
		if room2 < 1 {
			room2 = 1
		}
		rv := runewidth.TruncateLeft(string(s.repl), max(0, runewidth.StringWidth(string(s.repl))-room2+1), "")
		line2 := acc.Render(label2) + st.Render(fit(rv, room2)+hint)
		if lw := lipgloss.Width(line2); lw < w {
			line2 += st.Render(strings.Repeat(" ", w-lw))
		}
		bars = append(bars, line2)
		if s.field == 1 {
			cx, cy = runewidth.StringWidth(label2)+runewidth.StringWidth(rv), 1
		}
	}
	return bars, min(cx, w-1), cy
}
