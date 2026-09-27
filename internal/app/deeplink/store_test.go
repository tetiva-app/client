package deeplink

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

const slug = "petstore-api-k3f9x2qa"

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestStore() (*Store, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)}
	s := NewStore()
	s.now = clock.now
	return s, clock
}

type hookCounts struct{ notify, raise int }

func withHooks(s *Store) *hookCounts {
	c := &hookCounts{}
	s.SetHooks(func() { c.notify++ }, func() { c.raise++ })
	return c
}

func TestOfferAcceptsImportLinks(t *testing.T) {
	cases := map[string]Link{
		"tetiva://import?slug=" + slug:                         {Slug: slug},
		"tetiva-dev://import?slug=" + slug:                     {Slug: slug},
		"tetiva://import/?slug=" + slug:                        {Slug: slug},
		"TETIVA://import?slug=" + slug:                         {Slug: slug},
		"tetiva://import?slug=" + slug + "&token=GJ2WQ4TJNZ5A": {Slug: slug, Token: "GJ2WQ4TJNZ5A"},
		"tetiva://import?slug=" + slug + "&token=":             {Slug: slug},
		"tetiva://import?token=a.b_c-d~e&slug=" + slug:         {Slug: slug, Token: "a.b_c-d~e"},
		"tetiva://import?slug=a-" + strings.Repeat("b", 38) + "-12345678": {
			Slug: "a-" + strings.Repeat("b", 38) + "-12345678",
		},
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			s, _ := newTestStore()

			if !s.Offer(raw) {
				t.Fatalf("Offer(%q) = false, want queued", raw)
			}
			if got := s.Take(); !reflect.DeepEqual(got, []Link{want}) {
				t.Fatalf("Take() = %+v, want [%+v]", got, want)
			}
		})
	}
}

func TestOfferRejectsEverythingElse(t *testing.T) {
	cases := []string{
		"",
		"tetiva://export?slug=" + slug,
		"tetiva://import.evil.com?slug=" + slug,
		"tetiva://user@import?slug=" + slug,
		"tetiva://import:8080?slug=" + slug,
		"tetiva://import/extra?slug=" + slug,
		"tetiva:import?slug=" + slug,
		"tetiva://import",
		"tetiva://import?slug=",
		"tetiva://import?slug=Petstore-API-K3F9X2QA",
		"tetiva://import?slug=petstore-api-k3f9x2q",
		"tetiva://import?slug=petstore_api-k3f9x2qa",
		"tetiva://import?slug=" + strings.Repeat("a", 41) + "-k3f9x2qa",
		"tetiva://import?slug=" + slug + "&token=" + strings.Repeat("t", 513),
		"tetiva://import?slug=" + slug + "&token=bad%20token",
		"tetiva://import?slug=" + slug + "&token=a%0Ab",
		"tetiva://import?slug=%zz",
		"http://import?slug=" + slug,
		"https://share.tetiva.app/" + slug,
		"tetivax://import?slug=" + slug,
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			s, _ := newTestStore()

			if s.Offer(raw) {
				t.Fatalf("Offer(%q) = true, want rejected", raw)
			}
			if got := s.Take(); len(got) != 0 {
				t.Fatalf("Take() = %+v after a rejected link", got)
			}
		})
	}
}

func TestOfferAcceptsMaxLengthToken(t *testing.T) {
	s, _ := newTestStore()
	token := strings.Repeat("t", 512)

	if !s.Offer("tetiva://import?slug=" + slug + "&token=" + token) {
		t.Fatal("a 512-char token was rejected")
	}
}

func TestOfferDeduplicatesWithinTwoSeconds(t *testing.T) {
	s, clock := newTestStore()
	raw := "tetiva://import?slug=" + slug

	if !s.Offer(raw) {
		t.Fatal("first offer rejected")
	}
	s.Take()
	clock.advance(1900 * time.Millisecond)
	if s.Offer(raw) {
		t.Fatal("the same link within 2s was queued again after the window took the first one")
	}
	clock.advance(100 * time.Millisecond)
	if !s.Offer(raw) {
		t.Fatal("the same link after 2s was dropped")
	}
}

func TestOfferDropsALinkStillPending(t *testing.T) {
	s, clock := newTestStore()
	raw := "tetiva://import?slug=" + slug
	s.Deliver([]string{raw})
	clock.advance(2500 * time.Millisecond)

	s.Deliver([]string{raw})
	if s.Offer("tetiva://import/?slug=" + slug) {
		t.Fatal("the same link spelled differently was queued while it is still pending")
	}

	if got := len(s.Take()); got != 1 {
		t.Fatalf("%d confirmations queued for one link clicked twice during a cold start", got)
	}
}

func TestOfferQueuesTheSameSlugWithAnotherToken(t *testing.T) {
	s, _ := newTestStore()
	s.Offer("tetiva://import?slug=" + slug + "&token=first")

	if !s.Offer("tetiva://import?slug=" + slug + "&token=second") {
		t.Fatal("a link with another token was dropped")
	}
	want := []Link{{Slug: slug, Token: "first"}, {Slug: slug, Token: "second"}}
	if got := s.Take(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Take() = %+v, want %+v", got, want)
	}
}

func TestDeliverFindsLinkAnywhereInArgs(t *testing.T) {
	s, _ := newTestStore()
	hooks := withHooks(s)

	ok := s.Deliver([]string{"--inspect", "/tmp/file", "tetiva://import?slug=" + slug})

	if !ok {
		t.Fatal("Deliver = false, want the link in the third argument queued")
	}
	if hooks.notify != 1 || hooks.raise != 1 {
		t.Fatalf("hooks = %+v, want notify and raise once", *hooks)
	}
	if got := s.Take(); !reflect.DeepEqual(got, []Link{{Slug: slug}}) {
		t.Fatalf("Take() = %+v", got)
	}
}

func TestDeliverKeepsArrivalOrder(t *testing.T) {
	s, _ := newTestStore()
	second := "other-collection-a1b2c3d4"

	s.Deliver([]string{"tetiva://import?slug=" + slug, "tetiva-dev://import?slug=" + second})

	want := []Link{{Slug: slug}, {Slug: second}}
	if got := s.Take(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Take() = %+v, want %+v", got, want)
	}
}

func TestDeliverWithoutQueuedLinkCallsNoHooks(t *testing.T) {
	s, _ := newTestStore()
	hooks := withHooks(s)
	raw := "tetiva://import?slug=" + slug
	s.Deliver([]string{raw})
	*hooks = hookCounts{}

	cases := [][]string{nil, {}, {"--flag"}, {"tetiva://import?slug=bad"}, {raw}}
	for _, args := range cases {
		if s.Deliver(args) {
			t.Fatalf("Deliver(%q) = true", args)
		}
	}
	if *hooks != (hookCounts{}) {
		t.Fatalf("hooks = %+v, want none", *hooks)
	}
}

func TestActivateOnlyRaises(t *testing.T) {
	s, _ := newTestStore()
	hooks := withHooks(s)

	s.Activate()

	if *hooks != (hookCounts{raise: 1}) {
		t.Fatalf("hooks = %+v, want a single raise", *hooks)
	}
}

func TestNoHooksDoesNotPanic(t *testing.T) {
	s, _ := newTestStore()

	s.Activate()
	if !s.Deliver([]string{"tetiva://import?slug=" + slug}) {
		t.Fatal("Deliver without hooks must still queue")
	}
	s.SetHooks(nil, nil)
	s.Activate()
}

func TestSetHooksNotifiesAboutEarlierLinks(t *testing.T) {
	s, _ := newTestStore()
	s.Deliver([]string{"tetiva://import?slug=" + slug})

	hooks := withHooks(s)

	if *hooks != (hookCounts{notify: 1}) {
		t.Fatalf("hooks = %+v, want one notify for the pending link", *hooks)
	}
	empty := NewStore()
	if c := withHooks(empty); *c != (hookCounts{}) {
		t.Fatalf("hooks = %+v on an empty store, want none", *c)
	}
}

func TestTakeClearsAndNeverReturnsNil(t *testing.T) {
	s, _ := newTestStore()

	if got := s.Take(); got == nil || len(got) != 0 {
		t.Fatalf("Take() on an empty store = %#v, want an empty non-nil slice", got)
	}
	s.Offer("tetiva://import?slug=" + slug)
	if got := s.Take(); len(got) != 1 {
		t.Fatalf("Take() = %+v, want one link", got)
	}
	if got := s.Take(); got == nil || len(got) != 0 {
		t.Fatalf("second Take() = %#v, want empty", got)
	}
}
