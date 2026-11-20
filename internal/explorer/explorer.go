// Package explorer implémente un arbre de fichiers à chargement paresseux.
package explorer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"boutdecode/etui/internal/theme"
)

// OpenFileMsg demande l'ouverture d'un fichier.
type OpenFileMsg struct{ Path string }

type node struct {
	name     string
	path     string
	dir      bool
	expanded bool
	loaded   bool
	children []*node
	parent   *node
	depth    int
}

// Model est l'explorateur.
type Model struct {
	root       *node
	showHidden bool
	cursor     int
	top        int
	width      int
	height     int
	rows       []*node
	err        error
}

// New crée un explorateur enraciné en dir.
func New(dir string) *Model {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	m := &Model{root: &node{name: filepath.Base(abs), path: abs, dir: true, expanded: true}}
	m.load(m.root)
	m.rebuild()
	return m
}

func (m *Model) Root() string { return m.root.path }

// Err renvoie la dernière erreur de lecture de dossier.
func (m *Model) Err() error { return m.err }

func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.fixScroll()
}

func (m *Model) ShowHidden() bool { return m.showHidden }

// load (re)lit les enfants d'un dossier en conservant l'état des sous-dossiers déjà connus.
func (m *Model) load(n *node) {
	entries, err := os.ReadDir(n.path)
	n.loaded = true
	if err != nil {
		m.err = err
		n.children = nil
		return
	}
	old := map[string]*node{}
	for _, c := range n.children {
		old[c.name] = c
	}
	n.children = n.children[:0:0]
	for _, e := range entries {
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(n.path, e.Name())); err == nil && st.IsDir() {
				isDir = true
			}
		}
		c := &node{name: e.Name(), path: filepath.Join(n.path, e.Name()), dir: isDir, parent: n, depth: n.depth + 1}
		if prev, ok := old[c.name]; ok && prev.dir == c.dir {
			c.expanded, c.loaded, c.children = prev.expanded, prev.loaded, prev.children
			for _, gc := range c.children {
				gc.parent = c
			}
		}
		n.children = append(n.children, c)
	}
	sort.SliceStable(n.children, func(i, j int) bool {
		a, b := n.children[i], n.children[j]
		if a.dir != b.dir {
			return a.dir
		}
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	})
}

func (m *Model) rebuild() {
	var sel string
	if m.cursor < len(m.rows) {
		sel = m.rows[m.cursor].path
	}
	m.rows = m.rows[:0]
	var walk func(n *node)
	walk = func(n *node) {
		for _, c := range n.children {
			if !m.showHidden && strings.HasPrefix(c.name, ".") {
				continue
			}
			m.rows = append(m.rows, c)
			if c.dir && c.expanded {
				walk(c)
			}
		}
	}
	walk(m.root)
	if sel != "" {
		for i, r := range m.rows {
			if r.path == sel {
				m.cursor = i
				break
			}
		}
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.fixScroll()
}

func (m *Model) fixScroll() {
	h := m.bodyHeight()
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+h {
		m.top = m.cursor - h + 1
	}
	if m.top < 0 {
		m.top = 0
	}
}

func (m *Model) bodyHeight() int {
	if m.height-1 < 1 {
		return 1
	}
	return m.height - 1 // une ligne d'en-tête
}

// Selected renvoie le chemin sous le curseur ("" si vide).
func (m *Model) Selected() string {
	if m.cursor < len(m.rows) {
		return m.rows[m.cursor].path
	}
	return ""
}

// Visible renvoie les chemins affichés, dans l'ordre (utile aux tests).
func (m *Model) Visible() []string {
	out := make([]string, len(m.rows))
	for i, r := range m.rows {
		out[i] = r.path
	}
	return out
}

func (m *Model) toggle(n *node, expand bool) {
	if !n.dir {
		return
	}
	if expand && !n.loaded {
		m.load(n)
	}
	n.expanded = expand
	m.rebuild()
}

// Reload relit le disque en conservant l'état de l'arbre.
func (m *Model) Reload() {
	m.err = nil
	var walk func(n *node)
	walk = func(n *node) {
		m.load(n)
		for _, c := range n.children {
			if c.dir && c.expanded {
				walk(c)
			}
		}
	}
	walk(m.root)
	m.rebuild()
}

// Update traite une touche. Renvoie une commande d'ouverture de fichier le cas échéant.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok || len(m.rows) == 0 {
		if ok && k.Keystroke() == "." {
			m.showHidden = !m.showHidden
			m.rebuild()
		}
		return nil
	}
	n := m.rows[m.cursor]
	switch k.Keystroke() {
	case "up", "k":
		m.cursor--
	case "down", "j":
		m.cursor++
	case "pgup":
		m.cursor -= m.bodyHeight()
	case "pgdown":
		m.cursor += m.bodyHeight()
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(m.rows) - 1
	case "right", "l":
		if n.dir && !n.expanded {
			m.toggle(n, true)
		} else if n.dir && len(n.children) > 0 {
			m.cursor++
		} else if !n.dir {
			return openCmd(n.path)
		}
	case "enter", "space":
		if n.dir {
			m.toggle(n, !n.expanded)
		} else {
			return openCmd(n.path)
		}
	case "left", "h":
		if n.dir && n.expanded {
			m.toggle(n, false)
		} else if n.parent != nil && n.parent != m.root {
			for i, r := range m.rows {
				if r == n.parent {
					m.cursor = i
				}
			}
		}
	case ".":
		m.showHidden = !m.showHidden
		m.rebuild()
	case "f5":
		m.Reload()
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	m.fixScroll()
	return nil
}

func openCmd(p string) tea.Cmd { return func() tea.Msg { return OpenFileMsg{p} } }

// View rend l'explorateur sur width×height cellules.
func (m *Model) View(th theme.Theme, focused bool) string {
	w := m.width
	if w < 1 || m.height < 1 {
		return ""
	}
	pad := func(s string) string {
		s = runewidth.Truncate(s, w, "…")
		if d := w - runewidth.StringWidth(s); d > 0 {
			s += strings.Repeat(" ", d)
		}
		return s
	}
	head := lipgloss.NewStyle().Foreground(th.Accent).Bold(true).Render(pad(" " + strings.ToUpper(m.root.name)))
	lines := []string{head}
	selBg := th.ExplorerSelBgNF
	if focused {
		selBg = th.ExplorerSelBg
	}
	for i := 0; i < m.bodyHeight(); i++ {
		idx := m.top + i
		if idx >= len(m.rows) {
			lines = append(lines, strings.Repeat(" ", w))
			continue
		}
		n := m.rows[idx]
		icon := "  "
		if n.dir {
			icon = "▸ "
			if n.expanded {
				icon = "▾ "
			}
		}
		text := pad(" " + strings.Repeat("  ", n.depth-1) + icon + n.name)
		st := lipgloss.NewStyle()
		if n.dir {
			st = st.Foreground(th.ExplorerDirFg)
		} else if strings.HasPrefix(n.name, ".") {
			st = st.Foreground(th.Dim)
		}
		if idx == m.cursor {
			st = st.Background(selBg).Bold(focused)
		}
		lines = append(lines, st.Render(text))
	}
	return strings.Join(lines, "\n")
}
