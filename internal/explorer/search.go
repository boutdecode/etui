package explorer

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

const (
	walkLimit   = 50000 // plafond d'entrées conservées par le parcours
	resultLimit = 2000  // plafond de correspondances affichées
)

// SearchResultMsg porte le résultat d'un parcours asynchrone.
// Il est ignoré si Gen n'est plus la génération courante.
type SearchResultMsg struct {
	Gen       int
	Entries   []entry
	Truncated bool
}

type entry struct {
	path string
	dir  bool
}

// skipDir : dossiers jamais parcourus par la recherche.
func skipDir(name string) bool { return name == ".git" || name == "node_modules" }

// walk parcourt root récursivement. Il ne lit aucun état partagé et s'arrête
// quand ctx est annulé. Les erreurs de lecture sont ignorées silencieusement.
func walk(ctx context.Context, root string, showHidden bool, limit int) ([]entry, bool) {
	var out []entry
	truncated := false
	var rec func(dir string) bool
	rec = func(dir string) bool {
		if ctx.Err() != nil {
			return false
		}
		des, err := os.ReadDir(dir)
		if err != nil {
			return true
		}
		for _, de := range des {
			name := de.Name()
			if !showHidden && strings.HasPrefix(name, ".") {
				continue
			}
			if skipDir(name) && de.IsDir() {
				continue
			}
			if len(out) >= limit {
				truncated = true
				return false
			}
			p := filepath.Join(dir, name)
			isDir := de.IsDir()
			if de.Type()&os.ModeSymlink != 0 { // dossier lié : affiché comme dossier, jamais suivi
				if st, err := os.Stat(p); err == nil && st.IsDir() {
					out = append(out, entry{p, true})
					continue
				}
			}
			out = append(out, entry{p, isDir})
			if isDir && !rec(p) {
				return false
			}
		}
		return true
	}
	rec(root)
	return out, truncated
}

// matchRange renvoie la plage d'octets de la première occurrence de q dans
// name, insensible à la casse ; ok=false si absente. q vide ne correspond à rien.
func matchRange(name, q string) (start, end int, ok bool) {
	n := utf8.RuneCountInString(q)
	if n == 0 {
		return 0, 0, false
	}
	for i := range name {
		j, cnt := i, 0
		for cnt < n && j < len(name) {
			_, sz := utf8.DecodeRuneInString(name[j:])
			j += sz
			cnt++
		}
		if cnt < n {
			return 0, 0, false
		}
		if strings.EqualFold(name[i:j], q) {
			return i, j, true
		}
	}
	return 0, 0, false
}

// filter garde les entrées dont le nom (pas le chemin) contient query.
func filter(entries []entry, query string) []entry {
	var out []entry
	for _, e := range entries {
		if _, _, ok := matchRange(filepath.Base(e.path), query); ok {
			out = append(out, e)
		}
	}
	return out
}

// buildTree construit l'arbre éphémère (correspondances + ancêtres, tout déplié)
// et renvoie ses lignes aplaties, dossiers d'abord.
func buildTree(root string, matches []entry) []*node {
	top := &node{path: root, dir: true, expanded: true}
	nodes := map[string]*node{root: top}
	var ensure func(p string, dir bool) *node
	ensure = func(p string, dir bool) *node {
		if n, ok := nodes[p]; ok {
			return n
		}
		parent := ensure(filepath.Dir(p), true)
		n := &node{name: filepath.Base(p), path: p, dir: dir, expanded: dir, loaded: true, parent: parent, depth: parent.depth + 1}
		nodes[p] = n
		parent.children = append(parent.children, n)
		return n
	}
	for _, e := range matches {
		if rel, err := filepath.Rel(root, e.path); err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		n := ensure(e.path, e.dir)
		n.dir = n.dir || e.dir
	}
	var rows []*node
	var flat func(n *node)
	flat = func(n *node) {
		sort.SliceStable(n.children, func(i, j int) bool {
			a, b := n.children[i], n.children[j]
			if a.dir != b.dir {
				return a.dir
			}
			return strings.ToLower(a.name) < strings.ToLower(b.name)
		})
		for _, c := range n.children {
			rows = append(rows, c)
			flat(c)
		}
	}
	flat(top)
	return rows
}

// --- état de recherche du Model ---------------------------------------------

// Searching indique si la saisie de recherche est active.
func (m *Model) Searching() bool { return m.searching }

// Query renvoie la requête courante.
func (m *Model) Query() string { return m.query }

func (m *Model) swapView() {
	m.rows, m.treeRows = m.treeRows, m.rows
	m.cursor, m.treeCursor = m.treeCursor, m.cursor
	m.top, m.treeTop = m.treeTop, m.top
}

func (m *Model) startSearch() tea.Cmd {
	m.searching = true
	m.query = ""
	m.treeRows, m.treeCursor, m.treeTop = m.rows, m.cursor, m.top
	m.rows, m.cursor, m.top = nil, 0, 0
	m.entries, m.truncated = nil, false
	return m.scan()
}

// scan lance un parcours annulable (l'éventuel précédent est annulé).
func (m *Model) scan() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	m.gen++
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.scanning = true
	gen, root, hidden := m.gen, m.root.path, m.showHidden
	return func() tea.Msg {
		es, tr := walk(ctx, root, hidden, walkLimit)
		return SearchResultMsg{Gen: gen, Entries: es, Truncated: tr}
	}
}

// CancelSearch annule tout parcours en cours sans changer l'affichage.
func (m *Model) CancelSearch() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

// endSearch ferme la recherche et restaure l'arbre réel. Si reveal, le
// chemin sélectionné y est révélé.
func (m *Model) endSearch(reveal bool) {
	if !m.searching {
		return
	}
	target := ""
	if reveal {
		target = m.Selected()
	}
	m.CancelSearch()
	m.gen++
	m.swapView()
	m.searching, m.scanning, m.truncated = false, false, false
	m.query, m.entries, m.treeRows = "", nil, nil
	if target != "" {
		m.reveal(target)
	}
}

// reveal charge et déplie les ancêtres de path dans l'arbre réel puis place le curseur dessus.
func (m *Model) reveal(path string) {
	rel, err := filepath.Rel(m.root.path, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return
	}
	parts := strings.Split(rel, string(filepath.Separator))
	cur := m.root
	for _, part := range parts {
		if !cur.loaded {
			m.load(cur)
		}
		cur.expanded = true
		var next *node
		for _, c := range cur.children {
			if c.name == part {
				next = c
				break
			}
		}
		if next == nil {
			break
		}
		cur = next
	}
	m.rebuild()
	for i, r := range m.rows {
		if r.path == path {
			m.cursor = i
			break
		}
	}
	m.fixScroll()
}

// refilter recalcule l'arbre éphémère ; keep conserve la sélection si possible.
func (m *Model) refilter(keep bool) {
	sel := ""
	if keep {
		sel = m.Selected()
	}
	matches := filter(m.entries, m.query)
	if len(matches) > resultLimit {
		matches = matches[:resultLimit]
		m.truncated = true
	} else {
		m.truncated = m.truncatedWalk
	}
	m.rows = buildTree(m.root.path, matches)
	m.cursor, m.top = 0, 0
	if sel != "" {
		for i, r := range m.rows {
			if r.path == sel {
				m.cursor = i
				break
			}
		}
	}
	m.fixScroll()
}

func (m *Model) handleSearchMsg(r SearchResultMsg) {
	if !m.searching || r.Gen != m.gen {
		return
	}
	m.entries, m.truncatedWalk, m.scanning = r.Entries, r.Truncated, false
	m.refilter(true)
}

func (m *Model) updateSearch(k tea.KeyPressMsg) tea.Cmd {
	ks := k.Keystroke()
	switch ks {
	case "esc":
		m.endSearch(false)
		return nil
	case "enter":
		if len(m.rows) == 0 {
			return nil
		}
		n := m.rows[m.cursor]
		if n.dir {
			m.endSearch(true)
			return nil
		}
		m.endSearch(false)
		return openCmd(n.path)
	case "backspace":
		if _, sz := utf8.DecodeLastRuneInString(m.query); sz > 0 {
			m.query = m.query[:len(m.query)-sz]
			m.refilter(false)
		}
		return nil
	case "up":
		m.cursor--
	case "down":
		m.cursor++
	case "pgup":
		m.cursor -= m.bodyHeight()
	case "pgdown":
		m.cursor += m.bodyHeight()
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = len(m.rows) - 1
	case "f5":
		m.Reload()
		return m.scan()
	default:
		if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			m.query += k.Text
			m.refilter(false)
		}
		return nil
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.fixScroll()
	return nil
}

// EndSearch ferme la recherche sans révéler de chemin.
func (m *Model) EndSearch() { m.endSearch(false) }
