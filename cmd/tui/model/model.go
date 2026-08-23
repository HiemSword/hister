// SPDX-FileContributor: FlameFlag <github@flameflag.dev>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"fmt"
	"math/rand"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/asciimoo/hister/client"
	"github.com/asciimoo/hister/cmd/tui/component"
	"github.com/asciimoo/hister/cmd/tui/theme"
	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/indexer"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
	"github.com/muesli/termenv"
	"github.com/rs/zerolog/log"
)

type Model struct {
	// Core UI components
	TextInput textinput.Model
	Viewport  viewport.Model
	Spinner   spinner.Model
	Help      help.Model
	Keys      component.KeyMap
	Workspace viewport.Model
	State     ViewState
	PrevState ViewState
	Cfg       *config.Config
	Client    *client.Client
	Results   *indexer.Results

	// Dimensions and readiness
	Width, Height int
	Ready         bool

	// Viewport line tracking
	LineOffsets         []int
	TotalLines          int
	TabTargets          []HintRegion
	WorkspaceTargets    []WorkspaceTarget
	WorkspaceSelectionY int

	// WebSocket communication
	Conn    *websocket.Conn
	WsMu    sync.Mutex
	WsChan  chan tea.Msg
	WsDone  chan struct{}
	WsReady bool

	// Selection and search state
	SelectedIdx int
	Limit       int
	IsSearching bool
	SortMode    string // "" for relevance, "domain" for domain

	// Rendering
	Styles theme.Styles

	// Dialogs and overlays
	DialogMsg       string
	DialogConfirm   func() tea.Cmd
	DialogBtnIdx    int // 0=Cancel, 1=Delete
	DialogURL       string
	DialogReturnTab int // -1 = return to results/search, >=0 = stay on that tab

	// Connection state
	ConnError error
	HintFlash config.Action
	Notice    string
	NoticeID  uint64

	// Scrollbar interaction
	ScrollbarDragging bool

	// Mouse/overlay drag state
	OverlayOffX int
	OverlayOffY int
	IsDragging  bool
	DragStartX  int
	DragStartY  int
	DragOffX0   int
	DragOffY0   int

	// Terminal background, queried asynchronously by Bubble Tea.
	IsDarkBg bool
	BgSet    bool

	// Clickable suggestion
	SuggestionHeight int

	// Theme picker state
	ThemeName          string
	ThemePickerIdx     int
	OrigThemeName      string
	ThemePickerMode    string // "auto", "dark", "light"
	ThemePickerSection int    // 0=dark, 1=light
	DarkThemeIdx       int
	LightThemeIdx      int
	OrigDarkTheme      string
	OrigLightTheme     string
	OrigColorScheme    string
	ThemeDarkStart     int
	ThemeDarkCount     int
	ThemeLightStart    int
	ThemeLightCount    int

	// Context menu
	MenuX, MenuY int
	MenuIdx      int // result index the menu targets
	MenuSelIdx   int // selected menu option

	// Settings panel
	SettingsIdx      int
	SettingsEditMode bool
	SettingsEditErr  string

	// Tab bar
	ActiveTab int // 0=Search, 1=History, 2=Rules, 3=Add

	// History tab
	HistoryItems   []HistoryItem
	HistoryIdx     int
	HistoryLoading bool

	// Rules tab (form-based UI)
	RulesData           RulesResponse
	RulesIdx            int
	RulesSection        int // skip, priority, versioning, or aliases
	RulesLoading        bool
	RulesPatternInputs  [3]textinput.Model // skip, priority, versioning
	RulesAliasKeyInput  textinput.Model
	RulesAliasValInput  textinput.Model
	RulesFormFocus      int // pattern, alias key, alias value, or list
	RulesEditingIdx     int // -1 = adding new, >=0 = editing existing item
	RulesEditingSection int // section containing the item being edited

	// Add tab
	AddInputs   [2]textinput.Model // URL and title
	AddText     textarea.Model
	AddFocusIdx int
	AddStatus   string

	// Prioritize dialog
	PrioritizeURL    string
	PrioritizeInput  textinput.Model
	PrioritizeBtnIdx int // 0=Cancel, 1=Confirm

	// Tips rotation
	TipIdx int
}

func InitialModel(cfg *config.Config) *Model {
	theme.LoadUserThemes(cfg.TUI.ThemesDir)
	isDarkBg := termenv.HasDarkBackground()
	palette, name := theme.ResolvePalette(&cfg.TUI, isDarkBg)
	st := theme.BuildStyles(palette)

	ti := newInput("Search...", 200, 50, st)
	ti.Focus()

	// Add tab text inputs
	var addInputs [2]textinput.Model
	for i, ph := range []string{"URL", "Title"} {
		addInputs[i] = newInput(ph, 500, 40, st)
	}
	addText := textarea.New()
	addText.Placeholder = "Optional text content"
	addText.Prompt = ""
	addText.ShowLineNumbers = false
	addText.CharLimit = 100_000
	addText.SetWidth(60)
	addText.SetHeight(AddTextHeight)
	applyTextareaStyles(&addText, st)

	// Spinner
	s := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(st.Spin),
	)
	h := help.New()
	h.ShowAll = true
	applyHelpStyles(&h, st)

	m := &Model{
		TextInput:          ti,
		Spinner:            s,
		Help:               h,
		Keys:               component.NewKeyMap(cfg.Hotkeys.TUI),
		IsDarkBg:           isDarkBg,
		State:              StateInput,
		PrevState:          StateInput,
		Cfg:                cfg,
		Client:             client.New(cfg.BaseURL(""), client.WithAccessToken(cfg.App.AccessToken)),
		SelectedIdx:        -1,
		DialogReturnTab:    -1,
		Limit:              ResultsPageSize,
		WsChan:             make(chan tea.Msg, 10),
		WsDone:             make(chan struct{}),
		Styles:             st,
		ThemeName:          name,
		ThemePickerMode:    cfg.TUI.ColorScheme,
		AddInputs:          addInputs,
		AddText:            addText,
		RulesAliasKeyInput: newInput("keyword...", 200, 40, st),
		RulesAliasValInput: newInput("value...", 200, 40, st),
		RulesFormFocus:     RulesFocusList, // start on list
		RulesEditingIdx:    -1,
		PrioritizeInput:    newInput("URL pattern...", 500, 40, st),
		TipIdx:             rand.Intn(len(SearchTips)),
	}
	for _, section := range RulesSections {
		if section.Aliases {
			continue
		}
		m.RulesPatternInputs[section.ID] = newInput(section.Placeholder, 200, 40, st)
	}
	m.Workspace = viewport.New(80, 20)
	if m.ThemePickerMode == "" {
		m.ThemePickerMode = "auto"
	}
	m.SetTerminalBg(palette.Base00)
	return m
}

func (m *Model) ApplyTheme(p theme.Palette) {
	m.Styles = theme.BuildStyles(p)
	m.ThemeName = p.Name
	applyInputStyles(&m.TextInput, m.Styles)
	for i := range m.AddInputs {
		applyInputStyles(&m.AddInputs[i], m.Styles)
	}
	applyTextareaStyles(&m.AddText, m.Styles)
	for i := range m.RulesPatternInputs {
		applyInputStyles(&m.RulesPatternInputs[i], m.Styles)
	}
	applyInputStyles(&m.RulesAliasKeyInput, m.Styles)
	applyInputStyles(&m.RulesAliasValInput, m.Styles)
	applyInputStyles(&m.PrioritizeInput, m.Styles)
	m.Spinner.Style = m.Styles.Spin
	applyHelpStyles(&m.Help, m.Styles)
	m.SetTerminalBg(p.Base00)
}

func applyHelpStyles(h *help.Model, st theme.Styles) {
	h.Styles.ShortKey = st.HintKey
	h.Styles.ShortDesc = st.Hint
	h.Styles.ShortSeparator = st.Hint
	h.Styles.Ellipsis = st.Hint
	h.Styles.FullKey = st.HintKey
	h.Styles.FullDesc = st.HelpAction
	h.Styles.FullSeparator = st.Div
}

func (m *Model) ScrollToSelected() {
	if m.SelectedIdx < 0 || m.SelectedIdx >= len(m.LineOffsets) {
		return
	}
	target := m.LineOffsets[m.SelectedIdx]
	vpH := m.Viewport.Height
	curY := m.Viewport.YOffset
	if target < curY {
		m.Viewport.SetYOffset(target)
	}
	if target >= curY+vpH {
		m.Viewport.SetYOffset(target - vpH + 3)
	}
}

func (m *Model) GetTotalResults() int {
	if m.Results == nil {
		return 0
	}
	c := len(m.Results.History) + len(m.VisibleDocuments())
	if c > m.Limit {
		return m.Limit + 1
	}
	return c
}

func (m *Model) GetSelectedURL() string {
	if m.Results == nil || m.SelectedIdx < 0 || m.SelectedIdx == m.Limit {
		return ""
	}
	if m.SelectedIdx < len(m.Results.History) {
		return m.Results.History[m.SelectedIdx].URL
	}
	documents := m.VisibleDocuments()
	docIdx := m.SelectedIdx - len(m.Results.History)
	if docIdx < len(documents) {
		return documents[docIdx].URL
	}
	return ""
}

func (m *Model) GetSelectedTitle() string {
	if m.Results == nil || m.SelectedIdx < 0 || m.SelectedIdx == m.Limit {
		return ""
	}
	if m.SelectedIdx < len(m.Results.History) {
		return m.Results.History[m.SelectedIdx].Title
	}
	documents := m.VisibleDocuments()
	docIdx := m.SelectedIdx - len(m.Results.History)
	if docIdx < len(documents) {
		return documents[docIdx].Title
	}
	return ""
}

func (m *Model) GetSelectedDocument() *document.Document {
	if m.Results == nil || m.SelectedIdx < 0 || m.SelectedIdx < len(m.Results.History) || m.SelectedIdx == m.Limit {
		return nil
	}
	documents := m.VisibleDocuments()
	idx := m.SelectedIdx - len(m.Results.History)
	if idx < len(documents) {
		return documents[idx]
	}
	return nil
}

func (m *Model) VisibleDocuments() []*document.Document {
	if m.Results == nil {
		return nil
	}
	return slices.Clone(m.Results.Documents)
}

func (m *Model) SortedSettingsItems() []SettingsItem {
	items := make([]SettingsItem, 0, len(m.Cfg.Hotkeys.TUI))
	for k, v := range m.Cfg.Hotkeys.TUI {
		items = append(items, SettingsItem{Key: k, Action: config.Action(v)})
	}
	slices.SortFunc(items, func(a, b SettingsItem) int {
		return strings.Compare(a.Key, b.Key)
	})
	return items
}

func (m *Model) SortedAliasKeys() []string {
	keys := make([]string, 0, len(m.RulesData.Aliases))
	for k := range m.RulesData.Aliases {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (m *Model) RulesSectionLen(section int) int {
	switch section {
	case RulesSectionSkip:
		return len(m.RulesData.Skip)
	case RulesSectionPriority:
		return len(m.RulesData.Priority)
	case RulesSectionVersioning:
		return len(m.RulesData.Versioning)
	case RulesSectionAliases:
		return len(m.RulesData.Aliases)
	}
	return 0
}

func (m *Model) OpenDeleteDialog(title, label string, returnTab int, confirm func() tea.Cmd) {
	m.OverlayOffX, m.OverlayOffY = 0, 0
	m.State = StateDialog
	m.DialogMsg = title
	m.DialogURL = label
	m.DialogBtnIdx = 0
	m.DialogReturnTab = returnTab
	m.DialogConfirm = confirm
}

func (m *Model) OpenThemePicker() {
	m.OrigThemeName = m.ThemeName
	m.OrigDarkTheme = m.Cfg.TUI.DarkTheme
	m.OrigLightTheme = m.Cfg.TUI.LightTheme
	m.OrigColorScheme = m.Cfg.TUI.ColorScheme
	darkNames, lightNames := theme.ClassifyThemes()
	m.DarkThemeIdx = 0
	for i, name := range darkNames {
		if name == m.Cfg.TUI.DarkTheme {
			m.DarkThemeIdx = i
			break
		}
	}
	m.LightThemeIdx = 0
	for i, name := range lightNames {
		if name == m.Cfg.TUI.LightTheme {
			m.LightThemeIdx = i
			break
		}
	}
	m.ThemePickerSection = 0
	for i, name := range theme.ThemeNames() {
		if name == m.ThemeName {
			m.ThemePickerIdx = i
			break
		}
	}
	m.OpenOverlay(StateThemePicker)
}

func (m *Model) DismissOverlay() {
	m.IsDragging = false
	m.OverlayOffX, m.OverlayOffY = 0, 0
	m.State = m.PrevState
}

// DismissDialog returns to the correct state after closing a dialog.
func (m *Model) DismissDialog() {
	m.DismissOverlay()
	if m.DialogReturnTab >= 0 {
		m.ActiveTab = m.DialogReturnTab
		m.State = StateResults
		m.DialogReturnTab = -1
		return
	}
	m.State = StateResults
}

func (m *Model) OpenContextMenu(idx, x, y, offX, offY int) {
	m.MenuX, m.MenuY = x, y
	m.MenuIdx = idx
	m.MenuSelIdx = 0
	m.OpenOverlay(StateContextMenu)
	m.OverlayOffX, m.OverlayOffY = offX, offY
}

func (m *Model) StartDrag(x, y int) {
	m.IsDragging = true
	m.DragStartX, m.DragStartY = x, y
	m.DragOffX0, m.DragOffY0 = m.OverlayOffX, m.OverlayOffY
}

func (m *Model) Close() {
	m.ResetTerminalBg()
	close(m.WsDone)
}

func (m *Model) FocusedRulesInput() *textinput.Model {
	switch m.RulesFormFocus {
	case RulesFocusPattern:
		if m.RulesSection >= RulesSectionSkip && m.RulesSection <= RulesSectionVersioning {
			return &m.RulesPatternInputs[m.RulesSection]
		}
	case RulesFocusAliasKey:
		return &m.RulesAliasKeyInput
	case RulesFocusAliasValue:
		return &m.RulesAliasValInput
	}
	return nil
}

// removes focus from all rules form inputs
func (m *Model) BlurAllRulesInputs() {
	for i := range m.RulesPatternInputs {
		m.RulesPatternInputs[i].Blur()
	}
	m.RulesAliasKeyInput.Blur()
	m.RulesAliasValInput.Blur()
}

func ScrollIdx(idx *int, delta, minVal, maxVal int) bool {
	n := max(minVal, min(maxVal, *idx+delta))
	if n == *idx {
		return false
	}
	*idx = n
	return true
}

// OpenOverlay sets up common overlay state.
func (m *Model) OpenOverlay(state ViewState) {
	m.OverlayOffX, m.OverlayOffY = 0, 0
	m.PrevState, m.State = m.State, state
	m.TextInput.Blur()
}

// returns the result index at the given content Y offset,
// or -1 if no result is found.
func (m *Model) FindResultAtY(contentY int) int {
	for i, offset := range slices.Backward(m.LineOffsets) {
		if offset <= contentY {
			return i
		}
	}
	return -1
}

func (m *Model) PostHistoryCmd(u string) tea.Cmd {
	q, title := m.TextInput.Value(), m.GetSelectedTitle()
	return func() tea.Msg {
		if err := m.Client.PostHistory(q, u, title); err != nil {
			log.Warn().Err(err).Msg("failed to post history")
		}
		return nil
	}
}

func (m *Model) SaveRulesCmd() tea.Cmd {
	skip := strings.Join(m.RulesData.Skip, "\n")
	priority := strings.Join(m.RulesData.Priority, "\n")
	versioning := strings.Join(m.RulesData.Versioning, "\n")
	return func() tea.Msg {
		return RulesSavedMsg{Err: m.Client.SaveRules(skip, priority, versioning)}
	}
}

// RulesPatterns returns the mutable collection for a non-alias rule section.
func (m *Model) RulesPatterns(section int) *[]string {
	switch section {
	case RulesSectionSkip:
		return &m.RulesData.Skip
	case RulesSectionPriority:
		return &m.RulesData.Priority
	case RulesSectionVersioning:
		return &m.RulesData.Versioning
	default:
		return nil
	}
}

func (m *Model) FetchHistoryCmd() tea.Cmd {
	return func() tea.Msg {
		items, err := m.Client.FetchHistory()
		return HistoryFetchedMsg{Items: items, Err: err}
	}
}

func (m *Model) FetchRulesCmd() tea.Cmd {
	return func() tea.Msg {
		data, err := m.Client.FetchRules()
		if err != nil {
			return RulesFetchedMsg{Err: err}
		}
		if data == nil {
			return RulesFetchedMsg{}
		}
		return RulesFetchedMsg{Data: *data}
	}
}

func (m *Model) AddPageCmd(u, title, text string) tea.Cmd {
	return func() tea.Msg {
		return AddResultMsg{Err: m.Client.AddPage(u, title, text)}
	}
}

func (m *Model) AddAliasCmd(keyword, value string) tea.Cmd {
	return func() tea.Msg {
		return RulesSavedMsg{Err: m.Client.AddAlias(keyword, value)}
	}
}

// PrioritizeRuleCmd reads the latest rules before appending a priority rule.
// Result context menus are usable without visiting the Rules tab first, so
// saving the model's possibly-empty cache here could erase server-side rules.
func (m *Model) PrioritizeRuleCmd(pattern string) tea.Cmd {
	return func() tea.Msg {
		rules, err := m.Client.FetchRules()
		if err != nil {
			return RulesSavedMsg{Err: err}
		}
		rules.Priority = append(rules.Priority, pattern)
		return RulesSavedMsg{Err: m.Client.SaveRules(
			strings.Join(rules.Skip, "\n"),
			strings.Join(rules.Priority, "\n"),
			strings.Join(rules.Versioning, "\n"),
		)}
	}
}

func (m *Model) DeleteAliasCmd(alias string) tea.Cmd {
	return func() tea.Msg {
		return RulesSavedMsg{Err: m.Client.DeleteAlias(alias)}
	}
}

func (m *Model) DeleteURLCmd(u string) tea.Cmd {
	return func() tea.Msg {
		return DeleteResultMsg{Err: m.Client.DeleteDocument(u)}
	}
}

func (m *Model) DeleteHistoryEntryCmd(query, url string) tea.Cmd {
	return func() tea.Msg {
		if err := m.Client.DeleteHistoryEntry(query, url); err != nil {
			return HistoryFetchedMsg{Err: err}
		}
		items, err := m.Client.FetchHistory()
		return HistoryFetchedMsg{Items: items, Err: err}
	}
}

func newInput(placeholder string, charLimit int, width int, st theme.Styles) textinput.Model {
	inp := textinput.New()
	inp.Placeholder = placeholder
	inp.CharLimit = charLimit
	inp.Width = width
	inp.Prompt = ""
	inp.PlaceholderStyle = st.Placeholder
	return inp
}

func applyInputStyles(inp *textinput.Model, st theme.Styles) {
	inp.PlaceholderStyle = st.Placeholder
	inp.CompletionStyle = st.Placeholder
	inp.Cursor.Style = lipgloss.NewStyle().Foreground(st.PromptActive.GetForeground())
}

func applyTextareaStyles(input *textarea.Model, st theme.Styles) {
	input.FocusedStyle.Placeholder = st.Placeholder
	input.BlurredStyle.Placeholder = st.Placeholder
	input.FocusedStyle.CursorLine = lipgloss.NewStyle()
	input.BlurredStyle.CursorLine = lipgloss.NewStyle()
	input.Cursor.Style = lipgloss.NewStyle().Foreground(st.PromptActive.GetForeground())
}

func (m *Model) SetTerminalBg(hex string) {
	if hex == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "\033]11;%s\a", hex)
	m.BgSet = true
}

func (m *Model) ResetTerminalBg() {
	if m.BgSet {
		fmt.Fprint(os.Stderr, "\033]111\a")
		m.BgSet = false
	}
}
