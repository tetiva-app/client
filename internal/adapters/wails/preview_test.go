package wails

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/constants/flavortest"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

func TestContract_Err_PreviewBuild(t *testing.T) {
	b, err := json.Marshal(Err[int](fmt.Errorf("connect: %w", ErrPreviewBuild)))
	require.NoError(t, err)
	require.Equal(t, `{"data":0,"error":{"code":"preview_build","message":"connect: preview build: sign-in, sync and publishing are unavailable"}}`, string(b))
}

func TestBuildService_ReportsTheFlavor(t *testing.T) {
	assert.Equal(t, "stable", NewBuildService().Info().Data.Flavor)

	flavortest.Preview(t)
	assert.Equal(t, "preview", NewBuildService().Info().Data.Flavor)
}

func TestSyncService_PreviewRefusesTheAccount(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	dials := 0
	f.svc.newClient = func(string) (*syncsvc.GRPCClient, error) {
		dials++
		return f.client, nil
	}
	flavortest.Preview(t)
	ws := seededWorkspaceID

	refusals := map[string]*ResultError{
		"Connect":  f.svc.Connect(dto.ConnectRequest{ServerURL: testServerURL, Email: "a@b.c", Password: "pw"}).Error,
		"Register": f.svc.Register(dto.RegisterRequest{ServerURL: testServerURL, Email: "a@b.c", Password: "pw"}).Error,
		"StartBrowserSignIn": f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
			FlowID: uuid.NewString(), ServerURL: testServerURL,
		}).Error,
		"GetServerCapabilities": f.svc.GetServerCapabilities(dto.ServerCapabilitiesRequest{ServerURL: testServerURL}).Error,
		"GetMe":                 f.svc.GetMe().Error,
		"ResendVerification":    f.svc.ResendVerification().Error,
		"ListSessions":          f.svc.ListSessions().Error,
		"LogoutAll":             f.svc.LogoutAll().Error,
		"ListRemoteWorkspaces":  f.svc.ListRemoteWorkspaces().Error,
		"CreateRemoteWorkspace": f.svc.CreateRemoteWorkspace(dto.CreateWorkspaceRequest{Name: "Cloud"}).Error,
		"LinkWorkspace": f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{
			LocalWorkspaceID: ws, RemoteWorkspaceID: "remote-1",
		}).Error,
	}
	for name, e := range refusals {
		if assert.NotNil(t, e, name) {
			assert.Equal(t, ErrCodePreviewBuild, e.Code, name)
		}
	}

	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))
	assert.Zero(t, dials, "a preview never dials the sync server")
	assert.Zero(t, f.signIn.startCalls)
	assert.Nil(t, f.svc.client())
	assert.False(t, f.engine.IsEnabledForWorkspace(ws))
}

func TestPublicationService_PreviewRefusesToPublish(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "Petstore", nil)
	publishReq := h.publishRequest(t, previewRequestFor(root))
	flavortest.Preview(t)
	id := root.ID.String()

	refusals := map[string]*ResultError{
		"Status":    h.svc.Status(dto.PublicationStatusRequest{CollectionID: id}).Error,
		"Plan":      h.svc.Plan(dto.PublishPlanRequest{CollectionID: id}).Error,
		"Publish":   h.svc.Publish(publishReq).Error,
		"Unpublish": h.svc.Unpublish(dto.UnpublishRequest{CollectionID: id}).Error,
		"List":      h.svc.List(dto.PublicationListRequest{WorkspaceID: h.localWS.String(), Remote: true}).Error,
	}
	for name, e := range refusals {
		if assert.NotNil(t, e, name) {
			assert.Equal(t, ErrCodePreviewBuild, e.Code, name)
		}
	}
	h.svc.ProcessPending(context.Background())

	h.remote.mu.Lock()
	defer h.remote.mu.Unlock()
	assert.Empty(t, h.remote.publishes)
	assert.Empty(t, h.remote.unpublishes)
	assert.Empty(t, h.remote.gets)
	assert.Empty(t, h.remote.planAsks)
	assert.Zero(t, h.remote.supportAsks)
}
