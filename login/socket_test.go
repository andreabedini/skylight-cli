package main

import (
	"testing"
	"time"
)

func TestSocketRelayRoundTrip(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	got := make(chan string, 1)
	errc := make(chan error, 1)
	go func() {
		u, err := listenForCallback(3 * time.Second)
		if err != nil {
			errc <- err
			return
		}
		got <- u
	}()

	// The listener may not have bound yet; retry delivery until it connects.
	deadline := time.Now().Add(2 * time.Second)
	delivered := false
	for time.Now().Before(deadline) {
		ok, err := deliverCallback("skylight-family://welcome?code=ABC&state=S")
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			delivered = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !delivered {
		t.Fatal("callback was never delivered")
	}

	select {
	case u := <-got:
		if u != "skylight-family://welcome?code=ABC&state=S" {
			t.Fatalf("relayed url = %q", u)
		}
	case err := <-errc:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("listener did not return")
	}
}

func TestDeliverCallbackNoListener(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	ok, err := deliverCallback("skylight-family://welcome?code=ABC&state=S")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected ok=false when no listener is present")
	}
}
