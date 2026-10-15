package requester

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/tetiva-app/client/internal/localpath"
)

const maxSchemaFileBytes = 32 << 20

var errNetworkSchemaPath = errors.New("schema files on network shares or URLs are not supported; copy the file to this computer and pick the copy")

// Synced paths are read on tab open, and Windows signs in to the host of a UNC or \??\UNC path.
func checkLocalPath(path string) error {
	if localpath.IsRemote(path) {
		return errNetworkSchemaPath
	}
	return nil
}

// Stat before Open: opening a FIFO blocks until a writer appears.
func readSchemaFile(path string) ([]byte, error) {
	const funcName = "readSchemaFile"

	if err := checkLocalPath(path); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %q is not a regular file", funcName, path)
	}
	if info.Size() > maxSchemaFileBytes {
		return nil, fmt.Errorf("%s: %q is larger than %s", funcName, path, limitText(maxSchemaFileBytes))
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, maxSchemaFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if len(data) > maxSchemaFileBytes {
		return nil, fmt.Errorf("%s: %q is larger than %s", funcName, path, limitText(maxSchemaFileBytes))
	}
	return data, nil
}

func protoFileAccessor(path string) (io.ReadCloser, error) {
	data, err := readSchemaFile(path)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
