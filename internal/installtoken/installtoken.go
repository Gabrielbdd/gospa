// Package installtoken loads and validates the bootstrap secret that
// gates the public /install handler.
//
// The /install RPCs are intentionally unauthenticated (no user exists
// yet), so without this token any reachable Gospa instance can be
// hijacked into provisioning against the operator's ZITADEL by anyone
// who hits the URL before install completes. The token is the operator
// proof-of-control: paste it into the wizard, the handler accepts the
// request; otherwise it returns Unauthenticated.
//
// Resolution order at startup:
//  1. GOSPA_INSTALL_TOKEN — literal value
//  2. GOSPA_INSTALL_TOKEN_FILE — path to a file with the token
//  3. neither set: generate a 32-hex-char token in-process and log it
//     loudly so a "try-it-out" container deploy still has a usable
//     bootstrap path. Token is not persisted; a process restart
//     generates a new one until the operator wires the env or file.
package installtoken

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

const (
	// EnvLiteral is read first when present.
	EnvLiteral = "GOSPA_INSTALL_TOKEN"
	// EnvFile is read when EnvLiteral is unset; the value is a path.
	EnvFile = "GOSPA_INSTALL_TOKEN_FILE"
)

// Source describes how the token was resolved. Used by the caller to
// decide whether to log a "generated, please persist" warning.
type Source int

const (
	SourceUnknown Source = iota
	SourceLiteralEnv
	SourceFile
	SourceGenerated
)

// Load resolves the install token. Returns the token, the source it
// came from, and an error if a configured path is unreadable or empty.
// When neither env nor file is configured, generates a token and
// returns SourceGenerated with no error — the caller is expected to
// log it.
func Load() (string, Source, error) {
	if v := strings.TrimSpace(os.Getenv(EnvLiteral)); v != "" {
		return v, SourceLiteralEnv, nil
	}

	if path := os.Getenv(EnvFile); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", SourceUnknown, fmt.Errorf(
				"reading install token at %q: %w (set %s to a literal value, or run `mise run infra` locally)",
				path, err, EnvLiteral,
			)
		}
		v := strings.TrimSpace(string(raw))
		if v == "" {
			return "", SourceUnknown, fmt.Errorf("install token file %q is empty", path)
		}
		return v, SourceFile, nil
	}

	gen, err := Generate()
	if err != nil {
		return "", SourceUnknown, fmt.Errorf("generating install token: %w", err)
	}
	return gen, SourceGenerated, nil
}

// Generate returns a random 32-hex-character token (16 random bytes).
func Generate() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// Equal compares two tokens in constant time. Returns false when
// either side is empty so a missing header never matches a missing
// expected token.
func Equal(provided, expected string) bool {
	if provided == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
