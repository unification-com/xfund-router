package main

import (
	"os"
	"path/filepath"

	"go-ooo/cmd"
)

var DefaultHome string

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	DefaultHome = filepath.Join(userHomeDir, ".go-ooo")
}

func main() {
	cmd.Execute(DefaultHome)
}
