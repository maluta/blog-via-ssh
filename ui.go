package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// tabs are the top-level sections. Blog is the default (index 0).
var tabs = []string{"Blog", "Sobre", "Contato"}

// blogMode is the sub-state of the Blog tab: browsing the index vs reading a post.
type blogMode int

const (
	blogList blogMode = iota
	blogReading
)

// ---------------------------------------------------------------------------
// Styles — deliberately monochrome: hierarchy comes from bold/faint/reverse and
// rules, never from color. Built per session from the client's renderer, because
// the package-level lipgloss renderer detects the color profile from the
// *server's* stdout (no TTY under systemd → every ANSI attribute stripped).
// ---------------------------------------------------------------------------

type styles struct {
	banner  lipgloss.Style
	tagline lipgloss.Style
	dim     lipgloss.Style // footer, metadata, empty-state message
	rule    lipgloss.Style // horizontal ─ separators

	tabActive   lipgloss.Style
	tabInactive lipgloss.Style

	itemLabel lipgloss.Style // opened-post title, Contato labels
	link      lipgloss.Style
	motto     lipgloss.Style // footer sign-off, right-aligned
	box       lipgloss.Style
	content   lipgloss.Style // fixed-size wrapper around the active tab body

	listNormalTitle   lipgloss.Style
	listNormalDesc    lipgloss.Style
	listSelectedTitle lipgloss.Style
	listSelectedDesc  lipgloss.Style

	pagActive   lipgloss.Style
	pagInactive lipgloss.Style
}

func newStyles(r *lipgloss.Renderer) styles {
	// The selected list item swaps the default "│" left border for a solid "▌".
	// Border (1 cell) + padding 1 occupies the same 2 cells as the normal items'
	// padding-left 2, so titles stay aligned whether selected or not.
	selBar := lipgloss.Border{Left: "▌"}
	return styles{
		banner:  r.NewStyle().Bold(true),
		tagline: r.NewStyle().Faint(true),
		dim:     r.NewStyle().Faint(true),
		rule:    r.NewStyle().Faint(true),

		tabActive:   r.NewStyle().Bold(true).Reverse(true).Padding(0, 2),
		tabInactive: r.NewStyle().Faint(true).Padding(0, 2),

		itemLabel: r.NewStyle().Bold(true),
		link:      r.NewStyle().Underline(true),
		motto:     r.NewStyle().Faint(true).Italic(true),
		box:       r.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 3),
		content:   r.NewStyle(),

		listNormalTitle:   r.NewStyle().Padding(0, 0, 0, 2),
		listNormalDesc:    r.NewStyle().Faint(true).Padding(0, 0, 0, 2),
		listSelectedTitle: r.NewStyle().Bold(true).Border(selBar, false, false, false, true).Padding(0, 0, 0, 1),
		listSelectedDesc:  r.NewStyle().Faint(true).Border(selBar, false, false, false, true).Padding(0, 0, 0, 1),

		pagActive:   r.NewStyle(),
		pagInactive: r.NewStyle().Faint(true),
	}
}

// ---------------------------------------------------------------------------
// postItem adapts a Post to the bubbles/list Item interface. Implementing
// list.DefaultItem (Title/Description/FilterValue) lets us use the default
// two-line delegate.
// ---------------------------------------------------------------------------

type postItem struct{ p Post }

func (i postItem) Title() string { return i.p.Title }

func (i postItem) Description() string {
	parts := []string{}
	if d := i.p.dateLabel(); d != "" {
		parts = append(parts, d)
	}
	if len(i.p.Tags) > 0 {
		parts = append(parts, "#"+strings.Join(i.p.Tags, " #"))
	}
	return strings.Join(parts, "  ·  ")
}

func (i postItem) FilterValue() string { return i.p.Title }

// ---------------------------------------------------------------------------
// The model.
// ---------------------------------------------------------------------------

type model struct {
	width, height int
	contentW      int // usable width inside the box for the active tab
	contentH      int // usable height inside the box for the active tab

	st      styles
	profile termenv.Profile // the client's color profile, for glamour

	activeTab int
	posts     []Post

	blog   blogMode
	list   list.Model
	reader viewport.Model
}

// newModel builds the initial model for one connection: it loads the post index
// into the list and sizes the components from the (already known) terminal size.
// Styles and profile come from the session's renderer (see teaHandler).
func newModel(posts []Post, width, height int, st styles, profile termenv.Profile) model {
	d := list.NewDefaultDelegate()
	// Replace the delegate's colored defaults with the monochrome set.
	d.Styles.NormalTitle = st.listNormalTitle
	d.Styles.NormalDesc = st.listNormalDesc
	d.Styles.SelectedTitle = st.listSelectedTitle
	d.Styles.SelectedDesc = st.listSelectedDesc

	l := list.New(postItems(posts), d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	// The default pagination dots carry colors from the global renderer.
	l.Styles.ActivePaginationDot = st.pagActive.SetString("•")
	l.Styles.InactivePaginationDot = st.pagInactive.SetString("•")

	m := model{
		width:   width,
		height:  height,
		st:      st,
		profile: profile,
		posts:   posts,
		list:    l,
		reader:  viewport.New(0, 0),
	}
	m.layout()
	return m
}

// postItems converts posts to list items.
func postItems(posts []Post) []list.Item {
	items := make([]list.Item, len(posts))
	for i, p := range posts {
		items[i] = postItem{p}
	}
	return items
}

// layout recomputes the content area and resizes the sub-components. Called on
// startup and on every resize. Width is capped for comfortable reading; the
// height reserves rows for the banner, tab bar and footer.
func (m *model) layout() {
	cw := m.width - 10 // border (2) + padding (6) + a little slack
	if cw > 80 {
		cw = 80
	}
	if cw < 20 {
		cw = 20
	}
	// Everything around the content costs 10 rows — tagline (1), blank (1),
	// tabs (1), two rules (2), footer (1), vertical padding (2), border (2) —
	// plus however many lines the banner has (measured, so it can be swapped
	// in content.go without breaking the frame).
	ch := m.height - lipgloss.Height(banner) - 10
	if ch < 4 {
		ch = 4
	}
	m.contentW, m.contentH = cw, ch
	m.list.SetSize(cw, ch)
	m.reader.Width = cw
	m.reader.Height = ch
}

func (m model) Init() tea.Cmd { return nil }

// Update routes messages. Key handling depends on which tab is active and, on
// the Blog tab, whether we're browsing the index or reading a post.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// Quit works from anywhere.
		if key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}

		// Reading a post: the viewport owns the keys; esc returns to the index.
		if m.activeTab == 0 && m.blog == blogReading {
			if key == "esc" {
				m.blog = blogList
				return m, nil
			}
			var cmd tea.Cmd
			m.reader, cmd = m.reader.Update(msg)
			return m, cmd
		}

		// Browsing the Blog index: enter opens, arrows/j/k move the list.
		if m.activeTab == 0 {
			switch key {
			case "enter":
				if it, ok := m.list.SelectedItem().(postItem); ok {
					body := RenderMarkdown(it.p.Body, m.contentW, m.profile)
					m.reader.SetContent(m.postHeader(it.p) + "\n" + body)
					m.reader.GotoTop()
					m.blog = blogReading
				}
				return m, nil
			case "r":
				m.posts = LoadPosts(postsDir)
				m.list.SetItems(postItems(m.posts))
				return m, nil
			case "tab":
				m.activeTab = (m.activeTab + 1) % len(tabs)
				return m, nil
			case "1", "2", "3":
				m.jumpTab(key)
				return m, nil
			}
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		// Sobre / Contato: plain tab switching, including left/right.
		switch key {
		case "esc":
			return m, tea.Quit
		case "left", "h":
			if m.activeTab > 0 {
				m.activeTab--
			}
		case "right", "l":
			if m.activeTab < len(tabs)-1 {
				m.activeTab++
			}
		case "tab":
			m.activeTab = (m.activeTab + 1) % len(tabs)
		case "1", "2", "3":
			m.jumpTab(key)
		}
	}
	return m, nil
}

// jumpTab selects a tab from a "1"/"2"/"3" key.
func (m *model) jumpTab(key string) {
	i := int(key[0] - '1')
	if i >= 0 && i < len(tabs) {
		m.activeTab = i
	}
}

// View assembles banner + tab bar + active tab content + footer, framed and
// centered in the terminal. Rules take the place of the blank lines that used
// to surround the content, so the height budget in layout() is unchanged.
func (m model) View() string {
	var b strings.Builder
	rule := m.st.rule.Render(strings.Repeat("─", m.contentW))

	b.WriteString(m.st.banner.Render(banner))
	b.WriteString("\n")
	b.WriteString(m.st.tagline.Render(tagline))
	b.WriteString("\n\n")
	b.WriteString(m.renderTabs())
	b.WriteString("\n" + rule + "\n")

	// Fixed-height content keeps the frame from jumping between tabs.
	content := m.st.content.Width(m.contentW).Height(m.contentH).Render(m.tabContent())
	b.WriteString(content)

	b.WriteString("\n" + rule + "\n")
	// Key hints on the left, motto on the right; the motto is dropped when the
	// two wouldn't fit with at least two cells of breathing room between them.
	help := m.footerHelp()
	footer := m.st.dim.Render(help)
	if gap := m.contentW - lipgloss.Width(help) - lipgloss.Width(motto); gap >= 2 {
		footer += strings.Repeat(" ", gap) + m.st.motto.Render(motto)
	}
	b.WriteString(footer)

	box := m.st.box.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderTabs draws the horizontal tab bar, highlighting the active tab.
func (m model) renderTabs() string {
	cells := make([]string, len(tabs))
	for i, label := range tabs {
		if i == m.activeTab {
			cells[i] = m.st.tabActive.Render(label)
		} else {
			cells[i] = m.st.tabInactive.Render(label)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cells...)
}

// tabContent renders the body of the active tab.
func (m model) tabContent() string {
	switch m.activeTab {
	case 0:
		return m.renderBlog()
	case 1:
		return aboutText
	case 2:
		return m.renderContact()
	}
	return ""
}

// renderBlog shows either the post index or the reader.
func (m model) renderBlog() string {
	if m.blog == blogReading {
		return m.reader.View()
	}
	if len(m.posts) == 0 {
		return m.st.dim.Render("Nenhum post ainda. Deixe um arquivo .md em posts/ 🙂")
	}
	return m.list.View()
}

// postHeader is the title block shown above a post's rendered body.
func (m model) postHeader(p Post) string {
	title := m.st.itemLabel.Render(p.Title)
	meta := postItem{p}.Description()
	if meta == "" {
		return title
	}
	return title + "\n" + m.st.dim.Render(meta)
}

// renderContact lists the links with URLs aligned in a column.
func (m model) renderContact() string {
	widest := 0
	for _, l := range links {
		if len(l.Label) > widest {
			widest = len(l.Label)
		}
	}
	var b strings.Builder
	for i, l := range links {
		if i > 0 {
			b.WriteString("\n")
		}
		label := m.st.itemLabel.Render(fmt.Sprintf("%-*s", widest, l.Label))
		fmt.Fprintf(&b, "%s  %s", label, m.st.link.Render(l.URL))
	}
	return b.String()
}

// footerHelp is the context-sensitive key hint at the bottom.
func (m model) footerHelp() string {
	switch {
	case m.activeTab == 0 && m.blog == blogReading:
		return "↑↓ rolar · esc voltar · q sair"
	// The r-to-reload key still works on the index; it's left out of the hint
	// to keep room for the motto (it's an owner shortcut, not a visitor one).
	case m.activeTab == 0:
		return "tab/1-3 abas · ↑↓ mover · enter abrir · q sair"
	default:
		return "tab/1-3 · ←→ trocar aba · q sair"
	}
}
