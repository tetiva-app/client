package mcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func TestFormatTimestamp_ConvertsToUTC(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)

	assert.Equal(t, "2026-10-09T22:30:00Z", formatTimestamp(time.Date(2026, 10, 10, 1, 30, 0, 0, msk)))
}

func TestSerialize_TimestampsInUTC(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)
	created := time.Date(2026, 10, 10, 1, 30, 0, 0, msk)
	updated := time.Date(2026, 10, 10, 3, 0, 0, 0, msk)

	collection := serializeCollection(&entities.Collection{CreatedAt: created, UpdatedAt: updated})
	request := serializeRequest(&entities.Request{CreatedAt: created, UpdatedAt: updated})

	assert.Equal(t, "2026-10-09T22:30:00Z", collection["created_at"])
	assert.Equal(t, "2026-10-10T00:00:00Z", collection["updated_at"])
	assert.Equal(t, "2026-10-09T22:30:00Z", request["created_at"])
	assert.Equal(t, "2026-10-10T00:00:00Z", request["updated_at"])
}
