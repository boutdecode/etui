package explorer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"boutdecode/etui/internal/theme"
)

func setup(t *testing.T) (string, *Model) {
	d := t.TempDir()
	for _, f := range []string{"b.txt", "A.go", ".hidden", "zdir/inner.txt", "zdir/sub/deep.go", "adir/x.md"} {
		p := filepath.Join(d, f)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	m := New(d)
	m.SetSize(30, 10)
	return d, m
}

func press(m *Model, ks string) tea.Cmd {
	k := tea.Key{Code: []rune(ks)[0], Text: ks}
	switch ks {
	case "enter":
		k = tea.Key{Code: tea.KeyEnter}
	case "right":
		k = tea.Key{Code: tea.KeyRight}
	case "left":
		k = tea.Key{Code: tea.KeyLeft}
	case "down":
		k = tea.Key{Code: tea.KeyDown}
	case "up":
		k = tea.Key{Code: tea.KeyUp}
	}
	return m.Update(tea.KeyPressMsg(k))
}

func names(m *Model) string {
	var out []string
	for _, p := range m.Visible() {
		r, _ := filepath.Rel(m.Root(), p)
		out = append(out, r)
	}
	return strings.Join(out, ",")
}

func TestOrderingAndHidden(t *testing.T) {
	_, m := setup(t)
	if got := names(m); got != "adir,zdir,A.go,b.txt" {
		t.Fatalf("ordre : %s", got)
	}
	press(m, ".")
	if got := names(m); !strings.Contains(got, ".hidden") {
		t.Fatalf("hidden files missing: %s", got)
	}
}

func TestLazyExpandCollapseAndOpen(t *testing.T) {
	d, m := setup(t)
	if m.root.children[1].loaded {
		t.Fatal("zdir must not be loaded before it is opened")
	}
	press(m, "down") // zdir
	press(m, "enter")
	if got := names(m); got != "adir,zdir,zdir/sub,zdir/inner.txt,A.go,b.txt" {
		t.Fatalf("after expansion: %s", got)
	}
	press(m, "left")
	if got := names(m); got != "adir,zdir,A.go,b.txt" {
		t.Fatalf("after collapse: %s", got)
	}
	press(m, "right") // reopens
	press(m, "down")
	press(m, "down") // inner.txt
	cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("commande d'ouverture attendue")
	}
	if msg, ok := cmd().(OpenFileMsg); !ok || msg.Path != filepath.Join(d, "zdir", "inner.txt") {
		t.Fatalf("message = %#v", cmd())
	}
	// left on a file goes up to the parent directory
	press(m, "left")
	if m.Selected() != filepath.Join(d, "zdir") {
		t.Fatalf("selection = %s", m.Selected())
	}
}

func TestReloadKeepsState(t *testing.T) {
	d, m := setup(t)
	press(m, "down")
	press(m, "enter")
	os.WriteFile(filepath.Join(d, "zdir", "new.txt"), nil, 0o644)
	m.Reload()
	if got := names(m); !strings.Contains(got, "zdir/new.txt") || !strings.Contains(got, "zdir/inner.txt") {
		t.Fatalf("rechargement : %s", got)
	}
}

func TestViewSize(t *testing.T) {
	_, m := setup(t)
	out := m.View(theme.Resolve(theme.Auto, true), true)
	lines := strings.Split(out, "\n")
	if len(lines) != 10 {
		t.Fatalf("lignes = %d", len(lines))
	}
	for i, l := range lines {
		if lipgloss.Width(l) != 30 {
			t.Errorf("ligne %d : %d", i, lipgloss.Width(l))
		}
	}
}
