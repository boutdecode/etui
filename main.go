// Commande etui : éditeur de texte TUI avec explorateur de fichiers et coloration syntaxique.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"boutdecode/etui/internal/app"
	"boutdecode/etui/internal/theme"
)

// version est injectée au build via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "affiche la version et quitte")
	themeFlag := flag.String("theme", "auto", "thème : auto, light ou dark")
	transparent := flag.Bool("transparent", false, "garde le fond du terminal au lieu du fond du thème")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage : etui [--theme=auto|light|dark] [--transparent] [--version] [dossier | fichier...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("etui", version)
		return
	}

	mode, err := theme.ParseMode(*themeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "etui :", err)
		os.Exit(2)
	}

	opts := app.Options{Dir: ".", Mode: mode, Transparent: *transparent}
	for i, a := range flag.Args() {
		st, err := os.Stat(a)
		switch {
		case err == nil && st.IsDir():
			if i == 0 {
				opts.Dir = a
			}
		default: // fichier existant ou à créer
			if len(opts.Files) == 0 && i == 0 {
				opts.Dir = filepath.Dir(a)
			}
			opts.Files = append(opts.Files, a)
		}
	}

	if _, err := tea.NewProgram(app.New(opts)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "etui :", err)
		os.Exit(1)
	}
}
