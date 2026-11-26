package i18n

// Key identifies a translatable message. The comment after a key gives the
// arguments its message takes, if any.
type Key string

const (
	// Theme names.
	ThemeAuto  Key = "theme.auto"
	ThemeLight Key = "theme.light"
	ThemeDark  Key = "theme.dark"

	// Command line.
	ErrUnknownTheme Key = "cli.unknownTheme" // %q value
	ErrUnknownLang  Key = "cli.unknownLang"  // %q value
	FlagVersion     Key = "cli.flag.version"
	FlagTheme       Key = "cli.flag.theme"
	FlagTransparent Key = "cli.flag.transparent"
	FlagLang        Key = "cli.flag.lang"
	Usage           Key = "cli.usage"

	// Editor.
	ErrBinaryFile Key = "editor.binaryFile" // %s file name
	ErrNoName     Key = "editor.noNameSave"
	NoName        Key = "editor.noName"

	// Search bar.
	SearchLabel    Key = "search.label"
	ReplaceLabel   Key = "search.replaceLabel"
	SearchKeys     Key = "search.keys"
	ReplaceKeys    Key = "search.replaceKeys"
	SearchNoMatch  Key = "search.noMatch"
	SearchReplaced Key = "search.replaced" // %d count

	// Explorer.
	ExplorerTruncated Key = "explorer.truncated"
	ExplorerSearching Key = "explorer.searching"
	ExplorerTypeHint  Key = "explorer.typeHint"
	ExplorerHiddenOn  Key = "explorer.hiddenOn"
	ExplorerNoResult  Key = "explorer.noResult"

	// Application.
	IsDirectory    Key = "app.isDirectory" // %s path
	SaveFailed     Key = "app.saveFailed"  // %s error
	Saved          Key = "app.saved"       // %s name
	NothingToCopy  Key = "app.nothingToCopy"
	PromptClose    Key = "app.promptClose" // %s name
	PromptQuit     Key = "app.promptQuit"  // %d count
	PromptYesKey   Key = "app.promptYesKey"
	ThemeNotice    Key = "app.themeNotice" // %s theme name
	StatusTheme    Key = "app.statusTheme" // %s theme name
	StatusHelp     Key = "app.statusHelp"
	EmptyHint      Key = "app.emptyHint"
	EmptyHelp      Key = "app.emptyHelp"
	HelpTitle      Key = "help.title"
	HelpClose      Key = "help.close"
	HelpFiles      Key = "help.files"
	HelpFilesText  Key = "help.files.text"
	HelpTabs       Key = "help.tabs"
	HelpTabsText   Key = "help.tabs.text"
	HelpPanes      Key = "help.panes"
	HelpPanesText  Key = "help.panes.text"
	HelpExplorer   Key = "help.explorer"
	HelpExplText   Key = "help.explorer.text"
	HelpEdit       Key = "help.edit"
	HelpEditText   Key = "help.edit.text"
	HelpSelect     Key = "help.select"
	HelpSelectText Key = "help.select.text"
	HelpClipboard  Key = "help.clipboard"
	HelpClipText   Key = "help.clipboard.text"
	HelpSearch     Key = "help.search"
	HelpSearchText Key = "help.search.text"
	HelpSearchMore Key = "help.search.more"
	HelpTheme      Key = "help.theme"
	HelpThemeText  Key = "help.theme.text"
)
