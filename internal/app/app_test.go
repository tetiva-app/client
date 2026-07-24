package app

import (
	"os"
	"testing"

	"go.uber.org/fx"
)

func TestDIGraphValid(t *testing.T) {
	// os.DirFS("../..") roots at the repo root (which contains migrations/),
	// matching main.go's embed; the value is not read during validation.
	if err := fx.ValidateApp(NewApp(os.DirFS("../..")), fx.NopLogger); err != nil {
		t.Fatalf("DI graph invalid: %v", err)
	}
}
