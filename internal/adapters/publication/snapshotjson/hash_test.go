package snapshotjson_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	"github.com/tetiva-app/client/migrations"
	"github.com/tetiva-app/client/pkg/migrate"
)

func TestContentHash_LeavesOutGeneratorAndLocale(t *testing.T) {
	s, _ := buildSnapshot(t, petstore())
	out, err := snapshotjson.Marshal(s)
	require.NoError(t, err)

	stripped := strings.Replace(string(out), `"generator":"Tetiva 1.2.0","locale":"ru",`, "", 1)
	require.NotEqual(t, string(out), stripped)
	sum := sha256.Sum256([]byte(stripped))

	got, err := snapshotjson.ContentHash(s)
	require.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(sum[:]), got)

	emptied := strings.Replace(string(out), `"generator":"Tetiva 1.2.0","locale":"ru",`, `"generator":"","locale":"",`, 1)
	emptiedSum := sha256.Sum256([]byte(emptied))
	assert.NotEqual(t, hex.EncodeToString(emptiedSum[:]), got)

	s.Generator, s.Locale = "Tetiva 9.9.9", "en"
	again, err := snapshotjson.ContentHash(s)
	require.NoError(t, err)
	assert.Equal(t, got, again, "a new client version or author locale is not a change")
}

func TestMarshal_InputOrderDoesNotMatter(t *testing.T) {
	base := petstore()
	s, _ := buildSnapshot(t, base)
	want, err := snapshotjson.Marshal(s)
	require.NoError(t, err)

	rng := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 20; i++ {
		in := petstore()
		rng.Shuffle(len(in.Collections), func(a, b int) { in.Collections[a], in.Collections[b] = in.Collections[b], in.Collections[a] })
		rng.Shuffle(len(in.Requests), func(a, b int) { in.Requests[a], in.Requests[b] = in.Requests[b], in.Requests[a] })
		rng.Shuffle(len(in.Variables), func(a, b int) { in.Variables[a], in.Variables[b] = in.Variables[b], in.Variables[a] })
		for _, list := range in.Examples {
			rng.Shuffle(len(list), func(a, b int) { list[a], list[b] = list[b], list[a] })
		}
		s, _ := buildSnapshot(t, in)
		got, err := snapshotjson.Marshal(s)
		require.NoError(t, err)
		require.Equal(t, string(want), string(got), "permutation %d", i)
	}
}

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	require.NoError(t, migrate.Run(db, migrations.FS, "."))
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// store writes the fixture in the given order and reads it back the way the publication service will.
func store(t *testing.T, in publication.BuildInput, reverse bool) publication.BuildInput {
	t.Helper()
	ctx := context.Background()
	db := openDB(t)
	collections, requests, variables := slices.Clone(in.Collections), slices.Clone(in.Requests), slices.Clone(in.Variables)
	if reverse {
		slices.Reverse(collections)
		slices.Reverse(requests)
		slices.Reverse(variables)
	}

	collRepo := sqlite.NewCollectionRepo(db)
	for _, c := range collections {
		require.NoError(t, collRepo.Create(ctx, c))
	}
	reqRepo := sqlite.NewRequestRepo(db)
	for _, r := range requests {
		require.NoError(t, reqRepo.Create(ctx, r))
	}
	exRepo := sqlite.NewResponseExampleRepo(db)
	ids := slices.Collect(func(yield func(uuid.UUID) bool) {
		for id := range in.Examples {
			if !yield(id) {
				return
			}
		}
	})
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	if reverse {
		slices.Reverse(ids)
	}
	for _, id := range ids {
		list := slices.Clone(in.Examples[id])
		if reverse {
			slices.Reverse(list)
		}
		for _, e := range list {
			require.NoError(t, exRepo.Create(ctx, e))
		}
	}
	require.NoError(t, sqlite.NewEnvironmentRepo(db).Create(ctx, in.Environment))
	varRepo := sqlite.NewVariableRepo(db)
	for _, v := range variables {
		require.NoError(t, varRepo.Create(ctx, v))
	}

	out := in
	var err error
	out.Collections, err = collRepo.List(ctx, collection.Filter{WorkspaceID: workspaceID})
	require.NoError(t, err)
	out.Requests = nil
	out.Examples = map[uuid.UUID][]*entities.ResponseExample{}
	for _, c := range out.Collections {
		list, err := reqRepo.List(ctx, request.Filter{CollectionID: c.ID})
		require.NoError(t, err)
		out.Requests = append(out.Requests, list...)
		for _, r := range list {
			examples, err := exRepo.ListByRequest(ctx, r.ID)
			require.NoError(t, err)
			out.Examples[r.ID] = examples
		}
		if c.ID == in.Root.ID {
			out.Root = c
		}
	}
	out.Variables, err = varRepo.List(ctx, in.Environment.ID)
	require.NoError(t, err)
	return out
}

func TestContentHash_SameDataInTwoDatabases(t *testing.T) {
	in := petstore()
	for _, c := range in.Collections {
		c.SortOrder = 0
	}
	// Equal sort order and timestamps leave only the id to break ties, which the repositories do not order by.
	for _, r := range in.Requests {
		r.CreatedAt = in.Root.CreatedAt
	}

	a, _ := buildSnapshot(t, store(t, in, false))
	b, _ := buildSnapshot(t, store(t, in, true))

	ha, err := snapshotjson.ContentHash(a)
	require.NoError(t, err)
	hb, err := snapshotjson.ContentHash(b)
	require.NoError(t, err)
	assert.Equal(t, ha, hb)
}
