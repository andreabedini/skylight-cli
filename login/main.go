package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: skylight-login <login|callback> [args]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "login":
		err = runLogin(os.Args[2:])
	case "callback":
		err = runCallback(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	baseURLFlag := fs.String("base-url", "", "override base URL (default: $SKYLIGHT_BASE_URL or https://app.ourskylight.com)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	baseURL := *baseURLFlag
	if baseURL == "" {
		if env := os.Getenv("SKYLIGHT_BASE_URL"); env != "" {
			baseURL = env
		} else {
			baseURL = "https://app.ourskylight.com"
		}
	}

	verifier, err := GenerateVerifier()
	if err != nil {
		return err
	}
	state, err := GenerateState()
	if err != nil {
		return err
	}
	authURL := buildAuthorizeURL(baseURL, Challenge(verifier), state)

	fmt.Println("Opening browser to:")
	fmt.Println(authURL)
	if err := exec.Command("xdg-open", authURL).Start(); err != nil {
		fmt.Fprintf(os.Stderr, "could not launch browser (%v); open the URL above manually\n", err)
	}

	rawCallback, err := listenForCallback(3 * time.Minute)
	if err != nil {
		return err
	}
	code, err := parseCallback(rawCallback, state)
	if err != nil {
		return err
	}
	tr, err := exchangeCode(http.DefaultClient, baseURL, code, verifier)
	if err != nil {
		return err
	}

	// Checkpoint build: print to confirm the exchange works.
	// Task 5 replaces this with persistTokens.
	fmt.Println("access_token:", tr.AccessToken)
	return nil
}

func runCallback(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: skylight-login callback <url>")
	}
	delivered, err := deliverCallback(args[0])
	if err != nil {
		return err
	}
	if !delivered {
		notifyFallback(args[0])
	}
	return nil
}
