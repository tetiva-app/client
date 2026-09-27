//go:build !production

package publicapi

import "os"

// DefaultBaseURL lets dev builds point at a local server; release builds cannot.
func DefaultBaseURL() string {
	if v := os.Getenv("TETIVA_PUBLIC_API"); v != "" {
		return v
	}
	return productionBaseURL
}
