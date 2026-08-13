// Package config contains configuration values and defaults used by the extension.
package config

import (
	"os"
	"strconv"
	"time"
)

const (
	Version = "1.0.0"

	OPTypeCredit       = "CREDIT"
	OPCommandOpen      = "OPEN"
	OPCommandQuote     = "QUOTE"
	OPCommandFinalize  = "FINALIZE"
	TimeoutShutdown    = 5 * time.Second
	TimeoutNodeRequest = 10 * time.Second

	// MaxEncryptedMessageBytes bounds both encrypted input and decrypted JSON.
	// Loan requests are tiny; the generous limit primarily protects the TEE from
	// accidental or adversarial unbounded allocations.
	MaxEncryptedMessageBytes = 64 * 1024
)

// Defaults.
var (
	ExtensionPort = 8080
	SignPort      = 9090
)

// Environment variables override defaults.
func init() {
	ep := os.Getenv("EXTENSION_PORT")
	sp := os.Getenv("SIGN_PORT")

	if ep != "" {
		if v, err := strconv.Atoi(ep); err == nil {
			ExtensionPort = v
		}
	}
	if sp != "" {
		if v, err := strconv.Atoi(sp); err == nil {
			SignPort = v
		}
	}
}
