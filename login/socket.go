package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// socketPath returns the well-known path for the login IPC socket.
// Prefers $XDG_RUNTIME_DIR (user-only, 0700); falls back to the config dir.
func socketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "skylight-login.sock")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "skylight", "login.sock")
}

// listenForCallback removes any stale socket, listens, and waits up to timeout
// for the scheme handler to deliver one callback URL. It always removes the
// socket before returning.
func listenForCallback(timeout time.Duration) (string, error) {
	path := socketPath()
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return "", fmt.Errorf("listen on %s: %w", path, err)
	}
	defer func() {
		ln.Close()
		_ = os.Remove(path)
	}()

	if ul, ok := ln.(*net.UnixListener); ok {
		_ = ul.SetDeadline(time.Now().Add(timeout))
	}
	conn, err := ln.Accept()
	if err != nil {
		return "", fmt.Errorf("waiting for callback: %w", err)
	}
	defer conn.Close()
	data, err := io.ReadAll(conn)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// deliverCallback connects to an in-flight login and sends the callback URL.
// Returns (false, nil) when no login is listening so the caller can fall back.
func deliverCallback(rawURL string) (bool, error) {
	conn, err := net.DialTimeout("unix", socketPath(), 2*time.Second)
	if err != nil {
		return false, nil // no listener -> not delivered, not an error
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(rawURL)); err != nil {
		return false, err
	}
	return true, nil
}

// notifyFallback preserves the original handler behavior when no login runs.
func notifyFallback(rawURL string) {
	if path, err := exec.LookPath("wl-copy"); err == nil {
		c := exec.Command(path)
		c.Stdin = strings.NewReader(rawURL)
		_ = c.Run()
	}
	if path, err := exec.LookPath("notify-send"); err == nil {
		_ = exec.Command(path, "Skylight link received", rawURL).Run()
	}
}
