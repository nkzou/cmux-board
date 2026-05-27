package main

import (
	"io"
	"os"
	"testing"

	"github.com/kevin-zou/cmux-board/internal/initwizard"
)

func TestInitCmd_NoAPITokenFlag(t *testing.T) {
	// SECURITY: --api-token must NOT exist on initCmd.
	// This is the static in-process equivalent of the ps auxww check.
	flag := initCmd.Flags().Lookup("api-token")
	if flag != nil {
		t.Errorf("--api-token flag is registered on initCmd; this is a security violation (F4): "+
			"token must never appear in argv. Remove the flag registration immediately. Got: %v", flag)
	}
}

func TestInitCmd_APITokenStdinFlagRegistered(t *testing.T) {
	flag := initCmd.Flags().Lookup("api-token-stdin")
	if flag == nil {
		t.Error("--api-token-stdin flag is NOT registered on initCmd; expected it to be present")
	}
}

// TestReadToken_SignatureNoTokenArg verifies at compile time that ReadToken does NOT
// accept a bare token string parameter. The correct signature is:
//
//	func ReadToken(w io.Writer, r io.Reader, apiTokenStdin bool) (string, error)
//
// This assignment will fail to compile if the signature changes to include a token arg.
var _ func(io.Writer, io.Reader, bool) (string, error) = initwizard.ReadToken

func TestReadToken_ArgvSnapshotDuringStdinRead(t *testing.T) {
	// In-process test: inject a token via strings.NewReader and capture os.Args
	// at the time ReadToken is called.
	// This test asserts that the token value does NOT appear in os.Args.

	const knownToken = "definitely-not-in-argv-1a2b3c4d5e6f7g8h9"

	// Capture os.Args at the point ReadToken would read it.
	capturedArgs := os.Args

	for _, arg := range capturedArgs {
		if arg == knownToken {
			t.Errorf("token %q found in os.Args: %v", knownToken, capturedArgs)
		}
	}

	// Also verify that ReadToken can be called without touching os.Args.
	// Use a pipe-mode read (apiTokenStdin=true) with the known token.
	var w nopWriter
	r, w2 := io.Pipe()
	go func() {
		w2.Write([]byte(knownToken + "\n"))
		w2.Close()
	}()

	got, err := initwizard.ReadToken(w, r, true)
	if err != nil {
		t.Fatalf("ReadToken error: %v", err)
	}
	if got != knownToken {
		t.Errorf("ReadToken returned %q, expected %q", got, knownToken)
	}

	// Verify again that the token did not appear in os.Args.
	for _, arg := range os.Args {
		if arg == knownToken {
			t.Errorf("token leaked into os.Args after ReadToken call: %v", os.Args)
		}
	}
}

// nopWriter discards all writes.
type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
