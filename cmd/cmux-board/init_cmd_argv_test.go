package main

import (
	"testing"
)

func TestInitCmd_NoAPITokenFlag(t *testing.T) {
	// SECURITY: --api-token must NOT exist on initCmd.
	// This is the static in-process equivalent of the ps auxww check.
	flag := initCmd.Flags().Lookup("api-token")
	if flag != nil {
		t.Errorf("--api-token flag is registered on initCmd; this is a security violation: "+
			"token must never appear in argv. Got: %v", flag)
	}
}

func TestInitCmd_NoAPITokenStdinFlag(t *testing.T) {
	// acli owns auth; --api-token-stdin is no longer needed.
	flag := initCmd.Flags().Lookup("api-token-stdin")
	if flag != nil {
		t.Errorf("--api-token-stdin flag should be absent (acli owns auth), but it is registered: %v", flag)
	}
}

func TestInitCmd_NoEmailFlag(t *testing.T) {
	// Email is resolved from acli auth status; --email flag no longer needed.
	flag := initCmd.Flags().Lookup("email")
	if flag != nil {
		t.Errorf("--email flag should be absent (resolved from acli), but it is registered: %v", flag)
	}
}
