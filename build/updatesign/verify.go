package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/tetiva-app/client/internal/domain/entities"
	infraupdate "github.com/tetiva-app/client/internal/infrastructure/appupdate"
)

func (t tool) verify(args []string) error {
	const funcName = "updatesign.verify"
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(t.stderr)
	manifest := flags.String("manifest", "", "signed manifest file")
	files := flags.String("files", "", "directory with the artifacts to re-hash")
	remote := flags.Bool("remote", false, "download every artifact URL and check it")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if *manifest == "" {
		return errors.New(usage)
	}

	raw, err := os.ReadFile(*manifest)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	rel, err := infraupdate.Codec{Keys: t.publicKeys}.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	listed, err := listedArtifacts(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if listed != len(rel.Artifacts) {
		return fmt.Errorf("%s: the payload lists %d artifacts, the app accepts %d", funcName, listed, len(rel.Artifacts))
	}
	_, _ = fmt.Fprintf(t.stdout, "%s: version %s, %d artifacts\n", *manifest, rel.Version, len(rel.Artifacts))
	for _, a := range rel.Artifacts {
		if *files != "" {
			size, sum, err := hashFile(filepath.Join(*files, path.Base(a.URL)))
			if err != nil {
				return fmt.Errorf("%s: %w", funcName, err)
			}
			if err := matches(a, size, sum); err != nil {
				return fmt.Errorf("%s: %w", funcName, err)
			}
		}
		if *remote {
			if err := t.matchesRemote(a); err != nil {
				return fmt.Errorf("%s: %w", funcName, err)
			}
		}
		_, _ = fmt.Fprintf(t.stdout, "  %s/%s/%s %d %s\n", a.OS, a.Arch, a.Format, a.Size, a.URL)
	}
	return nil
}

func listedArtifacts(raw []byte) (int, error) {
	var m struct{ Payload string }
	if err := json.Unmarshal(raw, &m); err != nil {
		return 0, err
	}
	payload, err := base64.StdEncoding.DecodeString(m.Payload)
	if err != nil {
		return 0, err
	}
	var p struct{ Artifacts []json.RawMessage }
	if err := json.Unmarshal(payload, &p); err != nil {
		return 0, err
	}
	return len(p.Artifacts), nil
}

func (t tool) matchesRemote(a entities.Artifact) error {
	resp, err := t.client.Get(a.URL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", a.URL, resp.Status)
	}
	size, sum, err := hashReader(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: %w", a.URL, err)
	}
	return matches(a, size, sum)
}

func matches(a entities.Artifact, size int64, sum string) error {
	if size != a.Size || sum != a.SHA256 {
		return fmt.Errorf("%s: %d bytes, sha256 %s; signed %d bytes, sha256 %s", path.Base(a.URL), size, sum, a.Size, a.SHA256)
	}
	return nil
}
