package main

// Temporary visual snapshot helper — run with `go test -run TestSnapshot -v`.

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestSnapshot(t *testing.T) {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.ANSI256)
	st := newStyles(r)
	m := newModel(LoadPosts("posts"), 90, 30, st, termenv.ANSI256)
	fmt.Println("=== INDEX ===")
	fmt.Println(m.View())

	m.activeTab = 2
	fmt.Println("=== CONTATO ===")
	fmt.Println(m.View())
}

func TestMottoDropsWhenNarrow(t *testing.T) {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.ANSI256)
	st := newStyles(r)

	// The motto may also appear in owner content (e.g. the tagline), so only
	// the footer line — the one with the "q sair" hint — is inspected.
	footerLine := func(view string) string {
		for _, line := range strings.Split(view, "\n") {
			if strings.Contains(line, "q sair") {
				return line
			}
		}
		return ""
	}

	wide := newModel(nil, 90, 30, st, termenv.ANSI256)
	if !strings.Contains(footerLine(wide.View()), motto) {
		t.Errorf("motto missing from the footer on a wide terminal")
	}
	narrow := newModel(nil, 40, 24, st, termenv.ANSI256)
	if strings.Contains(footerLine(narrow.View()), motto) {
		t.Errorf("motto should be dropped from the footer on a narrow terminal")
	}
}

// TestReaderOSC8Integrity opens the newest real post and scrolls through it,
// checking every frame for OSC 8 sequences broken by wrapping — glamour can
// hard-wrap mid-word, and a newline inside the sequence corrupts it.
func TestReaderOSC8Integrity(t *testing.T) {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.ANSI256)
	st := newStyles(r)
	m := newModel(LoadPosts("posts"), 100, 30, st, termenv.ANSI256)

	opens := regexp.MustCompile(`\x1b\]8;;`)
	complete := regexp.MustCompile(`\x1b\]8;;[^\x1b]*\x1b\\`)

	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mod := mm.(model)
	seen := 0
	for i := 0; i < 60; i++ {
		view := mod.View()
		o, c := len(opens.FindAllString(view, -1)), len(complete.FindAllString(view, -1))
		if o != c {
			t.Fatalf("frame %d: %d aberturas OSC 8, só %d completas (sequência quebrada)", i, o, c)
		}
		seen += c
		mm, _ = mod.Update(tea.KeyMsg{Type: tea.KeyDown})
		mod = mm.(model)
	}
	if seen == 0 {
		t.Skip("nenhum link OSC 8 visível nos posts atuais — nada a verificar")
	}
}

func TestMarkdownMono(t *testing.T) {
	md := "# Título\n\nTexto com **negrito** e *itálico* e `code`.\n\n```go\nfmt.Println(\"oi\")\n```\n\n[link](https://ex.com)\n\n> citação\n"
	out := RenderMarkdown(md, 60, termenv.ANSI256)
	fmt.Printf("%q\n", out)
	fmt.Println(out)

	if !strings.Contains(out, "\x1b]8;;https://ex.com\x1b\\") {
		t.Errorf("link should be wrapped in an OSC 8 hyperlink")
	}
	if strings.Contains(ansiSeq.ReplaceAllString(strings.ReplaceAll(out, "\x1b]8;;https://ex.com\x1b\\", ""), ""), "https://ex.com") {
		t.Errorf("url should not be visible outside the OSC 8 sequence")
	}
	for _, c := range []string{"\x00", "\x01", "\x02", "\x03"} {
		if strings.Contains(out, c) {
			t.Errorf("leftover sentinel %q in output", c)
		}
	}

	plain := RenderMarkdown("[link](https://ex.com)\n", 60, termenv.Ascii)
	if !strings.Contains(plain, "link (https://ex.com)") {
		t.Errorf("Ascii profile should render %q, got %q", "link (https://ex.com)", plain)
	}
	if strings.Contains(plain, "\x1b") {
		t.Errorf("Ascii profile output should have no escapes, got %q", plain)
	}
}
