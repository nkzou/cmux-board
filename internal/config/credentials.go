package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kevin-zou/cmux-board/internal/secretsink"
)

// Credentials stores adapter-keyed API tokens.
// Stored at ~/.config/cmux-board/credentials.json with mode 0600.
type Credentials struct {
	SchemaVersion int                     `json:"schema_version"`
	Adapters      map[string]AdapterCreds `json:"adapters,omitempty"`
}

// AdapterCreds holds credentials for one adapter.
type AdapterCreds struct {
	Email    string `json:"email,omitempty"`
	APIToken string `json:"api_token,omitempty"`
	SiteURL  string `json:"site_url,omitempty"`
}

// LoadCredentials reads credentials.json and registers the Jira API token for redaction.
// MUST be called before any logging or HTTP calls.
func LoadCredentials(path string) (Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to read credentials: %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return Credentials{}, fmt.Errorf("failed to parse credentials: %w", err)
	}
	// Register raw API token for redaction BEFORE any logging
	if jira, ok := creds.Adapters["jira"]; ok {
		secretsink.Register(jira.APIToken)
	}
	return creds, nil
}

// SaveCredentials writes creds to path atomically with mode 0600.
func SaveCredentials(path string, creds Credentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}
	return WriteFileAtomic(path, data, 0600)
}
