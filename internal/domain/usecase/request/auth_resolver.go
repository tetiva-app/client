package request

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// ResolvedAuth is the effective auth of a request together with the owner whose
// configuration it came from; the token store keys its rows by that owner.
type ResolvedAuth struct {
	Type  entities.AuthType
	Data  string
	Owner entities.AuthOwner
}

// AuthResolver walks up the collection hierarchy when the request uses inherit.
type AuthResolver interface {
	ResolveAuth(ctx context.Context, req *entities.Request) (ResolvedAuth, error)
	ResolveInherited(ctx context.Context, collectionID uuid.UUID) (ResolvedAuth, error)
}

type authResolver struct {
	collectionReader CollectionReader
}

func NewAuthResolver(cr CollectionReader) AuthResolver {
	return &authResolver{collectionReader: cr}
}

// ResolveAuth loads the owning collection even when the request carries its own auth: it is the only
// source of the workspace the caller is checked against.
func (r *authResolver) ResolveAuth(ctx context.Context, req *entities.Request) (ResolvedAuth, error) {
	const funcName = "authResolver.ResolveAuth"

	coll, err := r.collection(ctx, req.CollectionID)
	if err != nil {
		return ResolvedAuth{}, fmt.Errorf("%s: %w", funcName, err)
	}
	if req.AuthType != entities.AuthTypeInherit {
		return ResolvedAuth{
			Type:  req.AuthType,
			Data:  req.AuthData,
			Owner: entities.AuthOwner{WorkspaceID: coll.WorkspaceID, Kind: entities.AuthOwnerKindRequest, ID: req.ID},
		}, nil
	}
	ra, err := r.walk(ctx, coll, false)
	if err != nil {
		return ResolvedAuth{}, fmt.Errorf("%s: %w", funcName, err)
	}
	return ra, nil
}

func (r *authResolver) ResolveInherited(ctx context.Context, collectionID uuid.UUID) (ResolvedAuth, error) {
	const funcName = "authResolver.ResolveInherited"

	coll, err := r.collection(ctx, collectionID)
	if err != nil {
		return ResolvedAuth{}, fmt.Errorf("%s: %w", funcName, err)
	}
	ra, err := r.walk(ctx, coll, true)
	if err != nil {
		return ResolvedAuth{}, fmt.Errorf("%s: %w", funcName, err)
	}
	return ra, nil
}

func (r *authResolver) collection(ctx context.Context, id uuid.UUID) (*entities.Collection, error) {
	coll, err := r.collectionReader.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if coll == nil {
		return nil, &domain.NotFoundError{Entity: "collection", ID: id.String()}
	}
	return coll, nil
}

// walk counts a skipped start as level 0, so send and preview share the depth limit.
func (r *authResolver) walk(ctx context.Context, coll *entities.Collection, skipStart bool) (ResolvedAuth, error) {
	workspaceID := coll.WorkspaceID
	seen := make(map[uuid.UUID]bool, 4)
	for depth := 0; depth < maxCollectionDepth && !seen[coll.ID]; depth++ {
		seen[coll.ID] = true
		if (depth > 0 || !skipStart) && coll.AuthType != entities.AuthTypeNone {
			return ResolvedAuth{
				Type:  coll.AuthType,
				Data:  coll.AuthData,
				Owner: entities.AuthOwner{WorkspaceID: coll.WorkspaceID, Kind: entities.AuthOwnerKindCollection, ID: coll.ID},
			}, nil
		}
		if coll.ParentID == nil {
			break
		}
		parent, err := r.collectionReader.GetByID(ctx, *coll.ParentID)
		if err != nil {
			return ResolvedAuth{}, err
		}
		// A broken parent link ends the chain; only the start collection must exist.
		if parent == nil {
			break
		}
		coll = parent
	}
	return ResolvedAuth{Type: entities.AuthTypeNone, Data: "{}", Owner: entities.AuthOwner{WorkspaceID: workspaceID}}, nil
}

// resolveAuthFor rejects a request whose workspace differs from the caller's: a stale tab must not run
// against another workspace's variables, cookies and tokens. Runs before variables and scripts.
func (u *usecase) resolveAuthFor(ctx context.Context, req *entities.Request, workspaceID uuid.UUID) (ResolvedAuth, error) {
	ra, err := u.authResolver.ResolveAuth(ctx, req)
	if err != nil {
		return ResolvedAuth{}, err
	}
	if ra.Owner.WorkspaceID != workspaceID {
		return ResolvedAuth{}, &domain.ValidationError{Fields: map[string]string{
			"workspace": "request belongs to another workspace",
		}}
	}
	return ra, nil
}

// rejectNonHTTPAuth guards schemes applied on the final HTTP request; they cannot ride on gRPC or WS.
func rejectNonHTTPAuth(authType entities.AuthType) error {
	switch authType {
	case entities.AuthTypeDigest, entities.AuthTypeAWSSigV4:
		return &domain.ValidationError{Fields: map[string]string{
			"authType": string(authType) + " is supported for HTTP only",
		}}
	}
	return nil
}
