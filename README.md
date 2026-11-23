# etui

Terminal text editor (Bubble Tea v2, Lip Gloss v2, Chroma) with a file explorer, tabs, selection,
clipboard, search/replace and syntax highlighting.

## Usage

```sh
make build            # builds bin/etui (Go ≥ 1.26 required by Bubble Tea v2; GO=... to change the SDK)
bin/etui [--theme=auto|light|dark] [--lang=auto|en|fr] [--transparent] [folder | file...]
make test
```

## Quick install

```sh
curl -fsSL https://github.com/boutdecode/etui/releases/latest/download/install.sh | bash
```

The binary is installed in `~/.local/bin` (no `sudo`); the SHA256 checksum is verified before installing.
Options (after `bash -s --`) and environment variables:

```sh
# a specific version
curl -fsSL https://github.com/boutdecode/etui/releases/latest/download/install.sh | bash -s -- --version v1.2.3
# another directory (or ETUI_INSTALL_DIR=...)
curl -fsSL https://github.com/boutdecode/etui/releases/latest/download/install.sh | bash -s -- --dir /opt/bin
# uninstall
curl -fsSL https://github.com/boutdecode/etui/releases/latest/download/install.sh | bash -s -- --uninstall
```

Prefer to read the script before running it? Download [`install.sh`](install.sh), inspect it, then run `bash install.sh`.
`ETUI_VERSION` is also honoured; options take precedence over environment variables.

## Manual installation

Download the archive for your system (linux/darwin, amd64/arm64) from the [Releases](../../releases) page, then:

```sh
tar xzf etui_<version>_<os>_<arch>.tar.gz
sudo mv etui /usr/local/bin/
etui --version
```

## Publishing a release

```sh
git tag v1.2.3
git push origin v1.2.3
```

The `Release` workflow runs the tests, then GoReleaser, which publishes the binaries, `checksums.txt` and the changelog.

## Theme

The light/dark theme is detected from the terminal background (fallback: dark). `Ctrl+T` cycles auto → light → dark.
The theme background is applied to the terminal; `--transparent` keeps the terminal background.

## Language

The interface is available in English (default) and French. The language is chosen, in order of precedence, from:

1. the `--lang=auto|en|fr` flag;
2. the `ETUI_LANG` environment variable;
3. the standard locale variables `LC_ALL`, `LC_MESSAGES`, `LANGUAGE` and `LANG`;
4. English.

The first variable that is set decides, as with POSIX locales (for example `LC_ALL=C` gives English even if `LANG=fr_FR.UTF-8`).
Values such as `fr`, `fr_FR` or `fr_FR.UTF-8` select French; any other locale selects English. An unknown `ETUI_LANG` is ignored,
an unknown `--lang` is an error, and `--lang=auto` (or an empty value) goes back to detection from the system.

```sh
etui --lang=fr
ETUI_LANG=fr etui
LANG=fr_FR.UTF-8 etui
```

## Shortcuts (F1 in the application)

| Action | Keys |
|---|---|
| Save / close tab / quit | `Ctrl+S` / `Ctrl+W` / `Ctrl+Q` |
| Switch tab | `Alt+←/→`, `Ctrl+PgUp/PgDn`, `Alt+1…9` |
| Focus explorer ↔ editor, show/hide | `Ctrl+B`, `Ctrl+E` |
| Explorer | `↑↓`, `Enter`, `←`, `.` (hidden files), `F5`, `/` (search by name; `Esc` leaves) |
| Undo / redo | `Ctrl+Z` / `Ctrl+Y` |
| Selection | `Shift+arrows`, `Ctrl+Shift+←/→`, `Ctrl+A` (all), `Ctrl+L` (line), `Ctrl+D` (word) |
| Copy / cut / paste | `Ctrl+C` / `Ctrl+X` / `Ctrl+V` (system clipboard, OSC52 fallback over SSH) |
| Find / replace | `Ctrl+F` / `Ctrl+R`; `Enter`, `Shift+Enter`, `Alt+C` (case), `Tab`, `Alt+A` (all) |

## Structure

- `internal/editor`: rune buffer, selection, history, search, rendering (`view.go` defines the overlay order: syntax < current line < selection < search).
- `internal/explorer`: lazily loaded tree (`search.go`: asynchronous search by name).
- `internal/highlight`: Chroma → one token type per rune, deferred recomputation.
- `internal/theme`: light/dark themes and the matching Chroma styles.
- `internal/i18n`: translated messages (English, French) and language detection.
- `internal/app`: root model (layout, focus, tabs, clipboard, confirmations).
- `install.sh`: `curl | bash` installer, published as a release asset (`install_test.sh`: its tests; `make lint-sh test-sh`).

## Known limitations

No mouse support, no automatic line wrapping, search limited to a single line,
no "save as".
