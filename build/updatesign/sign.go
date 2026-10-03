package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/appupdate"
	infraupdate "github.com/tetiva-app/client/internal/infrastructure/appupdate"
)

const s3Base = "https://s3.twcstorage.ru/ccquota/releases/"

type artifactFile struct{ name, os, arch, format string }

func artifactFiles(version string) []artifactFile {
	prefix := "Tetiva-" + version
	return []artifactFile{
		{prefix + "-macos-universal.zip", "darwin", "universal", "zip"},
		{prefix + "-windows-amd64-installer.exe", "windows", "amd64", "nsis"},
		{prefix + "-windows-arm64-installer.exe", "windows", "arm64", "nsis"},
	}
}

func (t tool) sign(args []string) error {
	const funcName = "updatesign.sign"
	flags := flag.NewFlagSet("sign", flag.ContinueOnError)
	flags.SetOutput(t.stderr)
	version := flags.String("version", "", "release version X.Y.Z")
	dir := flags.String("dir", "", "directory with the release artifacts")
	legacy := flags.String("legacy-version", "", "top-level version for clients up to 1.2.0")
	disable := flags.String("disable", "", "comma-separated client versions with in-app install disabled")
	keyFile := flags.String("key-file", "", "base64 private key file instead of the keychain")
	baseURL := flags.String("base-url", s3Base, "URL prefix for the artifact file names")
	out := flags.String("out", "", "manifest file to write")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if *version == "" || *dir == "" || *out == "" {
		return errors.New(usage)
	}

	rel := entities.Release{Version: *version, PublishedAt: time.Now().UTC().Truncate(time.Second)}
	if *disable != "" {
		for _, v := range strings.Split(*disable, ",") {
			v = strings.TrimSpace(v)
			if !appupdate.ValidVersion(v) {
				return fmt.Errorf("%s: --disable: %q is not an x.y.z version", funcName, v)
			}
			rel.InAppDisabled = append(rel.InAppDisabled, v)
		}
	}
	for _, f := range artifactFiles(*version) {
		size, sum, err := hashFile(filepath.Join(*dir, f.name))
		if errors.Is(err, fs.ErrNotExist) {
			_, _ = fmt.Fprintf(t.stderr, "skipped %s: not in %s\n", f.name, *dir)
			continue
		}
		if err != nil {
			return fmt.Errorf("%s: %w", funcName, err)
		}
		rel.Artifacts = append(rel.Artifacts, entities.Artifact{OS: f.os, Arch: f.arch, Format: f.format, URL: *baseURL + f.name, Size: size, SHA256: sum})
	}
	if len(rel.Artifacts) == 0 {
		return fmt.Errorf("%s: no Tetiva-%s artifacts in %s", funcName, *version, *dir)
	}

	key, err := t.privateKey(*keyFile)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	signed, err := infraupdate.Sign(rel, *legacy, []ed25519.PrivateKey{key})
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if err := os.WriteFile(*out, signed, 0o644); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	_, _ = fmt.Fprintf(t.stdout, "wrote %s: version %s, %d artifacts\n", *out, rel.Version, len(rel.Artifacts))
	return nil
}

func (t tool) privateKey(keyFile string) (ed25519.PrivateKey, error) {
	const funcName = "updatesign.privateKey"
	var encoded string
	if keyFile != "" {
		b, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		encoded = string(b)
	} else {
		s, err := t.keys.Load()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		encoded = s
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%s: not a base64 ed25519 private key", funcName)
	}
	return key, nil
}

func hashFile(path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = f.Close() }()
	return hashReader(f)
}

func hashReader(r io.Reader) (int64, string, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return 0, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}
