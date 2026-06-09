package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"syscall"

	"go-ooo/auth"
	"go-ooo/config"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/term"
)

// issueAdminToken generates a new admin HTTP bearer, persists its bcrypt hash in
// the sidecar next to the keystore, and returns the raw token for one-time display
// to the operator. Shared by `keystore migrate`, `keystore set-admin-token` and init.
func issueAdminToken(keystoreFile string) (string, error) {
	raw, hash, err := auth.GenerateAdminToken()
	if err != nil {
		return "", err
	}
	if err := auth.WriteHashFile(keystoreFile, hash); err != nil {
		return "", err
	}
	return raw, nil
}

// parsePositiveUint parses a positive uint64 CLI argument (a fee or amount). It rejects
// non-numeric, negative and zero input — all of which the on-chain calls reject anyway,
// so without this a typo previously became an on-chain fee/amount of 0.
func parsePositiveUint(arg, label string) (uint64, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(arg), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a positive whole number, got %q", label, arg)
	}
	if v == 0 {
		return 0, fmt.Errorf("%s must be greater than zero", label)
	}
	return v, nil
}

// requireHexAddress validates that arg is a 0x-prefixed Ethereum address before it is
// sent to a chain operation (e.g. a withdraw recipient).
func requireHexAddress(arg, label string) (string, error) {
	a := strings.TrimSpace(arg)
	if !common.IsHexAddress(a) {
		return "", fmt.Errorf("%s must be a valid 0x Ethereum address, got %q", label, arg)
	}
	return a, nil
}

// readSecret prompts on stdout and reads a line from stdin without echoing it back
// to the terminal, returning the trimmed value. Used for passwords and passphrases.
func readSecret(prompt string) (string, error) {
	fmt.Print(prompt)
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println("")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// postAuthedTask prompts for the admin password and POSTs the JSON payload to the given
// path on the local service with the bearer auth header, returning the response status +
// body. Shared by the admin and analytics senders. It checks the NewRequest and Do errors
// before touching the response, fixing the previous nil-pointer panic when either failed.
func postAuthedTask(cfg *config.Config, path string, payload any) (int, []byte, error) {
	pass, err := readSecret("Enter your password:\t")
	if err != nil {
		return 0, nil, err
	}

	requestJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, fmt.Errorf("can't marshal request: %w", err)
	}

	url := fmt.Sprintf("http://%s:%s%s", cfg.Serve.Host, cfg.Serve.Port, path)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestJSON))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Add("Authorization", "Bearer "+pass)

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}
