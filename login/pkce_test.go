package main

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestGenerateVerifierLength(t *testing.T) {
	v, err := GenerateVerifier()
	if err != nil {
		t.Fatal(err)
	}
	if len(v) < 43 || len(v) > 128 {
		t.Fatalf("verifier length %d out of RFC 7636 range 43..128", len(v))
	}
}

func TestChallengeMatchesS256(t *testing.T) {
	v := "test-verifier-value"
	sum := sha256.Sum256([]byte(v))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if got := Challenge(v); got != want {
		t.Fatalf("Challenge = %q, want %q", got, want)
	}
}

func TestStateIsRandom(t *testing.T) {
	a, _ := GenerateState()
	b, _ := GenerateState()
	if a == "" || a == b {
		t.Fatalf("expected distinct non-empty states, got %q and %q", a, b)
	}
}
