package constants

const (
	AppName    = "Tetiva"
	AppVersion = "1.3.2"
	DBFileName = "data.db"
	// LegacyAppDir is the pre-rebrand data directory (GopherCourier).
	// ResolveDataDir migrates it to AppDir on first start after the rename.
	LegacyAppDir = ".gophercourier"

	FlavorStable  = "stable"
	FlavorPreview = "preview"
)

// Flavor is set at link time: -ldflags "-X .../internal/constants.Flavor=preview".
var Flavor = FlavorStable

// IsPreview reports the side-by-side build: own data and keychain, no account or updates.
func IsPreview() bool {
	return Flavor == FlavorPreview
}

// ProductName also yields the Linux GtkApplication id; a shared id merges both flavors.
func ProductName() string {
	if IsPreview() {
		return AppName + " Preview"
	}
	return AppName
}

func AppDir() string {
	if IsPreview() {
		return ".tetiva-preview"
	}
	return ".tetiva"
}

// AppTitle is the window title shown in the OS title bar. Includes the
// current version so the running build is always recognizable at a glance.
func AppTitle() string {
	return ProductName() + " " + AppVersion
}
