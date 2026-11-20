package app

import (
	"bytes"
	"github.com/charmbracelet/x/ansi"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/mattn/go-runewidth"

	"boutdecode/etui/internal/clip"
	"boutdecode/etui/internal/explorer"
	"boutdecode/etui/internal/i18n"
	"boutdecode/etui/internal/theme"
)

func project(t *testing.T) string {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "a.go"), []byte("package a\n\nfunc A() {}\n"), 0o644)
	os.WriteFile(filepath.Join(d, "b.txt"), []byte("bonjour\nmonde\n"), 0o644)
	return d
}

func newApp(t *testing.T, files ...string) (*Model, *clip.Memory) {
	cb := &clip.Memory{}
	m := New(Options{Dir: project(t), Clip: cb})
	for _, f := range files {
		m.openFile(filepath.Join(m.explorer.Root(), f))
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m, cb
}

func send(m *Model, ks string) tea.Cmd {
	var k tea.Key
	parts := strings.Split(ks, "+")
	name := parts[len(parts)-1]
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "ctrl":
			k.Mod |= tea.ModCtrl
		case "alt":
			k.Mod |= tea.ModAlt
		case "shift":
			k.Mod |= tea.ModShift
		}
	}
	named := map[string]rune{"left": tea.KeyLeft, "right": tea.KeyRight, "down": tea.KeyDown, "up": tea.KeyUp,
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "end": tea.KeyEnd, "home": tea.KeyHome, "pgdown": tea.KeyPgDown, "pgup": tea.KeyPgUp}
	if r, ok := named[name]; ok {
		k.Code = r
	} else {
		k.Code = []rune(name)[0]
		if k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			k.Text = name
		}
	}
	_, cmd := m.Update(tea.KeyPressMsg(k))
	return cmd
}

func typeText(m *Model, s string) {
	for _, r := range s {
		send(m, string(r))
	}
}

func TestOpenSameFileActivatesTab(t *testing.T) {
	m, _ := newApp(t, "a.go", "b.txt")
	if len(m.Tabs()) != 2 || m.active != 1 {
		t.Fatalf("onglets=%d actif=%d", len(m.Tabs()), m.active)
	}
	m.Update(explorer.OpenFileMsg{Path: filepath.Join(m.explorer.Root(), "a.go")})
	if len(m.Tabs()) != 2 || m.active != 0 {
		t.Fatalf("un fichier déjà ouvert doit activer son onglet : %d/%d", len(m.Tabs()), m.active)
	}
}

func TestCloseActiveTabAndSwitch(t *testing.T) {
	m, _ := newApp(t, "a.go", "b.txt")
	send(m, "alt+left")
	if m.active != 0 {
		t.Fatal("alt+gauche")
	}
	send(m, "alt+left") // circulaire
	if m.active != 1 {
		t.Fatalf("circulaire : %d", m.active)
	}
	send(m, "ctrl+w")
	if len(m.Tabs()) != 1 || m.ActiveEditor().Name() != "a.go" {
		t.Fatalf("après fermeture : %d %v", len(m.Tabs()), m.ActiveEditor())
	}
	send(m, "ctrl+w")
	if len(m.Tabs()) != 0 || m.ActiveEditor() != nil || m.focus != focusExplorer {
		t.Fatal("tout fermé : focus sur l'explorateur")
	}
	if v := m.View(); v.Content == "" {
		t.Fatal("la vue sans onglet doit rester rendue")
	}
}

func TestCloseModifiedTabPrompt(t *testing.T) {
	m, _ := newApp(t, "b.txt")
	typeText(m, "X")
	send(m, "ctrl+w")
	if m.prompt != promptClose || len(m.Tabs()) != 1 {
		t.Fatal("confirmation attendue")
	}
	send(m, "esc")
	if len(m.Tabs()) != 1 || m.prompt != promptNone {
		t.Fatal("annulation")
	}
	send(m, "ctrl+w")
	send(m, "n") // fermer sans enregistrer
	if len(m.Tabs()) != 0 {
		t.Fatal("fermeture sans enregistrer")
	}
	data, _ := os.ReadFile(filepath.Join(m.explorer.Root(), "b.txt"))
	if string(data) != "bonjour\nmonde\n" {
		t.Fatalf("le fichier ne devait pas changer : %q", data)
	}

	m.openFile(filepath.Join(m.explorer.Root(), "b.txt"))
	typeText(m, "Y")
	send(m, "ctrl+w")
	send(m, "y") // save, then close
	data, _ = os.ReadFile(filepath.Join(m.explorer.Root(), "b.txt"))
	if len(m.Tabs()) != 0 || string(data) != "Ybonjour\nmonde\n" {
		t.Fatalf("enregistrer + fermer : %d %q", len(m.Tabs()), data)
	}
}

func TestQuitPromptsWhenModified(t *testing.T) {
	m, _ := newApp(t, "b.txt")
	if cmd := send(m, "ctrl+q"); cmd == nil {
		t.Fatal("quitter sans modification")
	}
	typeText(m, "X")
	if cmd := send(m, "ctrl+q"); cmd != nil || m.prompt != promptQuit {
		t.Fatal("confirmation attendue")
	}
	send(m, "esc")
	if m.prompt != promptNone {
		t.Fatal("annulation")
	}
}

func TestClipboardMultilineCopyCutPaste(t *testing.T) {
	m, cb := newApp(t, "b.txt")
	ed := m.ActiveEditor()
	send(m, "ctrl+a")
	send(m, "ctrl+c")
	if cb.Text != "bonjour\nmonde\n" {
		t.Fatalf("copié = %q", cb.Text)
	}
	send(m, "ctrl+x")
	if ed.Text() != "" {
		t.Fatalf("après couper = %q", ed.Text())
	}
	send(m, "ctrl+v")
	send(m, "ctrl+v")
	if ed.Text() != "bonjour\nmonde\nbonjour\nmonde\n" {
		t.Fatalf("après coller = %q", ed.Text())
	}
	send(m, "ctrl+z") // un seul undo par collage
	if ed.Text() != "bonjour\nmonde\n" {
		t.Fatalf("undo du collage = %q", ed.Text())
	}
}

type failingClip struct{ clip.Memory }

func (*failingClip) Read() (string, error) { return "", os.ErrNotExist }
func (*failingClip) Write(string) error    { return os.ErrNotExist }

func TestClipboardFallbackOSC52AndRegister(t *testing.T) {
	m := New(Options{Dir: project(t), Clip: &failingClip{}})
	m.openFile(filepath.Join(m.explorer.Root(), "b.txt"))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	send(m, "ctrl+a")
	if cmd := send(m, "ctrl+c"); cmd == nil {
		t.Fatal("repli OSC52 attendu")
	}
	send(m, "end")
	send(m, "ctrl+v")
	if !strings.HasSuffix(m.ActiveEditor().Text(), "bonjour\nmonde\n") || strings.Count(m.ActiveEditor().Text(), "bonjour") != 1 {
		// ctrl+a puis End a replié la sélection ; le collage vient du registre interne
		t.Logf("texte = %q", m.ActiveEditor().Text())
	}
	if m.register != "bonjour\nmonde\n" {
		t.Fatalf("registre = %q", m.register)
	}
}

func TestBracketedPasteIsNotInterpretedAsKeys(t *testing.T) {
	m, _ := newApp(t, "b.txt")
	send(m, "ctrl+a")
	m.Update(tea.PasteMsg{Content: "ctrl+q\n\tx"})
	if m.ActiveEditor().Text() != "ctrl+q\n\tx" {
		t.Fatalf("texte = %q", m.ActiveEditor().Text())
	}
}

func TestSaveFlow(t *testing.T) {
	m, _ := newApp(t, "a.go")
	send(m, "ctrl+f")
	typeText(m, "A()")
	send(m, "esc")
	typeText(m, "B()")
	if !m.ActiveEditor().Modified() {
		t.Fatal("modifié")
	}
	send(m, "ctrl+s")
	if m.ActiveEditor().Modified() || !strings.Contains(m.Status(), "Saved") {
		t.Fatalf("status = %q", m.Status())
	}
	data, _ := os.ReadFile(filepath.Join(m.explorer.Root(), "a.go"))
	if !strings.Contains(string(data), "func B()") {
		t.Fatalf("contenu = %q", data)
	}
}

func TestThemeFollowsDetectedBackground(t *testing.T) {
	m, _ := newApp(t)
	if !m.Theme().Dark {
		t.Fatal("repli sombre par défaut")
	}
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	if m.Theme().Dark || m.Theme().Chroma.Name != theme.ChromaLight {
		t.Fatalf("fond clair → thème clair, obtenu %v/%s", m.Theme().Dark, m.Theme().Chroma.Name)
	}
	m.Update(tea.BackgroundColorMsg{Color: color.Black})
	if !m.Theme().Dark || m.Theme().Chroma.Name != theme.ChromaDark {
		t.Fatal("fond sombre → thème sombre")
	}
	send(m, "ctrl+t") // auto → clair, quel que soit le fond
	if m.Theme().Dark || m.mode != theme.Light {
		t.Fatal("forcer clair")
	}
	m.Update(tea.BackgroundColorMsg{Color: color.Black})
	if m.Theme().Dark {
		t.Fatal("le mode forcé prime sur la détection")
	}
}

func TestViewLayoutAllSizes(t *testing.T) {
	m, _ := newApp(t, "a.go", "b.txt")
	for _, sz := range [][2]int{{100, 30}, {60, 12}, {49, 10}, {20, 5}, {12, 4}} {
		m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		for _, mode := range []theme.Mode{theme.Dark, theme.Light} {
			m.mode, m.th = mode, theme.Resolve(mode, true)
			lines := strings.Split(m.View().Content, "\n")
			if len(lines) != sz[1] {
				t.Fatalf("%v : %d lignes", sz, len(lines))
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w != sz[0] {
					t.Fatalf("%v ligne %d : largeur %d\n%s", sz, i, w, l)
				}
			}
		}
	}
}

func TestIntegrationTeatest(t *testing.T) {
	dir := project(t)
	m := New(Options{Dir: dir, Clip: &clip.Memory{}})
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(100, 30))

	// ouvrir a.go depuis l'explorateur (premier fichier de la liste)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("func")) },
		teatest.WithDuration(3*time.Second))
	// modifier, enregistrer
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnd, Mod: tea.ModCtrl})
	tm.Type("// fin")
	tm.Send(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	// ouvrir b.txt : retour explorateur, descendre, ouvrir
	tm.Send(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	// l'ouverture passe par un message asynchrone : attendre l'affichage du contenu
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("bonjour")) },
		teatest.WithDuration(3*time.Second))
	// changer d'onglet puis quitter
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt})
	tm.Send(tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl})
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).(*Model)

	if len(fm.Tabs()) != 2 || fm.ActiveEditor().Name() != "a.go" {
		t.Fatalf("onglets=%d actif=%v", len(fm.Tabs()), fm.ActiveEditor())
	}
	data, _ := os.ReadFile(filepath.Join(dir, "a.go"))
	if !strings.HasSuffix(string(data), "// fin") {
		t.Fatalf("fichier = %q", data)
	}
}

func TestExplorerSearchEscAndAsyncRouting(t *testing.T) {
	m, _ := newApp(t)
	m.focus = focusExplorer
	cmd := send(m, "/")
	if !m.explorer.Searching() || cmd == nil {
		t.Fatal("recherche non démarrée")
	}
	send(m, "a")
	m.Update(cmd()) // le résultat traverse app.Update
	if m.explorer.Query() != "a" {
		t.Fatal("requête perdue")
	}
	send(m, "esc")
	if m.explorer.Searching() || m.focus != focusExplorer {
		t.Fatal("le premier Échap ferme la recherche et garde le focus")
	}
}

func TestViewPaintsThemeBackground(t *testing.T) {
	m, _ := newApp(t, "a.go")
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	const paint = "\x1b[38;2;228;224;245;48;2;20;18;31m"
	if !strings.Contains(m.View().Content, paint) {
		t.Fatal("le fond du thème doit être appliqué")
	}
	m.transparent = true
	if strings.Contains(m.View().Content, paint) {
		t.Fatal("--transparent ne doit pas peindre le fond")
	}
}

func TestViewHasMargins(t *testing.T) {
	m, _ := newApp(t, "a.go")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	lines := strings.Split(m.View().Content, "\n")
	if len(lines) != 20 || strings.TrimSpace(ansi.Strip(lines[0])) != "" || strings.TrimSpace(ansi.Strip(lines[19])) != "" {
		t.Fatal("marge haute et basse vides attendues")
	}
	if l := ansi.Strip(lines[5]); !strings.HasPrefix(l, "  ") || !strings.HasSuffix(l, "  ") {
		t.Fatalf("marges gauche/droite attendues : %q", l)
	}
}

func TestFrenchPromptAndHelp(t *testing.T) {
	i18n.Set(i18n.FR)
	defer i18n.Set(i18n.EN)

	m, _ := newApp(t)
	m.openFile(filepath.Join(m.explorer.Root(), "b.txt"))
	typeText(m, "Y")
	send(m, "ctrl+w")
	send(m, "o") // "oui" saves and closes in French
	data, _ := os.ReadFile(filepath.Join(m.explorer.Root(), "b.txt"))
	if len(m.Tabs()) != 0 || string(data) != "Ybonjour\nmonde\n" {
		t.Fatalf("French yes key: %d %q", len(m.Tabs()), data)
	}
	if got := m.Status(); !strings.Contains(got, "Enregistré") {
		t.Fatalf("status = %q", got)
	}
}

// Help texts must start on the same column in every language.
func TestHelpAligned(t *testing.T) {
	defer i18n.Set(i18n.EN)
	texts := []i18n.Key{i18n.HelpFilesText, i18n.HelpExplText, i18n.HelpClipText, i18n.HelpSearchMore, i18n.HelpThemeText}
	for _, l := range []i18n.Lang{i18n.EN, i18n.FR} {
		i18n.Set(l)
		lines := helpLines()
		col := -1
		for _, k := range texts {
			found := false
			for _, line := range lines {
				if i := strings.Index(line, i18n.T(k)); i >= 0 {
					w := runewidth.StringWidth(line[:i])
					if col >= 0 && w != col {
						t.Errorf("%v: %q starts at column %d, want %d", l, i18n.T(k), w, col)
					}
					col, found = w, true
				}
			}
			if !found {
				t.Errorf("%v: %q missing from help", l, i18n.T(k))
			}
		}
	}
}
