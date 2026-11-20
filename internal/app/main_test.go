package app

import (
	"os"
	"testing"

	"boutdecode/etui/internal/i18n"
)

// TestMain pins the language so assertions on messages do not depend on the
// environment of the machine running the tests.
func TestMain(m *testing.M) {
	i18n.Set(i18n.EN)
	os.Exit(m.Run())
}
