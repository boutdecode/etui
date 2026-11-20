package i18n

import (
	"os"
	"reflect"
	"regexp"
	"sort"
	"testing"
)

func TestMain(m *testing.M) {
	Set(EN)
	os.Exit(m.Run())
}

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestParse(t *testing.T) {
	ok := map[string]Lang{
		"": Auto, "auto": Auto, " AUTO ": Auto,
		"en": EN, "EN": EN, "en_US.UTF-8": EN, "en-GB": EN,
		"fr": FR, "Fr": FR, "fr_FR": FR, "fr_FR.UTF-8": FR, "fr_CA@euro": FR, "fr-CA": FR,
	}
	for in, want := range ok {
		if got, err := Parse(in); err != nil || got != want {
			t.Errorf("Parse(%q) = %v, %v ; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"xx", "de_DE.UTF-8", "C", "POSIX", "french"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Lang
	}{
		{"empty", nil, EN},
		{"LANG fr", map[string]string{"LANG": "fr_FR.UTF-8"}, FR},
		{"LANG C", map[string]string{"LANG": "C"}, EN},
		{"LANG other", map[string]string{"LANG": "de_DE.UTF-8"}, EN},
		{"ETUI_LANG over LANG", map[string]string{"ETUI_LANG": "en", "LANG": "fr_FR.UTF-8"}, EN},
		{"ETUI_LANG fr over LANG en", map[string]string{"ETUI_LANG": "fr", "LANG": "en_US.UTF-8"}, FR},
		{"invalid ETUI_LANG ignored", map[string]string{"ETUI_LANG": "xx", "LANG": "fr_FR.UTF-8"}, FR},
		{"LC_ALL over LANG", map[string]string{"LC_ALL": "C", "LANG": "fr_FR.UTF-8"}, EN},
		{"LC_ALL over LC_MESSAGES", map[string]string{"LC_ALL": "fr_FR", "LC_MESSAGES": "en_US"}, FR},
		{"LC_MESSAGES over LANGUAGE", map[string]string{"LC_MESSAGES": "en_US", "LANGUAGE": "fr"}, EN},
		{"LANGUAGE over LANG", map[string]string{"LANGUAGE": "fr", "LANG": "en_US"}, FR},
		{"LANGUAGE list", map[string]string{"LANGUAGE": "de:fr:en"}, FR},
		{"auto in ETUI_LANG falls through", map[string]string{"ETUI_LANG": "auto", "LANG": "fr_FR"}, FR},
	}
	for _, c := range cases {
		if got := Detect(env(c.env)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSetAndT(t *testing.T) {
	defer Set(EN)
	Set(FR)
	if Current() != FR || T(Saved, "x") != "Enregistré : x" {
		t.Fatalf("French not active: %v %q", Current(), T(Saved, "x"))
	}
	Set(EN)
	if T(Saved, "x") != "Saved: x" {
		t.Fatalf("English not active: %q", T(Saved, "x"))
	}
	if T("no.such.key") != "no.such.key" {
		t.Error("an unknown key should return itself")
	}
}

// Both catalogues must define the same keys with the same format verbs.
func TestCatalogsConsistent(t *testing.T) {
	verbs := regexp.MustCompile(`%[#+-]?[a-zA-Z]`)
	var missing []string
	for k, e := range en {
		f, ok := fr[k]
		if !ok {
			missing = append(missing, "fr:"+string(k))
			continue
		}
		if !reflect.DeepEqual(verbs.FindAllString(e, -1), verbs.FindAllString(f, -1)) {
			t.Errorf("%s: format verbs differ: %q vs %q", k, e, f)
		}
	}
	for k := range fr {
		if _, ok := en[k]; !ok {
			missing = append(missing, "en:"+string(k))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("keys missing from a catalogue: %v", missing)
	}
}
