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
	walkLimit   = 50000 // cap on entries kept by the walk
	resultLimit = 2000  // cap on displayed matches
)

// SearchResultMsg carries the result of an asynchronous walk.
// It is ignored if Gen is no longer the current generation.
type SearchResultMsg struct {
	Gen       int
	Entries   []entry
	Truncated bool
}

type entry struct {
	path string
	dir  bool
}

// skipDir: directories the search never walks into.
func skipDir(name string) bool { return name == ".git" || name == "node_modules" }

// walk walks root recursively. It reads no shared state and stops
// when ctx is cancelled. Read errors are silently ignored.
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
			if de.Type()&os.ModeSymlink != 0 { // symlinked directory: shown as a directory, never followed
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

// matchRange returns the byte range of the first occurrence of q in
// name, case-insensitively; ok=false if absent. An empty q matches nothing.
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

// filter keeps the entries whose name (not path) contains query.
func filter(entries []entry, query string) []entry {
	var out []entry
	for _, e := range entries {
		if _, _, ok := matchRange(filepath.Base(e.path), query); ok {
			out = append(out, e)
		}
	}
	return out
}

// buildTree builds the ephemeral tree (matches + ancestors, fully expanded)
// and returns its flattened rows, directories first.
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

// --- search state of the Model -----------------------------------------------

// Searching reports whether search input is active.
func (m *Model) Searching() bool { return m.searching }

// Query returns the current query.
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

// scan starts a cancellable walk (any previous one is cancelled).
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

// CancelSearch cancels any walk in progress without changing the display.
func (m *Model) CancelSearch() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

// endSearch closes the search and restores the real tree. If reveal is set, the
// selected path is revealed in it.
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

// reveal loads and expands the ancestors of path in the real tree, then puts the cursor on it.
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

// refilter recomputes the ephemeral tree; keep preserves the selection if possible.
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

// EndSearch closes the search without revealing a path.
func (m *Model) EndSearch() { m.endSearch(false) }
