package entities

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func requestNames(list []*Request) []string {
	names := make([]string, len(list))
	for i, r := range list {
		names[i] = r.Name
	}
	return names
}

func TestSortRequests_ByNameIgnoringCaseWithNumbersByValue(t *testing.T) {
	at := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	list := []*Request{
		{ID: uuid.New(), Name: "users", CreatedAt: at},
		{ID: uuid.New(), Name: "Request 10", CreatedAt: at},
		{ID: uuid.New(), Name: "Auth", CreatedAt: at},
		{ID: uuid.New(), Name: "Request 2", CreatedAt: at},
		{ID: uuid.New(), Name: "Get users (copy)", CreatedAt: at},
		{ID: uuid.New(), Name: "Get users", CreatedAt: at},
		{ID: uuid.New(), Name: "Заказ (копия)", CreatedAt: at},
		{ID: uuid.New(), Name: "Заказ", CreatedAt: at},
	}

	SortRequests(list)

	assert.Equal(t, []string{"Auth", "Get users", "Get users (copy)", "Request 2", "Request 10", "users", "Заказ", "Заказ (копия)"}, requestNames(list))
}

func TestSortCollections_BreaksNameTiesByCreationThenID(t *testing.T) {
	at := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	lowID := uuid.MustParse("00000000-0000-4000-a000-000000000001")
	highID := uuid.MustParse("00000000-0000-4000-a000-000000000002")
	list := []*Collection{
		{ID: lowID, Name: "copy", CreatedAt: at.Add(time.Second)},
		{ID: highID, Name: "Copy", CreatedAt: at},
		{ID: uuid.MustParse("00000000-0000-4000-a000-000000000000"), Name: "COPY", CreatedAt: at.Add(time.Second)},
	}

	SortCollections(list)

	assert.Equal(t, []uuid.UUID{highID, uuid.MustParse("00000000-0000-4000-a000-000000000000"), lowID}, []uuid.UUID{list[0].ID, list[1].ID, list[2].ID})
}
