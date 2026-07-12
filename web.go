package main

// ---------------------------------------------------------------------------
// WEB — a one-page HTTP(S) landing for people who open dev.maluta.com.br in a
// browser. The blog itself lives on SSH (main.go); this page's only job is to
// hand the visitor the `ssh` command. It reuses the owner content in
// content.go and the live posts from posts.go, so there is a single source of
// truth for everything shown.
//
// Modes (decided by the WEB_DOMAIN environment variable):
//   - WEB_DOMAIN=dev.maluta.com.br  → production: HTTPS on :443 with an
//     automatic Let's Encrypt certificate (autocert), plus :80 serving the
//     ACME HTTP-01 challenge and redirecting everything else to https.
//     Certificates are cached in .autocert/ under the working directory.
//   - WEB_DOMAIN unset              → local preview: plain HTTP on :8080.
//
// The same setcap that lets the binary bind port 22 also covers 80/443 — the
// capability is per-binary, not per-port.
// ---------------------------------------------------------------------------

import (
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"golang.org/x/crypto/acme/autocert"
)

// defaultWebDomain is what the landing advertises when WEB_DOMAIN is unset
// (local preview) — the page should always show the real address.
const defaultWebDomain = "dev.maluta.com.br"

// startWeb runs the web side of the app. It blocks, so call it in a goroutine.
func startWeb() {
	domain := os.Getenv("WEB_DOMAIN")

	mux := http.NewServeMux()
	mux.HandleFunc("/", landingHandler(domain))

	if domain == "" {
		addr := "localhost:8080"
		log.Info("Starting web (local preview)", "address", "http://"+addr)
		srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		if err := srv.ListenAndServe(); err != nil {
			log.Error("web server stopped", "error", err)
		}
		return
	}

	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domain),
		Cache:      autocert.DirCache(".autocert"),
	}

	// :80 answers the ACME HTTP-01 challenge and 302s everything else to https.
	go func() {
		srv := &http.Server{Addr: ":80", Handler: m.HTTPHandler(nil), ReadHeaderTimeout: 5 * time.Second}
		if err := srv.ListenAndServe(); err != nil {
			log.Error("web :80 stopped", "error", err)
		}
	}()

	log.Info("Starting web", "domain", domain)
	srv := &http.Server{
		Addr:              ":443",
		Handler:           mux,
		TLSConfig:         m.TLSConfig(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	// Cert and key come from the autocert manager via TLSConfig.
	if err := srv.ListenAndServeTLS("", ""); err != nil {
		log.Error("web :443 stopped", "error", err)
	}
}

// landingData is everything the template needs, assembled per request so new
// posts show up immediately — same behaviour as the SSH side (teaHandler).
type landingData struct {
	Domain  string
	Cmd     string
	CmdLen  int // width of the typing animation, in ch units
	Banner  string
	Tagline string
	Motto   string
	Posts   []webPost
}

type webPost struct {
	Title string
	Date  string
}

// landingHandler serves the single page. Any other path is a 404 — there is
// nothing else here on purpose.
func landingHandler(domain string) http.HandlerFunc {
	if domain == "" {
		domain = defaultWebDomain
	}
	cmd := "ssh " + domain

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		posts := LoadPosts(postsDir)
		if len(posts) > 3 {
			posts = posts[:3]
		}
		data := landingData{
			Domain:  domain,
			Cmd:     cmd,
			CmdLen:  len(cmd),
			Banner:  banner,
			Tagline: tagline,
			Motto:   motto,
		}
		for _, p := range posts {
			data.Posts = append(data.Posts, webPost{Title: p.Title, Date: p.dateLabel()})
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := landingTmpl.Execute(w, data); err != nil {
			log.Error("landing render", "error", err)
		}
	}
}

var landingTmpl = template.Must(template.New("landing").Parse(landingHTML))

// The page is fully self-contained (inline CSS/JS, no external assets) and
// deliberately monochrome-plus-amber: the TUI itself renders without colors
// (see monoStyle in posts.go), and the amber is the one accent — a nod to
// amber-phosphor CRTs — reserved for the cursor and the copy action.
const landingHTML = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>maluta — um blog via SSH</title>
<meta name="description" content="Blog técnico que roda no terminal. Acesse: ssh {{.Domain}}">
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3E%3Crect width='16' height='16' fill='%23101216'/%3E%3Crect x='3' y='4' width='5' height='9' fill='%23f0a45d'/%3E%3C/svg%3E">
<style>
:root{
  --bg:#101216; --panel:#15181e; --line:#2a2f38;
  --fg:#d8d4c8; --faint:#7a7f88; --amber:#f0a45d; --amber-ink:#221607;
}
*{box-sizing:border-box;margin:0;padding:0}
body{
  background:var(--bg); color:var(--fg);
  font:15px/1.6 ui-monospace,"Cascadia Code","JetBrains Mono",Menlo,Consolas,monospace;
  min-height:100svh; display:grid; place-items:center; padding:28px 16px;
}
main{width:min(680px,100%)}

/* the terminal ------------------------------------------------------------ */
.term{
  background:var(--panel); border:1px solid var(--line); border-radius:10px;
  padding:18px 20px 20px; box-shadow:0 24px 70px rgba(0,0,0,.5);
}
.cmdline{white-space:nowrap; overflow:hidden}
.prompt{color:var(--faint)}
.typed{
  display:inline-block; overflow:hidden; white-space:nowrap; vertical-align:bottom;
  width:0; animation:type 1.3s steps({{.CmdLen}}) .5s forwards;
}
.cursor{
  display:inline-block; width:.6em; height:1.15em; margin-left:1px;
  background:var(--amber); vertical-align:text-bottom;
  animation:blink 1.1s steps(1) infinite;
}
.tui{
  border:1px solid var(--line); border-radius:8px; padding:14px 16px;
  margin-top:14px; opacity:0; animation:appear .35s ease 2s forwards;
}
.banner{
  font-weight:700; line-height:1.15; overflow-x:auto;
  font-size:clamp(7px,2.1vw,13.5px);
}
.tagline{color:var(--faint); margin:10px 0 14px; font-size:13px}
/* content you only get to read over ssh */
.blur{filter:blur(5px); user-select:none}
.tabs{display:flex; gap:2px; margin-bottom:12px}
.tab{padding:1px 12px; color:var(--faint)}
.tab.active{background:var(--fg); color:var(--panel); font-weight:700}
.posts{list-style:none; display:grid; gap:6px; margin-bottom:14px}
.posts li{
  display:flex; justify-content:space-between; gap:14px;
  color:var(--faint); padding-left:12px;
}
.posts li:first-child{
  border-left:3px solid var(--fg); padding-left:9px; color:var(--fg); font-weight:700;
}
.posts .date{flex-shrink:0; font-weight:400; color:var(--faint); font-size:13px}
.hints{
  display:flex; justify-content:space-between; gap:12px; flex-wrap:wrap;
  border-top:1px solid var(--line); padding-top:10px;
  color:var(--faint); font-size:12px;
}
.motto{font-style:italic}

/* below the terminal ------------------------------------------------------ */
.lead{color:var(--faint); margin:20px 0 12px}
.lead b{color:var(--fg)}
.cta{display:flex; gap:10px; flex-wrap:wrap; align-items:stretch}
.cta code{
  flex:1 1 220px; display:flex; align-items:center;
  background:var(--panel); border:1px solid var(--line); border-radius:8px;
  padding:10px 14px; color:var(--amber); user-select:all; font-size:15px;
}
.cta button{
  font:inherit; font-weight:700; cursor:pointer;
  background:var(--amber); color:var(--amber-ink);
  border:0; border-radius:8px; padding:10px 18px;
}
.cta button:hover{filter:brightness(1.08)}
button:focus-visible{outline:2px solid var(--amber); outline-offset:2px}

@keyframes type{to{width:{{.CmdLen}}ch}}
@keyframes blink{50%{opacity:0}}
@keyframes appear{to{opacity:1}}
@media (prefers-reduced-motion:reduce){
  .typed{width:{{.CmdLen}}ch; animation:none}
  .tui{opacity:1; animation:none}
  .cursor{animation:none}
}
</style>
</head>
<body>
<main>
  <section class="term" aria-label="prévia do blog no terminal">
    <p class="cmdline"><span class="prompt">~ $</span> <span class="typed">{{.Cmd}}</span><span class="cursor" aria-hidden="true"></span></p>
    <div class="tui">
      <pre class="banner">{{.Banner}}</pre>
      <p class="tagline blur" aria-hidden="true">{{.Tagline}}</p>
      <nav class="tabs" aria-hidden="true"><span class="tab active">Blog</span><span class="tab">Sobre</span><span class="tab">Contato</span></nav>
      <ul class="posts">
        {{- range .Posts}}
        <li><span class="blur" aria-hidden="true">{{.Title}}</span>{{if .Date}}<span class="date">{{.Date}}</span>{{end}}</li>
        {{- end}}
      </ul>
      <p class="hints"><span>tab alternar · ↑/↓ navegar · enter abrir · q sair</span><span class="motto">{{.Motto}}</span></p>
    </div>
  </section>

  <p class="lead">Este blog não tem versão web — <b>ele roda no seu terminal</b>. Copie o comando e cole em qualquer terminal com ssh (macOS, Linux, Windows):</p>

  <div class="cta">
    <code id="cmd">{{.Cmd}}</code>
    <button id="copy" type="button" aria-live="polite">copiar</button>
  </div>
</main>
<script>
const btn = document.getElementById('copy');
btn.addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(document.getElementById('cmd').textContent);
    btn.textContent = 'copiado ✓';
  } catch {
    btn.textContent = 'selecione e copie';
  }
  setTimeout(() => { btn.textContent = 'copiar'; }, 2200);
});
</script>
</body>
</html>
`
