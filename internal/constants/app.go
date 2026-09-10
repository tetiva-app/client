package constants

const (
	AppName    = "Tetiva"
	AppVersion = "1.1.0"
	DBFileName = "data.db"
	AppDir     = ".tetiva"
	// LegacyAppDir is the pre-rebrand data directory (GopherCourier).
	// NewDB migrates it to AppDir on first start after the rename.
	LegacyAppDir = ".gophercourier"
)

// AppTitle is the window title shown in the OS title bar. Includes the
// current version so the running build is always recognizable at a glance.
func AppTitle() string {
	return AppName + " " + AppVersion
}
