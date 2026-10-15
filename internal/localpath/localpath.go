package localpath

import "regexp"

var (
	urlScheme     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]+://`)
	extendedDrive = regexp.MustCompile(`^\\\\\?\\[A-Za-z]:[\\/]`)
	devicePrefix  = regexp.MustCompile(`^[\\/]([\\/]|\?\?[\\/])`)
)

// IsRemote reports URLs and UNC or NT device paths, except \\?\C:\ drive paths.
func IsRemote(path string) bool {
	if extendedDrive.MatchString(path) {
		return false
	}
	return devicePrefix.MatchString(path) || urlScheme.MatchString(path)
}
