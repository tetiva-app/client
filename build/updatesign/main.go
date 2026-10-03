package main

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	infraupdate "github.com/tetiva-app/client/internal/infrastructure/appupdate"
)

const usage = `usage:
  updatesign keygen
  updatesign sign --version X.Y.Z --dir <dir> [--legacy-version V] [--disable 1.2.1,...] [--key-file <path>] [--base-url <url>] --out <file>
  updatesign verify --manifest <file> [--files <dir>] [--remote]
  updatesign requirement`

type tool struct {
	keys       keyStore
	publicKeys []ed25519.PublicKey
	client     *http.Client
	stdout     io.Writer
	stderr     io.Writer
}

func main() {
	t := tool{
		keys:       keychain{},
		publicKeys: infraupdate.ReleaseKeys(),
		client:     http.DefaultClient,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
	}
	if err := t.run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (t tool) run(args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "keygen":
		return t.keygen()
	case "sign":
		return t.sign(args[1:])
	case "verify":
		return t.verify(args[1:])
	case "requirement":
		_, _ = fmt.Fprintln(t.stdout, infraupdate.CodeRequirement)
		return nil
	}
	return errors.New(usage)
}
