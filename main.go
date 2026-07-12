package main

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"
	lm "github.com/charmbracelet/wish/logging"
)

// Default listen settings. These are overridden by the HOST and PORT environment
// variables (see envOr), so the same binary works locally and on a server.
//   - Locally you can leave them unset (defaults to all interfaces, port 23234).
//   - On a host, "0.0.0.0" means "accept connections on every network interface",
//     which is what makes the app reachable from the public internet.
const (
	defaultHost = "0.0.0.0"
	defaultPort = "23234"
)

// envOr returns the value of environment variable key, or fallback if it's unset.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	host := envOr("HOST", defaultHost)
	port := envOr("PORT", defaultPort)

	// wish.NewServer builds an SSH server. Each option configures one aspect of it.
	srv, err := wish.NewServer(
		// The TCP address to listen on.
		wish.WithAddress(net.JoinHostPort(host, port)),
		// The server's identity key. Wish generates this file automatically on first
		// run if it doesn't exist — it's like the host key of any SSH server.
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		// Middleware runs for every connection. Order matters: the LAST one in this
		// list is the OUTERMOST (it runs first on the way in). So reading bottom-up:
		// log the connection, ensure there's a real terminal, then run our TUI.
		wish.WithMiddleware(
			bm.Middleware(teaHandler), // run the Bubble Tea program (the actual app)
			activeterm.Middleware(),   // require an interactive terminal (PTY)
			lm.Middleware(),           // log connects/disconnects to our console
		),
	)
	if err != nil {
		log.Error("Could not create server", "error", err)
		return
	}

	// Listen for Ctrl+C / kill so we can shut down cleanly.
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// The browser-facing landing page (web.go). It lives and dies with the
	// process, so it doesn't participate in the graceful shutdown below.
	go startWeb()

	log.Info("Starting SSH blog", "address", net.JoinHostPort(host, port))
	go func() {
		// ListenAndServe blocks until the server stops. ErrServerClosed is the
		// normal, expected error when we shut down on purpose.
		if err = srv.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			done <- nil
		}
	}()

	<-done // block here until a signal arrives
	log.Info("Stopping SSH blog")
	// Give in-flight connections up to 30s to finish before forcing shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server gracefully", "error", err)
	}
}

// teaHandler runs once per SSH connection. It loads the current posts (so a new
// .md file shows up on the next connection, no restart needed) and returns the
// initial UI model sized to the visitor's terminal.
func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	// Pty() gives us the pseudo-terminal: its type and window size.
	pty, _, _ := s.Pty()

	// MakeRenderer builds a lipgloss renderer from the *client's* terminal (its
	// TERM, over the SSH session). The package-level renderer would look at the
	// server's stdout instead — under systemd that's not a TTY, which strips
	// bold/faint/reverse for every visitor.
	r := bm.MakeRenderer(s)
	st := newStyles(r)

	posts := LoadPosts(postsDir)
	m := newModel(posts, pty.Window.Width, pty.Window.Height, st, r.ColorProfile())

	// WithAltScreen runs the UI in the terminal's "alternate screen" — a full-screen
	// canvas that's restored to your previous shell content when the app exits.
	return m, []tea.ProgramOption{tea.WithAltScreen()}
}
