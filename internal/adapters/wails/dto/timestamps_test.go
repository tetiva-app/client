package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func TestResponses_FormatTimestampsInUTC(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)
	created := time.Date(2026, 10, 10, 1, 30, 0, 0, msk)
	updated := time.Date(2026, 10, 10, 3, 0, 0, 0, msk)
	want := [2]string{"2026-10-09T22:30:00Z", "2026-10-10T00:00:00Z"}

	collection := CollectionToResponse(&entities.Collection{CreatedAt: created, UpdatedAt: updated})
	request := RequestToResponse(&entities.Request{CreatedAt: created, UpdatedAt: updated})
	example := ExampleToResponse(&entities.ResponseExample{CreatedAt: created, UpdatedAt: updated})
	workspace := WorkspaceToResponse(&entities.Workspace{CreatedAt: created, UpdatedAt: updated})
	environment := EnvironmentToResponse(&entities.Environment{CreatedAt: created, UpdatedAt: updated})
	variable := VariableToResponse(&entities.Variable{CreatedAt: created, UpdatedAt: updated})

	assert.Equal(t, want, [2]string{collection.CreatedAt, collection.UpdatedAt}, "collection")
	assert.Equal(t, want, [2]string{request.CreatedAt, request.UpdatedAt}, "request")
	assert.Equal(t, want, [2]string{example.CreatedAt, example.UpdatedAt}, "example")
	assert.Equal(t, want, [2]string{workspace.CreatedAt, workspace.UpdatedAt}, "workspace")
	assert.Equal(t, want, [2]string{environment.CreatedAt, environment.UpdatedAt}, "environment")
	assert.Equal(t, want, [2]string{variable.CreatedAt, variable.UpdatedAt}, "variable")
}
