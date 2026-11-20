// Package editor implémente un widget d'édition de texte maison au-dessus d'un
// buffer de runes : curseur, sélection, annulation, recherche, coloration.
package editor

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/mattn/go-runewidth"

	"boutdecode/etui/internal/highlight"
	"boutdecode/etui/internal/i18n"
)

const (
	TabWidth      = 4
	highlightWait = 120 * time.Millisecond
)

// CutMsg supprime la sélection (le texte a déjà été copié par l'appelant).
type CutMsg struct{}

// HighlightMsg déclenche le recalcul différé de la coloration d'un onglet.
type HighlightMsg struct {
	ID      int
	Version int
}

// Editor est un onglet : un document, son curseur, son défilement et son état.
type Editor struct {
	ID   int
	Path string // chemin absolu, vide si sans nom

	buf    *buffer
	cur    Pos
	anchor Pos
	sel    bool
	wantX  int // colonne d'affichage souhaitée pour les déplacements verticaux

	top, leftCol  int // défilement (ligne, colonne d'affichage)
	width, height int // taille totale allouée (barre de recherche comprise)

	crlf bool

	hl       *highlight.Highlighter
	tokens   [][]chroma.TokenType
	version  int // incrémenté à chaque modification
	hlVerRun int // version colorée en dernier

	search searchState
}

// New crée un onglet vide.
func New(id int) *Editor {
	e := &Editor{ID: id, buf: newBuffer("")}
	e.hl = highlight.New("", "")
	return e
}

// Open charge un fichier. Un fichier inexistant donne un document vide
// qui sera créé à l'enregistrement.
func Open(id int, path string) (*Editor, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	e := New(id)
	e.Path = abs
	data, err := os.ReadFile(abs)
	switch {
	case errors.Is(err, os.ErrNotExist):
		// nouveau fichier
	case err != nil:
		return nil, err
	default:
		if bytes.IndexByte(data, 0) >= 0 {
			return nil, errors.New(i18n.T(i18n.ErrBinaryFile, filepath.Base(abs)))
		}
		text := string(data)
		if strings.Contains(text, "\r\n") {
			e.crlf = true
			text = strings.ReplaceAll(text, "\r\n", "\n")
		}
		e.buf.setText(text)
	}
	e.hl = highlight.New(abs, e.buf.text())
	e.recolor()
	return e, nil
}

// Save enregistre le document sur disque.
func (e *Editor) Save() error {
	if e.Path == "" {
		return errors.New(i18n.T(i18n.ErrNoName))
	}
	text := e.buf.text()
	if e.crlf {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(e.Path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.WriteFile(e.Path, []byte(text), mode); err != nil {
		return err
	}
	e.buf.markSaved()
	return nil
}

// Name est le nom court affiché dans l'onglet.
func (e *Editor) Name() string {
	if e.Path == "" {
		return i18n.T(i18n.NoName)
	}
	return filepath.Base(e.Path)
}

func (e *Editor) Modified() bool   { return e.buf.modified() }
func (e *Editor) Language() string { return e.hl.Language() }
func (e *Editor) Text() string     { return e.buf.text() }
func (e *Editor) Cursor() Pos      { return e.cur }

// SetSize fixe la taille allouée à l'éditeur.
func (e *Editor) SetSize(w, h int) {
	e.width, e.height = w, h
	e.scrollToCursor()
}

func (e *Editor) recolor() {
	e.tokens = e.hl.Tokens(e.buf.lines)
	e.hlVerRun = e.version
}

// --- sélection -------------------------------------------------------------

// Selection renvoie la sélection ordonnée, ok=false si vide.
func (e *Editor) Selection() (a, z Pos, ok bool) {
	if !e.sel || e.anchor == e.cur {
		return Pos{}, Pos{}, false
	}
	if e.cur.Less(e.anchor) {
		return e.cur, e.anchor, true
	}
	return e.anchor, e.cur, true
}

// SelectedText renvoie le texte sélectionné.
func (e *Editor) SelectedText() string {
	a, z, ok := e.Selection()
	if !ok {
		return ""
	}
	return e.buf.textRange(a, z)
}

func (e *Editor) SelectAll() {
	e.buf.breakGroup()
	e.anchor, e.cur, e.sel = Pos{}, e.buf.end(), true
	e.scrollToCursor()
}

// SetSelection sélectionne [a,z) et place le curseur en z.
func (e *Editor) SetSelection(a, z Pos) {
	e.buf.breakGroup()
	e.anchor, e.cur, e.sel = e.buf.clamp(a), e.buf.clamp(z), true
	e.scrollToCursor()
}

func (e *Editor) selectLine() {
	e.buf.breakGroup()
	l := e.cur.Line
	e.anchor = Pos{l, 0}
	if l+1 < len(e.buf.lines) {
		e.cur = Pos{l + 1, 0}
	} else {
		e.cur = Pos{l, e.buf.lineLen(l)}
	}
	e.sel = true
	e.scrollToCursor()
}

func (e *Editor) selectWord() {
	e.buf.breakGroup()
	a, z := e.buf.wordAt(e.cur)
	e.anchor, e.cur, e.sel = a, z, true
	e.scrollToCursor()
}

// --- édition ---------------------------------------------------------------

func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func (e *Editor) afterEdit() {
	e.version++
	e.sel = false
	e.search.dirty = true
	e.scrollToCursor()
}

// deleteSelection supprime la sélection ; renvoie true s'il y en avait une.
func (e *Editor) deleteSelection() bool {
	a, z, ok := e.Selection()
	if !ok {
		e.sel = false
		return false
	}
	e.cur = e.buf.replace(a, z, "", kindOther, e.cur)
	e.afterEdit()
	return true
}

// DeleteSelection est l'action « couper » une fois le texte copié.
func (e *Editor) DeleteSelection() bool { return e.deleteSelection() }

// Insert insère du texte au curseur en remplaçant la sélection.
func (e *Editor) Insert(s string) {
	s = normalize(s)
	if s == "" {
		return
	}
	kind := kindType
	if strings.Contains(s, "\n") || len([]rune(s)) > 1 {
		kind = kindOther
	}
	a, z, ok := e.Selection()
	if !ok {
		a, z = e.cur, e.cur
	}
	e.cur = e.buf.replace(a, z, s, kind, e.cur)
	e.afterEdit()
}

func (e *Editor) newline() {
	l := e.buf.lines[e.cur.Line]
	n := 0
	for n < len(l) && n < e.cur.Col && (l[n] == ' ' || l[n] == '\t') {
		n++
	}
	e.Insert("\n" + string(l[:n]))
}

func (e *Editor) backspace() {
	if e.deleteSelection() {
		return
	}
	if e.cur == (Pos{}) {
		return
	}
	from := e.cur
	if e.cur.Col > 0 {
		from.Col--
	} else {
		from = Pos{e.cur.Line - 1, e.buf.lineLen(e.cur.Line - 1)}
	}
	e.buf.replace(from, e.cur, "", kindBackspace, e.cur)
	e.cur = from
	e.afterEdit()
}

func (e *Editor) deleteForward() {
	if e.deleteSelection() {
		return
	}
	to := e.cur
	if e.cur.Col < e.buf.lineLen(e.cur.Line) {
		to.Col++
	} else if e.cur.Line+1 < len(e.buf.lines) {
		to = Pos{e.cur.Line + 1, 0}
	} else {
		return
	}
	e.buf.replace(e.cur, to, "", kindDelete, e.cur)
	e.afterEdit()
}

// Undo / Redo.
func (e *Editor) Undo() bool {
	p, ok := e.buf.undoEdit()
	if ok {
		e.cur = e.buf.clamp(p)
		e.afterEdit()
	}
	return ok
}

func (e *Editor) Redo() bool {
	p, ok := e.buf.redoEdit()
	if ok {
		e.cur = e.buf.clamp(p)
		e.afterEdit()
	}
	return ok
}

// --- déplacements ----------------------------------------------------------

// displayCol renvoie la colonne d'affichage de la rune col de la ligne l.
func (e *Editor) displayCol(l, col int) int {
	return displayCol(e.buf.lines[l], col)
}

func displayCol(line []rune, col int) int {
	x := 0
	for i := 0; i < col && i < len(line); i++ {
		x += runeWidth(line[i], x)
	}
	return x
}

func runeWidth(r rune, x int) int {
	if r == '\t' {
		return TabWidth - x%TabWidth
	}
	return runewidth.RuneWidth(r)
}

// colAtDisplay renvoie l'index de rune le plus proche de la colonne d'affichage x.
func colAtDisplay(line []rune, x int) int {
	cx := 0
	for i, r := range line {
		w := runeWidth(r, cx)
		if cx+w > x {
			if x-cx > w/2 && w > 1 {
				return i + 1
			}
			return i
		}
		cx += w
	}
	return len(line)
}

// moveTo déplace le curseur ; extend étend la sélection (Shift).
func (e *Editor) moveTo(p Pos, extend, keepWant bool) {
	e.buf.breakGroup()
	p = e.buf.clamp(p)
	if extend {
		if !e.sel {
			e.anchor, e.sel = e.cur, true
		}
	} else {
		e.sel = false
	}
	e.cur = p
	if !keepWant {
		e.wantX = e.displayCol(p.Line, p.Col)
	}
	e.scrollToCursor()
}

func (e *Editor) left(extend bool) {
	if !extend {
		if a, _, ok := e.Selection(); ok {
			e.moveTo(a, false, false)
			return
		}
	}
	p := e.cur
	if p.Col > 0 {
		p.Col--
	} else if p.Line > 0 {
		p = Pos{p.Line - 1, e.buf.lineLen(p.Line - 1)}
	}
	e.moveTo(p, extend, false)
}

func (e *Editor) right(extend bool) {
	if !extend {
		if _, z, ok := e.Selection(); ok {
			e.moveTo(z, false, false)
			return
		}
	}
	p := e.cur
	if p.Col < e.buf.lineLen(p.Line) {
		p.Col++
	} else if p.Line+1 < len(e.buf.lines) {
		p = Pos{p.Line + 1, 0}
	}
	e.moveTo(p, extend, false)
}

func (e *Editor) vertical(delta int, extend bool) {
	l := e.cur.Line + delta
	if l < 0 {
		e.moveTo(Pos{0, 0}, extend, false)
		return
	}
	if l >= len(e.buf.lines) {
		e.moveTo(e.buf.end(), extend, false)
		return
	}
	e.moveTo(Pos{l, colAtDisplay(e.buf.lines[l], e.wantX)}, extend, true)
}

func (e *Editor) textHeight() int {
	h := e.height - e.search.barHeight()
	if h < 1 {
		h = 1
	}
	return h
}

// home : « smart home », alterne début de texte / début de ligne.
func (e *Editor) home(extend bool) {
	l := e.buf.lines[e.cur.Line]
	n := 0
	for n < len(l) && (l[n] == ' ' || l[n] == '\t') {
		n++
	}
	c := n
	if e.cur.Col == n {
		c = 0
	}
	e.moveTo(Pos{e.cur.Line, c}, extend, false)
}

// --- défilement ------------------------------------------------------------

// GutterWidth est la largeur de la marge des numéros de ligne.
func (e *Editor) GutterWidth() int {
	return len(fmt.Sprint(len(e.buf.lines))) + 2
}

func (e *Editor) textWidth() int {
	w := e.width - e.GutterWidth()
	if w < 1 {
		w = 1
	}
	return w
}

func (e *Editor) scrollToCursor() {
	if e.width == 0 || e.height == 0 {
		return
	}
	h := e.textHeight()
	if e.cur.Line < e.top {
		e.top = e.cur.Line
	}
	if e.cur.Line >= e.top+h {
		e.top = e.cur.Line - h + 1
	}
	x := e.displayCol(e.cur.Line, e.cur.Col)
	w := e.textWidth()
	if x < e.leftCol {
		e.leftCol = x
	}
	if x >= e.leftCol+w-1 {
		e.leftCol = x - w + 2
	}
	if e.leftCol < 0 {
		e.leftCol = 0
	}
	e.clampTop()
}

func (e *Editor) clampTop() {
	max := len(e.buf.lines) - 1
	if e.top > max {
		e.top = max
	}
	if e.top < 0 {
		e.top = 0
	}
}

// --- touches ---------------------------------------------------------------

// Update traite un message ; le curseur et le contenu sont modifiés en place.
func (e *Editor) Update(msg tea.Msg) tea.Cmd {
	before := e.version
	switch m := msg.(type) {
	case HighlightMsg:
		if m.ID == e.ID && m.Version == e.version && e.hlVerRun != e.version {
			e.recolor()
		}
		return nil
	case CutMsg:
		e.deleteSelection()
	case tea.PasteMsg:
		if e.search.active() {
			e.search.paste(e, m.Content)
		} else {
			e.Insert(m.Content)
		}
	case tea.KeyPressMsg:
		if e.search.active() {
			e.searchKey(m)
		} else {
			e.key(m)
		}
	}
	e.refreshSearch()
	if e.version != before {
		id, v := e.ID, e.version
		return tea.Tick(highlightWait, func(time.Time) tea.Msg { return HighlightMsg{id, v} })
	}
	return nil
}

func (e *Editor) key(m tea.KeyPressMsg) {
	ks := m.Keystroke()
	switch ks {
	case "left", "shift+left":
		e.left(strings.HasPrefix(ks, "shift"))
	case "right", "shift+right":
		e.right(strings.HasPrefix(ks, "shift"))
	case "up", "shift+up":
		e.vertical(-1, strings.HasPrefix(ks, "shift"))
	case "down", "shift+down":
		e.vertical(1, strings.HasPrefix(ks, "shift"))
	case "pgup", "shift+pgup":
		e.vertical(-e.textHeight(), strings.HasPrefix(ks, "shift"))
	case "pgdown", "shift+pgdown":
		e.vertical(e.textHeight(), strings.HasPrefix(ks, "shift"))
	case "ctrl+left", "ctrl+shift+left", "alt+b":
		e.moveTo(e.buf.wordLeft(e.cur), strings.Contains(ks, "shift"), false)
	case "ctrl+right", "ctrl+shift+right", "alt+f":
		e.moveTo(e.buf.wordRight(e.cur), strings.Contains(ks, "shift"), false)
	case "home", "shift+home":
		e.home(strings.HasPrefix(ks, "shift"))
	case "end", "shift+end":
		e.moveTo(Pos{e.cur.Line, e.buf.lineLen(e.cur.Line)}, strings.HasPrefix(ks, "shift"), false)
	case "ctrl+home", "ctrl+shift+home":
		e.moveTo(Pos{}, strings.Contains(ks, "shift"), false)
	case "ctrl+end", "ctrl+shift+end":
		e.moveTo(e.buf.end(), strings.Contains(ks, "shift"), false)
	case "enter":
		e.newline()
	case "tab":
		e.Insert("\t")
	case "backspace", "ctrl+h":
		e.backspace()
	case "delete":
		e.deleteForward()
	case "ctrl+a":
		e.SelectAll()
	case "ctrl+l":
		e.selectLine()
	case "ctrl+d":
		e.selectWord()
	case "ctrl+z":
		e.Undo()
	case "ctrl+y", "ctrl+shift+z":
		e.Redo()
	case "ctrl+f":
		e.openSearch(false)
	case "ctrl+r":
		e.openSearch(true)
	case "f3":
		e.searchStep(1)
	case "shift+f3":
		e.searchStep(-1)
	default:
		if m.Text != "" && m.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			e.Insert(m.Text)
		}
	}
}
