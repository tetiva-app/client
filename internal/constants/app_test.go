package constants_test

import (
	"testing"

	"github.com/tetiva-app/client/internal/constants"
	"github.com/tetiva-app/client/internal/constants/flavortest"
)

func TestFlavor_StableIsTheDefault(t *testing.T) {
	if constants.IsPreview() {
		t.Fatal("a build without the ldflag must be the regular app")
	}
	if got := constants.ProductName(); got != "Tetiva" {
		t.Errorf("ProductName = %q", got)
	}
	if got := constants.AppDir(); got != ".tetiva" {
		t.Errorf("AppDir = %q", got)
	}
	if got := constants.AppTitle(); got != "Tetiva "+constants.AppVersion {
		t.Errorf("AppTitle = %q", got)
	}
}

func TestFlavor_Preview(t *testing.T) {
	flavortest.Preview(t)

	if got := constants.ProductName(); got != "Tetiva Preview" {
		t.Errorf("ProductName = %q", got)
	}
	if got := constants.AppDir(); got != ".tetiva-preview" {
		t.Errorf("AppDir = %q", got)
	}
	if got := constants.AppTitle(); got != "Tetiva Preview "+constants.AppVersion {
		t.Errorf("AppTitle = %q", got)
	}
}

func TestFlavor_UnknownValueIsStable(t *testing.T) {
	prev := constants.Flavor
	constants.Flavor = "Preview"
	t.Cleanup(func() { constants.Flavor = prev })

	if constants.IsPreview() {
		t.Fatal("only the exact value switches to the preview")
	}
}
