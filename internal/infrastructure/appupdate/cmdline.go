package appupdate

// NSIS takes everything after /D= as the path, so it goes last and unquoted even with spaces.
func installerCmdLine(installer, installDir string) string {
	return `"` + installer + `" /S /UPDATE /D=` + installDir
}
