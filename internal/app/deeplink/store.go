// Package deeplink queues tetiva:// import links until the main window takes them.
package deeplink

import (
	"errors"
	"log/slog"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

const dedupWindow = 2 * time.Second

var (
	slugPattern = regexp.MustCompile(`^[a-z0-9-]{1,40}-[a-z0-9]{8}$`)
	// The token goes into an Authorization header later, so only URL-unreserved characters pass.
	tokenPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{1,512}$`)
)

// Link is an accepted import link; Token is the optional one-time import token.
type Link struct {
	Slug  string
	Token string
}

// Store is the one delivery point for links from IPC, macOS URL events and cold-start argv.
type Store struct {
	mu      sync.Mutex
	pending []Link
	recent  map[string]time.Time
	notify  func()
	raise   func()
	now     func() time.Time
}

func NewStore() *Store {
	return &Store{recent: map[string]time.Time{}, now: time.Now}
}

// Offer queues a valid import link. The same rawURL within 2 s (a double click) is dropped, and so is
// a link still waiting in the queue: a slow cold start outlasts the 2 s.
func (s *Store) Offer(rawURL string) bool {
	link, err := parseLink(rawURL)
	if err != nil {
		// The URL itself stays out of the log: it may carry a one-time token.
		slog.Warn("deeplink: link ignored", "reason", err.Error())
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for u, at := range s.recent {
		if now.Sub(at) >= dedupWindow {
			delete(s.recent, u)
		}
	}
	if _, dup := s.recent[rawURL]; dup || slices.Contains(s.pending, link) {
		return false
	}
	s.recent[rawURL] = now
	s.pending = append(s.pending, link)
	return true
}

// Deliver offers every argument with a tetiva scheme, wherever it sits in args, and
// notifies the window and raises it when at least one link was queued.
func (s *Store) Deliver(args []string) bool {
	queued := false
	for _, arg := range args {
		if hasLinkScheme(arg) && s.Offer(arg) {
			queued = true
		}
	}
	if !queued {
		return false
	}
	notify, raise := s.hooks()
	if notify != nil {
		notify()
	}
	if raise != nil {
		raise()
	}
	return true
}

// Activate brings the window up for a repeated launch that carried no link.
func (s *Store) Activate() {
	if _, raise := s.hooks(); raise != nil {
		raise()
	}
}

// Take returns pending links in arrival order and clears the queue.
func (s *Store) Take() []Link {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.pending
	s.pending = nil
	if out == nil {
		out = []Link{}
	}
	return out
}

// SetHooks wires the window once it exists; hooks run outside the lock and must not block.
func (s *Store) SetHooks(notify, raise func()) {
	s.mu.Lock()
	s.notify, s.raise = notify, raise
	hasPending := len(s.pending) > 0
	s.mu.Unlock()

	if hasPending && notify != nil {
		notify()
	}
}

func (s *Store) hooks() (notify, raise func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.notify, s.raise
}

func hasLinkScheme(arg string) bool {
	scheme, _, ok := strings.Cut(arg, ":")
	if !ok {
		return false
	}
	scheme = strings.ToLower(scheme)
	return scheme == "tetiva" || scheme == "tetiva-dev"
}

func parseLink(rawURL string) (Link, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Link{}, errors.New("unparsable URL")
	}
	if u.Scheme != "tetiva" && u.Scheme != "tetiva-dev" {
		return Link{}, errors.New("unknown scheme")
	}
	if u.Opaque != "" || u.User != nil || !strings.EqualFold(u.Host, "import") || (u.Path != "" && u.Path != "/") {
		return Link{}, errors.New("not an import link")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return Link{}, errors.New("malformed query")
	}
	link := Link{Slug: q.Get("slug"), Token: q.Get("token")}
	if !slugPattern.MatchString(link.Slug) {
		return Link{}, errors.New("invalid slug")
	}
	if link.Token != "" && !tokenPattern.MatchString(link.Token) {
		return Link{}, errors.New("invalid token")
	}
	return link, nil
}
