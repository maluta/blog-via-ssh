package main

// ---------------------------------------------------------------------------
// CONTENT — the static, owner-authored parts of the site. Blog posts live as
// markdown files under posts/ (see posts.go); everything here is the "chrome"
// around them: the banner, the tagline, the Sobre bio, and the contact links.
// Edit freely — none of this requires touching the UI code in ui.go.
// ---------------------------------------------------------------------------

// banner is the ASCII wordmark shown at the top of every screen. It's a plain
// constant (no figlet dependency): regenerate it for a different name with any
// figlet-style tool and paste the result here, escaping backslashes as \\.
const banner = "                   _         _\n _ __ ___    __ _ | | _   _ | |_   __ _\n| '_ ` _ \\  / _` || || | | || __| / _` |\n| | | | | || (_| || || |_| || |_ | (_| |\n|_| |_| |_| \\__,_||_| \\__,_| \\__| \\__,_|"

// tagline sits just below the banner — a one-liner describing the blog.
const tagline = "· Se você veio pelo LinkedIn comente 'Less scrolling. More SSH.'"

// motto is the sign-off shown at the right edge of the footer, next to the
// key hints. It's dropped automatically on terminals too narrow to fit both.
const motto = "Less scrolling. More SSH."

// aboutText is the body of the "Sobre" tab. Blank lines separate paragraphs.
const aboutText = `Oi!

Eu sou Tiago Maluta. Para o lado mais profissional, existe o LinkedIn.

Aqui é outra coisa.

Este é meu "canto nerd" na internet: um blog que roda no terminal (e se você está aqui você meio que já entendeu).

Aqui você contra notas, experimentos, reflexões de coisas mais técnicas que vou explorando e aprendendo.

Para você que vive no terminal, esse blog é para você.

ssh dev.maluta.com.br
`

// Link is one labelled entry on the "Contato" tab.
type Link struct {
	Label string
	URL   string
}

// links are shown on the "Contato" tab, in order.
var links = []Link{
	{"GitHub", "github.com/maluta"},
	{"LinkedIn", "linkedin.com/in/maluta"},
	{"Blog", "maluta.github.io/blog"},
	{"X", "twitter.com/maluta"},
	{"Email", "maluta@hey.com"},
}
