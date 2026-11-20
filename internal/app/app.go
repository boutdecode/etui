// Package app contient le modèle racine : disposition, focus, onglets, presse-papiers.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"boutdecode/etui/internal/clip"
	"boutdecode/etui/internal/editor"
	"boutdecode/etui/internal/explorer"
	"boutdecode/etui/internal/theme"
)

type focus int

const (
	focusEditor focus = iota
	focusExplorer
)

type promptKind int

const (
	promptNone promptKind = iota
	promptClose
	promptQuit
)

// Options configure le démarrage.
type Options struct {
	Dir   string   // racine de l'explorateur
	Files []string // fichiers à ouvrir au démarrage
	Mode  theme.Mode
	Clip  clip.Clipboard // presse-papiers (système par défaut)
}

// Model est le modèle racine Bubble Tea.
type Model struct {
	explorer *explorer.Model
	tabs     []*editor.Editor
	active   int
	focus    focus
	showTree bool

	mode     theme.Mode
	bgDark   bool
	th       theme.Theme
	clip     clip.Clipboard
	register string // repli quand le presse-papiers système est indisponible

	width, height int
	status        string
	statusErr     bool
	prompt        promptKind
	promptTab     int
	help          bool
	nextID        int
}

// New construit le modèle.
func New(o Options) *Model {
	if o.Dir == "" {
		o.Dir = "."
	}
	if o.Clip == nil {
		o.Clip = clip.System{}
	}
	m := &Model{
		explorer: explorer.New(o.Dir),
		showTree: true,
		mode:     o.Mode,
		bgDark:   true, // repli si le terminal ne répond pas à la requête de fond
		clip:     o.Clip,
		focus:    focusExplorer,
	}
	m.th = theme.Resolve(m.mode, m.bgDark)
	for _, f := range o.Files {
		m.openFile(f)
	}
	return m
}

// Init demande la couleur de fond du terminal pour choisir le thème.
func (m *Model) Init() tea.Cmd { return tea.RequestBackgroundColor }

// --- accesseurs (tests) ----------------------------------------------------

func (m *Model) ActiveEditor() *editor.Editor {
	if m.active >= 0 && m.active < len(m.tabs) {
		return m.tabs[m.active]
	}
	return nil
}
func (m *Model) Tabs() []*editor.Editor { return m.tabs }
func (m *Model) Status() string         { return m.status }
func (m *Model) Theme() theme.Theme     { return m.th }

// --- mise en page ----------------------------------------------------------

func (m *Model) explorerWidth() int {
	if !m.showTree || m.width < 50 {
		return 0
	}
	w := m.width / 4
	if w < 20 {
		w = 20
	}
	if w > 36 {
		w = 36
	}
	return w
}

func (m *Model) editorWidth() int {
	w := m.width
	if ew := m.explorerWidth(); ew > 0 {
		w -= ew + 1
	}
	return w
}

func (m *Model) layout() {
	if m.width == 0 {
		return
	}
	m.explorer.SetSize(m.explorerWidth(), m.height-1)
	for _, t := range m.tabs {
		t.SetSize(m.editorWidth(), max(1, m.height-2))
	}
}

// --- ouverture / fermeture -------------------------------------------------

func (m *Model) setStatus(s string, isErr bool) { m.status, m.statusErr = s, isErr }

func (m *Model) openFile(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	for i, t := range m.tabs {
		if t.Path == abs {
			m.active, m.focus = i, focusEditor
			return
		}
	}
	if st, err := os.Stat(abs); err == nil && st.IsDir() {
		m.setStatus(abs+" est un dossier", true)
		return
	}
	m.nextID++
	ed, err := editor.Open(m.nextID, abs)
	if err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	ed.SetSize(m.editorWidth(), max(1, m.height-2))
	m.tabs = append(m.tabs, ed)
	m.active, m.focus = len(m.tabs)-1, focusEditor
}

func (m *Model) closeTab(i int) {
	if i < 0 || i >= len(m.tabs) {
		return
	}
	m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
	switch {
	case len(m.tabs) == 0:
		m.active, m.focus = 0, focusExplorer
	case m.active > i || m.active >= len(m.tabs):
		m.active = max(0, min(m.active-1, len(m.tabs)-1))
	}
}

func (m *Model) switchTab(delta int) {
	if n := len(m.tabs); n > 0 {
		m.active = ((m.active+delta)%n + n) % n
	}
}

func (m *Model) modifiedTabs() []*editor.Editor {
	var out []*editor.Editor
	for _, t := range m.tabs {
		if t.Modified() {
			out = append(out, t)
		}
	}
	return out
}

func (m *Model) save(t *editor.Editor) bool {
	if err := t.Save(); err != nil {
		m.setStatus("Échec de l'enregistrement : "+err.Error(), true)
		return false
	}
	m.setStatus("Enregistré : "+t.Name(), false)
	m.explorer.Reload()
	return true
}

// --- presse-papiers --------------------------------------------------------

func (m *Model) copyText(text string) tea.Cmd {
	m.register = text
	if err := m.clip.Write(text); err != nil {
		// repli OSC52 : fonctionne à travers SSH dans la plupart des terminaux modernes
		return tea.SetClipboard(text)
	}
	return nil
}

func (m *Model) pasteText() string {
	text, err := m.clip.Read()
	if err != nil {
		return m.register
	}
	return text
}

// --- Update ----------------------------------------------------------------

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil
	case tea.BackgroundColorMsg:
		m.bgDark = msg.IsDark()
		m.th = theme.Resolve(m.mode, m.bgDark)
		return m, nil
	case explorer.OpenFileMsg:
		m.openFile(msg.Path)
		return m, nil
	case editor.HighlightMsg:
		for _, t := range m.tabs {
			if t.ID == msg.ID {
				return m, t.Update(msg)
			}
		}
		return m, nil
	case tea.PasteMsg:
		if ed := m.ActiveEditor(); ed != nil && m.focus == focusEditor && m.prompt == promptNone {
			return m, m.forward(ed, msg)
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) forward(ed *editor.Editor, msg tea.Msg) tea.Cmd {
	cmd := ed.Update(msg)
	if n := ed.TakeNotice(); n != "" {
		m.setStatus(n, false)
	}
	return cmd
}

func (m *Model) handleKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.setStatus("", false)
	if m.prompt != promptNone {
		return m.handlePrompt(k)
	}
	if m.help {
		m.help = false
		return m, nil
	}
	ed := m.ActiveEditor()
	ks := k.Keystroke()

	switch ks {
	case "ctrl+q":
		if mod := m.modifiedTabs(); len(mod) > 0 {
			m.prompt = promptQuit
			return m, nil
		}
		return m, tea.Quit
	case "f1":
		m.help = true
		return m, nil
	case "ctrl+t":
		m.mode = m.mode.Next()
		m.th = theme.Resolve(m.mode, m.bgDark)
		m.setStatus("Thème : "+m.mode.String(), false)
		return m, nil
	case "ctrl+e":
		m.showTree = !m.showTree
		if !m.showTree && m.focus == focusExplorer && ed != nil {
			m.focus = focusEditor
		}
		m.layout()
		return m, nil
	case "ctrl+b":
		switch {
		case m.focus == focusEditor || !m.showTree:
			m.showTree = true
			m.focus = focusExplorer
			m.layout()
		case ed != nil:
			m.focus = focusEditor
		}
		return m, nil
	case "ctrl+w":
		if ed == nil {
			return m, nil
		}
		if ed.Modified() {
			m.prompt, m.promptTab = promptClose, m.active
			return m, nil
		}
		m.closeTab(m.active)
		return m, nil
	case "ctrl+s":
		if ed != nil {
			m.save(ed)
		}
		return m, nil
	case "alt+left", "ctrl+pgup":
		m.switchTab(-1)
		return m, nil
	case "alt+right", "ctrl+pgdown":
		m.switchTab(1)
		return m, nil
	}
	if strings.HasPrefix(ks, "alt+") && len(ks) == 5 && ks[4] >= '1' && ks[4] <= '9' {
		if i := int(ks[4] - '1'); i < len(m.tabs) {
			m.active = i
		}
		return m, nil
	}

	if m.focus == focusExplorer {
		if ks == "esc" && ed != nil {
			m.focus = focusEditor
			return m, nil
		}
		return m, m.explorer.Update(k)
	}
	if ed == nil {
		return m, nil
	}

	switch ks {
	case "ctrl+c":
		text := ed.SelectedText()
		if text == "" {
			m.setStatus("Rien à copier", false)
			return m, nil
		}
		return m, m.copyText(text)
	case "ctrl+x":
		text := ed.SelectedText()
		if text == "" || ed.SearchActive() {
			return m, nil
		}
		cmd := m.copyText(text)
		return m, tea.Batch(cmd, m.forward(ed, editor.CutMsg{}))
	case "ctrl+v":
		text := m.pasteText()
		if text == "" {
			return m, nil
		}
		return m, m.forward(ed, tea.PasteMsg{Content: text})
	}
	return m, m.forward(ed, k)
}

func (m *Model) handlePrompt(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	kind := m.prompt
	m.prompt = promptNone
	ks := k.Keystroke()
	yes := ks == "o" || ks == "y" || ks == "enter"
	no := ks == "n"
	switch kind {
	case promptClose:
		if m.promptTab >= len(m.tabs) {
			return m, nil
		}
		t := m.tabs[m.promptTab]
		switch {
		case yes:
			if m.save(t) {
				m.closeTab(m.promptTab)
			}
		case no:
			m.closeTab(m.promptTab)
		}
	case promptQuit:
		switch {
		case yes:
			for _, t := range m.modifiedTabs() {
				if !m.save(t) {
					return m, nil
				}
			}
			return m, tea.Quit
		case no:
			return m, tea.Quit
		}
	}
	return m, nil
}

// --- View ------------------------------------------------------------------

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = runewidth.Truncate(s, w, "…")
	if d := w - runewidth.StringWidth(s); d > 0 {
		s += strings.Repeat(" ", d)
	}
	return s
}

func (m *Model) tabBar(w int) string {
	th := m.th
	if len(m.tabs) == 0 {
		return lipgloss.NewStyle().Background(th.TabBg).Render(strings.Repeat(" ", w))
	}
	labels := make([]string, len(m.tabs))
	for i, t := range m.tabs {
		name := runewidth.Truncate(t.Name(), 24, "…")
		mark := " "
		if t.Modified() {
			mark = "●"
		}
		labels[i] = " " + name + " " + mark + " "
	}
	first, total := 0, 0
	for i := 0; i <= m.active; i++ {
		total += runewidth.StringWidth(labels[i])
	}
	for first < m.active && total > w {
		total -= runewidth.StringWidth(labels[first])
		first++
	}
	var sb strings.Builder
	used := 0
	for i := first; i < len(labels) && used < w; i++ {
		l := runewidth.Truncate(labels[i], w-used, "")
		st := lipgloss.NewStyle().Background(th.TabBg).Foreground(th.Dim)
		if i == m.active {
			st = lipgloss.NewStyle().Background(th.TabActiveBg).Foreground(th.TabActiveFg).Bold(true)
			if m.tabs[i].Modified() {
				st = st.Foreground(th.Accent)
			}
		}
		sb.WriteString(st.Render(l))
		used += runewidth.StringWidth(l)
	}
	if used < w {
		sb.WriteString(lipgloss.NewStyle().Background(th.TabBg).Render(strings.Repeat(" ", w-used)))
	}
	return sb.String()
}

func (m *Model) statusBar() string {
	th := m.th
	base := lipgloss.NewStyle().Background(th.StatusBg).Foreground(th.StatusFg)
	if m.prompt != promptNone {
		var q string
		switch m.prompt {
		case promptClose:
			name := ""
			if m.promptTab < len(m.tabs) {
				name = m.tabs[m.promptTab].Name()
			}
			q = fmt.Sprintf(" « %s » a des modifications. Enregistrer avant de fermer ? (o)ui / (n)on / Échap pour annuler", name)
		case promptQuit:
			q = fmt.Sprintf(" %d fichier(s) modifié(s). (o) enregistrer et quitter / (n) quitter sans enregistrer / Échap pour annuler", len(m.modifiedTabs()))
		}
		return base.Foreground(th.Error).Bold(true).Render(fit(q, m.width))
	}

	var right string
	left := " etui"
	if ed := m.ActiveEditor(); ed != nil {
		c := ed.Cursor()
		left = " " + m.displayPath(ed)
		if ed.Modified() {
			left += " ●"
		}
		right = fmt.Sprintf("%s │ Ln %d, Col %d │ ", ed.Language(), c.Line+1, c.Col+1)
	}
	themeName := m.mode.String()
	if m.mode == theme.Auto {
		themeName += "/" + map[bool]string{true: "sombre", false: "clair"}[m.th.Dark]
	}
	right += "Thème " + themeName + " │ F1 aide "

	rw := runewidth.StringWidth(right)
	msg := ""
	if m.status != "" {
		msg = "  " + m.status
	}
	avail := m.width - rw
	if avail < 0 {
		return base.Render(fit(left, m.width))
	}
	if avail < 6 {
		return base.Render(fit(left, m.width))
	}
	if runewidth.StringWidth(left) > avail {
		left = runewidth.Truncate(left, avail, "…")
		msg = ""
	}
	lw := runewidth.StringWidth(left)
	if lw+runewidth.StringWidth(msg) > avail {
		msg = runewidth.Truncate(msg, max(0, avail-lw), "…")
	}
	l := base.Render(fit(left, lw))
	var ms string
	if msg != "" {
		st := base.Bold(true)
		if m.statusErr {
			st = st.Foreground(th.Error)
		} else {
			st = st.Foreground(th.Accent)
		}
		ms = st.Render(msg)
	}
	pad := avail - lw - runewidth.StringWidth(msg)
	return l + ms + base.Render(strings.Repeat(" ", max(0, pad))+right)
}

func (m *Model) displayPath(ed *editor.Editor) string {
	if ed.Path == "" {
		return ed.Name()
	}
	if rel, err := filepath.Rel(m.explorer.Root(), ed.Path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return ed.Path
}

var helpLines = []string{
	"Raccourcis",
	"",
	"  Fichiers   Ctrl+S enregistrer · Ctrl+W fermer l'onglet · Ctrl+Q quitter",
	"  Onglets    Alt+←/→ ou Ctrl+PgUp/PgDn · Alt+1…9",
	"  Panneaux   Ctrl+B focus explorateur/éditeur · Ctrl+E afficher/masquer · Échap → éditeur",
	"  Explorateur  ↑↓ naviguer · Entrée ouvrir/déplier · ← replier · . fichiers cachés · F5 recharger",
	"  Édition    Ctrl+Z annuler · Ctrl+Y rétablir · Tab insère une tabulation",
	"  Sélection  Maj+flèches · Ctrl+Maj+←/→ par mot · Ctrl+A tout · Ctrl+L ligne · Ctrl+D mot",
	"  Presse-papiers  Ctrl+C copier · Ctrl+X couper · Ctrl+V coller (repli OSC52 en SSH)",
	"  Recherche  Ctrl+F chercher · Ctrl+R remplacer · Entrée/F3 suivant · Maj+Entrée précédent",
	"             Alt+C casse · Tab changer de champ · Alt+A tout remplacer · Échap fermer",
	"  Thème      Ctrl+T auto → clair → sombre",
	"",
	"Appuyez sur une touche pour fermer.",
}

func (m *Model) emptyArea(w, h int) []string {
	lines := make([]string, h)
	var content []string
	if m.help {
		content = helpLines
	} else {
		content = []string{"etui", "", "Ouvrez un fichier depuis l'explorateur (Entrée).", "F1 : aide"}
	}
	dim := lipgloss.NewStyle().Foreground(m.th.Dim)
	title := lipgloss.NewStyle().Foreground(m.th.Accent).Bold(true)
	for i := range lines {
		lines[i] = strings.Repeat(" ", w)
		if i < len(content) {
			st := dim
			if i == 0 {
				st = title
			}
			lines[i] = st.Render(fit(" "+content[i], w))
		}
	}
	return lines
}

// View assemble l'écran.
func (m *Model) View() tea.View {
	if m.width < 10 || m.height < 4 {
		return tea.NewView("")
	}
	th := m.th
	ew := m.explorerWidth()
	cw := m.editorWidth()
	bodyH := m.height - 2

	var edLines []string
	var cur editor.CursorPos
	if ed := m.ActiveEditor(); ed != nil && !m.help {
		out, c := ed.View(th, m.focus == focusEditor && m.prompt == promptNone)
		edLines = strings.Split(out, "\n")
		cur = c
	} else {
		edLines = m.emptyArea(cw, bodyH)
	}
	col := append([]string{m.tabBar(cw)}, edLines...)

	var rows []string
	var exLines []string
	if ew > 0 {
		exLines = strings.Split(m.explorer.View(th, m.focus == focusExplorer), "\n")
	}
	border := lipgloss.NewStyle().Foreground(th.Border).Render("│")
	for i := 0; i < m.height-1; i++ {
		var line string
		if ew > 0 {
			if i < len(exLines) {
				line = exLines[i]
			} else {
				line = strings.Repeat(" ", ew)
			}
			line += border
		}
		if i < len(col) {
			line += col[i]
		} else {
			line += strings.Repeat(" ", cw)
		}
		rows = append(rows, line)
	}
	rows = append(rows, m.statusBar())

	v := tea.NewView(strings.Join(rows, "\n"))
	v.AltScreen = true
	v.WindowTitle = "etui"
	if ed := m.ActiveEditor(); ed != nil {
		v.WindowTitle = "etui — " + ed.Name()
	}
	if cur.Visible && m.focus == focusEditor && m.prompt == promptNone && !m.help {
		x := cur.X
		if ew > 0 {
			x += ew + 1
		}
		v.Cursor = tea.NewCursor(x, cur.Y+1)
	}
	if m.mode != theme.Auto {
		v.BackgroundColor = th.Bg
		v.ForegroundColor = th.Fg
	}
	return v
}
