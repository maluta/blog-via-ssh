package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	gstyles "github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	"gopkg.in/yaml.v3"
)

// postsDir is where blog posts live, relative to the server's working directory.
const postsDir = "posts"

// Post is one blog entry, loaded from a markdown file with YAML frontmatter.
type Post struct {
	Title string
	Date  time.Time
	Tags  []string
	Slug  string // derived from the filename, e.g. "2026-06-24-apps-via-ssh"
	Body  string // raw markdown (rendered with glamour when opened)
}

// frontmatter is the YAML block at the top of a post file.
type frontmatter struct {
	Title string    `yaml:"title"`
	Date  time.Time `yaml:"date"`
	Tags  []string  `yaml:"tags"`
}

// LoadPosts reads every *.md file in dir, parses its frontmatter + body, and
// returns the posts sorted newest-first. Unparseable files are skipped and
// logged rather than failing the whole load — one broken post shouldn't take
// the blog down. A missing directory yields an empty slice, not an error.
func LoadPosts(dir string) []Post {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn("could not read posts dir", "dir", dir, "error", err)
		}
		return nil
	}

	var posts []Post
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			log.Warn("could not read post", "file", e.Name(), "error", err)
			continue
		}

		fm, body := splitFrontmatter(string(raw))
		var meta frontmatter
		if fm != "" {
			if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
				log.Warn("bad frontmatter, skipping", "file", e.Name(), "error", err)
				continue
			}
		}

		slug := strings.TrimSuffix(e.Name(), ".md")
		if meta.Title == "" {
			meta.Title = slug // fall back to the filename so it's never blank
		}
		posts = append(posts, Post{
			Title: meta.Title,
			Date:  meta.Date,
			Tags:  meta.Tags,
			Slug:  slug,
			Body:  body,
		})
	}

	// Newest first. Posts without a date sort to the bottom (zero time).
	sort.SliceStable(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})
	return posts
}

// splitFrontmatter separates a leading `---`-delimited YAML block from the body.
// If there's no frontmatter, it returns ("", wholeContent).
func splitFrontmatter(s string) (fm, body string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return "", s
	}
	// Look for the closing delimiter after the opening "---\n".
	rest := s[len("---\n"):]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", s // opened but never closed — treat as plain body
	}
	fm = rest[:idx]
	body = rest[idx+len("\n---"):]
	// Drop the rest of the closing delimiter line and any blank lines after it.
	if nl := strings.IndexByte(body, '\n'); nl >= 0 {
		body = body[nl+1:]
	} else {
		body = ""
	}
	return fm, strings.TrimLeft(body, "\n")
}

// dateLabel formats a post's date for the index; empty for undated posts.
func (p Post) dateLabel() string {
	if p.Date.IsZero() {
		return ""
	}
	return p.Date.Format("02 Jan 2006")
}

func boolPtr(b bool) *bool { return &b }

// monoStyle is the blog's monochrome glamour theme: the built-in "ascii" style
// (which never emits ANSI — literal ** and no bold) upgraded with attributes,
// and never any color. Being colorless also sidesteps the light-vs-dark
// terminal background guess, which is unreliable over SSH. Glamour quirks this
// works around: Faint is in the style schema but never rendered (so quotes get
// Italic instead), and inline code reads Prefix, not BlockPrefix.
func monoStyle() ansi.StyleConfig {
	cfg := gstyles.ASCIIStyleConfig
	cfg.Heading.Bold = boolPtr(true) // keeps the "# " prefixes as anchors
	cfg.Strong.Bold = boolPtr(true)
	cfg.Strong.BlockPrefix, cfg.Strong.BlockSuffix = "", ""
	cfg.Emph.Italic = boolPtr(true)
	cfg.Emph.BlockPrefix, cfg.Emph.BlockSuffix = "", ""
	cfg.Code.Prefix, cfg.Code.Suffix = "`", "`" // code blocks stay marked by their 2-col indent
	cfg.Code.BlockPrefix, cfg.Code.BlockSuffix = "", ""
	// Glamour always prints links as "text url" and offers no way to drop the
	// url, so mark both parts with zero-width C0 sentinels and let
	// RenderMarkdown rewrite them into OSC 8 hyperlinks afterwards.
	cfg.LinkText.BlockPrefix, cfg.LinkText.BlockSuffix = "\x00", "\x01"
	cfg.Link.BlockPrefix, cfg.Link.BlockSuffix = "\x02", "\x03"
	cfg.BlockQuote.Italic = boolPtr(true)
	cfg.HorizontalRule.Format = "\n──────────\n" // matches the UI's rules
	return cfg
}

// Sentinel-delimited link parts emitted by monoStyle: one or more \x00text\x01
// runs (the link text may have several styled children), then possibly wrapped
// whitespace, then \x02url\x03. Bare autolinks have no text part.
var (
	mdLink   = regexp.MustCompile(`(?s)((?:\x00.*?\x01)+)\s*\x02(.*?)\x03`)
	bareLink = regexp.MustCompile(`(?s)\x02(.*?)\x03`)
	ansiSeq  = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	spaces   = regexp.MustCompile(`\s+`)
	sentinel = strings.NewReplacer("\x00", "", "\x01", "", "\x02", "", "\x03", "")
)

// hyperlink renders label as an OSC 8 hyperlink to url — clickable in modern
// terminals, silently ignored (plain underlined label) in older ones. Ascii
// clients (TERM=dumb) get "label (url)" instead so the url stays reachable.
func hyperlink(label, url string, profile termenv.Profile) string {
	if profile == termenv.Ascii {
		if label == url {
			return url
		}
		return label + " (" + url + ")"
	}
	return "\x1b]8;;" + url + "\x1b\\" + "\x1b[4m" + label + "\x1b[0m" + "\x1b]8;;\x1b\\"
}

// linkify rewrites the sentinel-marked links in glamour output into OSC 8
// hyperlinks showing only the link text. Dropping the url can leave a wrapped
// paragraph's right edge ragged (the wrap was computed with it present), which
// never overflows — it only under-fills.
func linkify(out string, profile termenv.Profile) string {
	// Glamour may have hard-wrapped the label or even the url mid-word; a
	// newline smuggled into the OSC 8 sequence corrupts it, so the label's
	// whitespace runs are collapsed and the url's removed entirely.
	label := func(s string) string {
		return strings.TrimSpace(spaces.ReplaceAllString(sentinel.Replace(ansiSeq.ReplaceAllString(s, "")), " "))
	}
	url := func(s string) string {
		return spaces.ReplaceAllString(sentinel.Replace(ansiSeq.ReplaceAllString(s, "")), "")
	}
	out = mdLink.ReplaceAllStringFunc(out, func(m string) string {
		g := mdLink.FindStringSubmatch(m)
		return hyperlink(label(g[1]), url(g[2]), profile)
	})
	out = bareLink.ReplaceAllStringFunc(out, func(m string) string {
		u := url(bareLink.FindStringSubmatch(m)[1])
		return hyperlink(u, u, profile)
	})
	return sentinel.Replace(out)
}

// RenderMarkdown turns a post's markdown body into styled terminal output using
// glamour, wrapped to width. The profile is the client's (from the session
// renderer), so an attribute-less terminal gets plain text instead of stray
// escapes — glamour's own default is a hardcoded TrueColor. On any error we
// fall back to the raw markdown.
func RenderMarkdown(md string, width int, profile termenv.Profile) string {
	if width < 20 {
		width = 20
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(monoStyle()),
		glamour.WithWordWrap(width),
		glamour.WithColorProfile(profile),
	)
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return strings.TrimRight(linkify(out, profile), "\n")
}
