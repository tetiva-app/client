package app

import (
	"testing"

	"go.uber.org/fx"

	"github.com/tetiva-app/client/migrations"
)

func TestDIGraphValid(t *testing.T) {
	// Same FS main.go passes; it is not read during validation.
	if err := fx.ValidateApp(NewApp(migrations.FS), fx.NopLogger); err != nil {
		t.Fatalf("DI graph invalid: %v", err)
	}
}
