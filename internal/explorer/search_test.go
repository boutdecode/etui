package explorer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"boutdecode/etui/internal/theme"
)

func key(m *Model, k tea.Key) tea.Cmd { return m.Update(tea.KeyPressMsg(k)) }

func typeQ(m *Model, s string) {
	for _, r := range s {
		key(m, tea.Key{Code: r, Text: string(r)})
	}
}

// startAndScan ouvre la recherche et injecte le résultat du parcours.
func startAndScan(t *testing.T, m *Model) {
	t.Helper()
	cmd := press(m, "/")
	if cmd == nil || !m.Searching() {
		t.Fatal("recherche non démarrée")
	}
	m.Update(cmd())
}

func fill(t *testing.T, files ...string) string {
	d := t.TempDir()
	for _, f := range files {
		p := filepath.Join(d, f)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	return d
}

func TestWalkIgnoresAndLimit(t *testing.T) {
	d := fill(t, "a.go", ".h", ".git/x", "node_modules/y", "s/b.go", "s/.hid")
	es, tr := walk(context.Background(), d, false, 100)
	var got []string
	for _, e := range es {
		r, _ := filepath.Rel(d, e.path)
		got = append(got, r)
	}
	if s := strings.Join(got, ","); s != "a.go,s,s/b.go" || tr {
		t.Fatalf("walk = %s (tr=%v)", s, tr)
	}
	es, _ = walk(context.Background(), d, true, 100)
	if len(es) != 5 { // a.go .h s s/b.go s/.hid
		t.Fatalf("avec cachés : %d", len(es))
	}
	if es, tr := walk(context.Background(), d, true, 2); len(es) != 2 || !tr {
		t.Fatalf("plafond : %d %v", len(es), tr)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if es, _ := walk(ctx, d, true, 100); len(es) != 0 {
		t.Fatal("contexte annulé : aucune entrée attendue")
	}
}

func TestFilterNameOnlyCaseInsensitive(t *testing.T) {
	es := []entry{{"/r/src", true}, {"/r/src/main.go", false}, {"/r/SRCx.txt", false}, {"/r/É.md", false}}
	got := filter(es, "src")
	if len(got) != 2 || got[0].path != "/r/src" || got[1].path != "/r/SRCx.txt" {
		t.Fatalf("filter = %v", got)
	}
	if len(filter(es, "é")) != 1 || len(filter(es, "")) != 0 {
		t.Fatal("unicode / requête vide")
	}
}

func TestBuildTree(t *testing.T) {
	rows := buildTree("/r", []entry{{"/r/z/sub/deep.go", false}, {"/r/a.go", false}, {"/r/z", true}})
	var got []string
	for _, n := range rows {
		got = append(got, strings.Repeat("-", n.depth)+n.name)
	}
	if s := strings.Join(got, ","); s != "-z,--sub,---deep.go,-a.go" {
		t.Fatalf("buildTree = %s", s)
	}
}

func TestSearchEmptyDirAndKeysGoToQuery(t *testing.T) {
	m := New(t.TempDir())
	m.SetSize(30, 10)
	startAndScan(t, m)
	typeQ(m, "jkgGhl.")
	if m.Query() != "jkgGhl." {
		t.Fatalf("query = %q", m.Query())
	}
	if m.ShowHidden() {
		t.Fatal(". ne doit pas basculer les cachés")
	}
	if cmd := key(m, tea.Key{Code: tea.KeyEnter}); cmd != nil {
		t.Fatal("Entrée sans résultat ne fait rien")
	}
	m.View(theme.Resolve(theme.Auto, true), true)
}

func TestSearchDeepAndRealTreeUntouched(t *testing.T) {
	_, m := setup(t)
	press(m, "down")
	before, cur := names(m), m.cursor
	startAndScan(t, m)
	typeQ(m, "deep")
	if got := names(m); got != "zdir,zdir/sub,zdir/sub/deep.go" {
		t.Fatalf("arbre filtré : %s", got)
	}
	if m.root.children[1].loaded || m.root.children[1].expanded {
		t.Fatal("arbre réel modifié")
	}
	key(m, tea.Key{Code: tea.KeyEscape})
	if m.Searching() || names(m) != before || m.cursor != cur {
		t.Fatalf("restauration : %s %d", names(m), m.cursor)
	}
	if m.root.children[1].loaded {
		t.Fatal("zdir ne doit pas avoir été chargé")
	}
}

func TestSearchStaleResultIgnored(t *testing.T) {
	_, m := setup(t)
	startAndScan(t, m)
	typeQ(m, "x")
	m.Update(SearchResultMsg{Gen: m.gen - 1, Entries: []entry{{filepath.Join(m.Root(), "x-stale"), false}}})
	if strings.Contains(names(m), "stale") {
		t.Fatal("résultat périmé pris en compte")
	}
}

func TestSearchEnterFileAndDir(t *testing.T) {
	d, m := setup(t)
	startAndScan(t, m)
	typeQ(m, "deep")
	key(m, tea.Key{Code: tea.KeyDown}) // zdir/sub
	key(m, tea.Key{Code: tea.KeyDown}) // deep.go
	cmd := key(m, tea.Key{Code: tea.KeyEnter})
	if msg, ok := cmd().(OpenFileMsg); !ok || msg.Path != filepath.Join(d, "zdir/sub/deep.go") || m.Searching() {
		t.Fatalf("fichier : %#v searching=%v", cmd(), m.Searching())
	}
	startAndScan(t, m)
	typeQ(m, "sub")
	key(m, tea.Key{Code: tea.KeyDown}) // zdir/sub
	if cmd := key(m, tea.Key{Code: tea.KeyEnter}); cmd != nil {
		t.Fatal("pas de commande pour un dossier")
	}
	if m.Searching() || m.Selected() != filepath.Join(d, "zdir/sub") || !m.root.children[1].expanded {
		t.Fatalf("révélation : %s %s", m.Selected(), names(m))
	}
}

func TestSearchBackspaceAndUnicode(t *testing.T) {
	d := fill(t, "été.txt", "a.go")
	m := New(d)
	m.SetSize(30, 10)
	startAndScan(t, m)
	typeQ(m, "éta")
	key(m, tea.Key{Code: tea.KeyBackspace})
	if m.Query() != "ét" || names(m) != "été.txt" {
		t.Fatalf("%q %s", m.Query(), names(m))
	}
}

func TestSearchViewSize(t *testing.T) {
	_, m := setup(t)
	startAndScan(t, m)
	for _, q := range []string{"", "deep", "zzzz", strings.Repeat("éé世", 40)} {
		m.query = q
		m.refilter(false)
		lines := strings.Split(m.View(theme.Resolve(theme.Auto, true), true), "\n")
		if len(lines) != 10 {
			t.Fatalf("%q : %d lignes", q, len(lines))
		}
		for i, l := range lines {
			if lipgloss.Width(l) != 30 {
				t.Errorf("%q ligne %d : %d", q, i, lipgloss.Width(l))
			}
		}
	}
}
