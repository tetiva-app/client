package request

import (
	"errors"
	"fmt"
	"os"

	"github.com/tetiva-app/client/internal/localpath"
)

var errNetworkBodyFile = errors.New("body files on network shares or URLs are not supported; copy the file to this computer and pick the copy")

// Stat before Open: opening a FIFO blocks until a writer appears.
func openBodyFile(path string) (*os.File, error) {
	const funcName = "request.openBodyFile"

	if _, err := statBodyFile(path); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return f, nil
}

func statBodyFile(path string) (os.FileInfo, error) {
	const funcName = "request.statBodyFile"

	if localpath.IsRemote(path) {
		return nil, fmt.Errorf("%s: %w", funcName, errNetworkBodyFile)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %q is not a regular file", funcName, path)
	}
	return info, nil
}
