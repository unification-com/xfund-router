package server

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// promptPassphrase reads the keystore passphrase from the terminal without echo.
func promptPassphrase() (string, error) {
	fmt.Print("Enter your keystore passphrase: ")
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println("")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// getPasswordFromFileOrFlag treats flagValue as a path to a password file if it
// names a readable file, otherwise as the password itself.
func getPasswordFromFileOrFlag(flagValue string) string {
	if data, err := os.ReadFile(flagValue); err == nil {
		return strings.TrimSpace(string(data))
	}
	return strings.TrimSpace(flagValue)
}
