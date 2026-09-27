package wails

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/app/deeplink"
)

const deepLinkSlug = "petstore-api-k3f9x2qa"

func TestDeepLinkTakePendingEmptyIsArray(t *testing.T) {
	svc := NewDeepLinkService(deeplink.NewStore())

	res := svc.TakePending()

	if res.Error != nil {
		t.Fatalf("TakePending error: %+v", res.Error)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"data":[]`) {
		t.Fatalf("TakePending JSON = %s, want data to be [] not null", raw)
	}
}

func TestDeepLinkTakePendingDrainsStore(t *testing.T) {
	store := deeplink.NewStore()
	svc := NewDeepLinkService(store)
	store.Deliver([]string{"tetiva://import?slug=" + deepLinkSlug + "&token=ONE"})

	first := svc.TakePending()
	second := svc.TakePending()

	want := []dto.DeepLink{{Slug: deepLinkSlug, Token: "ONE"}}
	if !reflect.DeepEqual(first.Data, want) {
		t.Fatalf("first TakePending = %+v, want %+v", first.Data, want)
	}
	if len(second.Data) != 0 {
		t.Fatalf("second TakePending = %+v, want empty", second.Data)
	}
}

func TestDeepLinkAttachEmitsOnDelivery(t *testing.T) {
	store := deeplink.NewStore()
	svc := NewDeepLinkService(store)
	var events []string
	svc.Attach(nil, func(name string, data any) {
		if data != nil {
			t.Errorf("event %q carried data %v, want none", name, data)
		}
		events = append(events, name)
	})

	store.Deliver([]string{"tetiva://import?slug=" + deepLinkSlug})
	store.Activate()

	if !reflect.DeepEqual(events, []string{DeepLinkReceivedEvent}) {
		t.Fatalf("events = %q, want one %q", events, DeepLinkReceivedEvent)
	}
}
