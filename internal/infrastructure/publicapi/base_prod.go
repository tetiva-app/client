//go:build production

package publicapi

func DefaultBaseURL() string {
	return productionBaseURL
}
