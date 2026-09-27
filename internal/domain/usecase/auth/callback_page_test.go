package auth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type callbackPageCase struct {
	name       string
	kind       pageKind
	oauthError string
	headings   map[string]string
}

var callbackPageCases = []callbackPageCase{
	{"signed in", pageSignedIn, "", map[string]string{
		"en": "You’re signed in to Tetiva", "ru": "Вы вошли в\u00a0Tetiva",
	}},
	{"authorized", pageAuthorized, "", map[string]string{
		"en": "Authorization received", "ru": "Авторизация получена",
	}},
	{"access_denied", pageOAuthError, "access_denied", map[string]string{
		"en": "Access denied", "ru": "Доступ не\u00a0выдан",
	}},
	{"invalid_scope", pageOAuthError, "invalid_scope", map[string]string{
		"en": "The provider rejected the scope", "ru": "Провайдер не\u00a0принял scope",
	}},
	{"server_error", pageOAuthError, "server_error", map[string]string{
		"en": "The provider ran into an error", "ru": "Сбой на\u00a0стороне провайдера",
	}},
	{"temporarily_unavailable", pageOAuthError, "temporarily_unavailable", map[string]string{
		"en": "The provider is temporarily unavailable", "ru": "Провайдер временно недоступен",
	}},
	{"other error", pageOAuthError, "invalid_request", map[string]string{
		"en": "The provider returned an error", "ru": "Провайдер вернул ошибку",
	}},
	{"sign-in already handled", pageSignInAlreadyHandled, "", map[string]string{
		"en": "This link has already been used", "ru": "Эта ссылка уже использована",
	}},
	{"authorization already handled", pageOAuthAlreadyHandled, "", map[string]string{
		"en": "This authorization link was already handled", "ru": "Эта ссылка авторизации уже обработана",
	}},
}

const successCheckMark = `d="M43.6 49.4l3.6 3.6 6.8-7.2"`

func readPage(t *testing.T, resp *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the page: %v", err)
	}

	return string(body)
}

func assertPageHeaders(t *testing.T, resp *http.Response) {
	t.Helper()

	want := map[string]string{
		"Content-Type":            "text/html; charset=utf-8",
		"Cache-Control":           "no-store",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"Content-Security-Policy": "default-src 'none'; style-src 'unsafe-inline'; img-src data:",
	}
	for name, value := range want {
		if got := resp.Header.Get(name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
}

func TestEveryCallbackPageRendersInBothLanguages(t *testing.T) {
	for _, tt := range callbackPageCases {
		for locale, heading := range tt.headings {
			t.Run(tt.name+"/"+locale, func(t *testing.T) {
				page := string(renderCallbackPage(locale, tt.kind, tt.oauthError))

				if !strings.Contains(page, `<html lang="`+locale+`">`) {
					t.Errorf("no lang=%q in the page", locale)
				}
				if !strings.Contains(page, "<title>"+heading+"</title>") || !strings.Contains(page, "<h1>"+heading+"</h1>") {
					t.Errorf("heading %q missing from:\n%s", heading, page)
				}
				if len(page) >= 16<<10 {
					t.Errorf("page is %d bytes", len(page))
				}
			})
		}
	}
}

func TestOnlyAllowListedOAuthErrorsShowTheirCode(t *testing.T) {
	for _, code := range []string{"access_denied", "invalid_scope", "server_error", "temporarily_unavailable"} {
		if page := string(renderCallbackPage("en", pageOAuthError, code)); !strings.Contains(page, "error="+code) {
			t.Errorf("%s: the page does not show its code", code)
		}
	}
	if page := string(renderCallbackPage("en", pageOAuthError, "invalid_request")); strings.Contains(page, "error=") {
		t.Error("an unknown code was shown")
	}
}

func TestAHostileOAuthErrorIsNeverReflected(t *testing.T) {
	page := string(renderCallbackPage("ru", pageOAuthError, "<script>alert(1)</script>"))

	if strings.Contains(page, "<script") || strings.Contains(page, "alert(1)") || strings.Contains(page, "error=") {
		t.Errorf("the error value reached the page:\n%s", page)
	}
	if !strings.Contains(page, "<h1>Провайдер вернул ошибку</h1>") {
		t.Error("the page does not use the copy for other errors")
	}
}

func TestAnEmptyOrUnknownLocaleRendersEnglish(t *testing.T) {
	for _, locale := range []string{"", "de"} {
		page := string(renderCallbackPage(locale, pageAuthorized, ""))
		if !strings.Contains(page, `<html lang="en">`) || !strings.Contains(page, "<h1>Authorization received</h1>") {
			t.Errorf("locale %q did not fall back to English", locale)
		}
	}
}

func TestNonBreakingSpacesReachThePage(t *testing.T) {
	for _, tt := range []struct {
		locale string
		kind   pageKind
		code   string
		phrase string
	}{
		{"en", pageOAuthError, "access_denied", "press Get\u00a0token on the Auth tab"},
		{"en", pageOAuthError, "invalid_request", "the Client\u00a0ID, the Authorization\u00a0URL"},
		{"ru", pageSignedIn, "", "Вернитесь в\u00a0приложение\u00a0— вход"},
		{"ru", pageOAuthError, "access_denied", "вернитесь в\u00a0Tetiva и\u00a0нажмите Get\u00a0token на\u00a0вкладке Auth"},
		{"ru", pageAuthorized, "", "на\u00a0вашем компьютере, поэтому в\u00a0адресе"},
	} {
		if page := string(renderCallbackPage(tt.locale, tt.kind, tt.code)); !strings.Contains(page, tt.phrase) {
			t.Errorf("%s: %q missing from:\n%s", tt.locale, tt.phrase, page)
		}
	}
}

func TestAnAuthorizationLinkUsedTwiceGetsANeutralPageOfItsOwn(t *testing.T) {
	for _, tt := range []struct{ locale, body string }{
		{"en", "<p>The result is in Tetiva.</p>"},
		{"ru", "<p>Результат\u00a0— в\u00a0Tetiva.</p>"},
	} {
		page := string(renderCallbackPage(tt.locale, pageOAuthAlreadyHandled, ""))

		if !strings.Contains(page, tt.body) {
			t.Errorf("%s: %q missing from:\n%s", tt.locale, tt.body, page)
		}
		if strings.Contains(page, successCheckMark) || strings.Contains(page, `class="b e"`) {
			t.Errorf("%s: the badge is not neutral:\n%s", tt.locale, page)
		}
	}
	if page := string(renderCallbackPage("en", pageSignInAlreadyHandled, "")); !strings.Contains(page, successCheckMark) {
		t.Error("the sign-in page lost its check mark")
	}
}

func TestAuthorizationPagesNameTheCollectionTabToo(t *testing.T) {
	places := map[string]string{
		"en": "on the Auth tab (Authorization for a collection)",
		"ru": "на\u00a0вкладке Auth (у\u00a0коллекции\u00a0— «Авторизация»)",
	}
	for _, tt := range []struct {
		kind pageKind
		code string
	}{
		{pageAuthorized, ""},
		{pageOAuthError, "access_denied"},
		{pageOAuthError, "invalid_scope"},
		{pageOAuthError, "invalid_request"},
	} {
		for locale, place := range places {
			if page := string(renderCallbackPage(locale, tt.kind, tt.code)); !strings.Contains(page, place) {
				t.Errorf("%s %d %q: %q missing from:\n%s", locale, tt.kind, tt.code, place, page)
			}
		}
	}
}

func TestLoopbackAnswersAnErrorRedirectWithAStyledPageAndThen410(t *testing.T) {
	lb := newTestLoopback(t, 30*time.Second)
	target := lb.redirectURI() + "?state=" + testState + "&error=access_denied&error_description=%3Cscript%3E"

	resp := callbackGet(t, target)
	page := readPage(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	assertPageHeaders(t, resp)
	if !strings.Contains(page, "<h1>Access denied</h1>") || strings.Contains(page, "<script") {
		t.Errorf("page:\n%s", page)
	}
	<-lb.done

	again := callbackGet(t, target)
	page = readPage(t, again)
	if again.StatusCode != http.StatusGone {
		t.Fatalf("repeated callback: status = %d, want 410", again.StatusCode)
	}
	assertPageHeaders(t, again)
	if !strings.Contains(page, "<h1>This authorization link was already handled</h1>") {
		t.Errorf("410 page:\n%s", page)
	}
}

func TestSignalLoopbackAnswersInItsLocaleAndThen410(t *testing.T) {
	lb, err := startSignalLoopback("0", "ru", FlowOptions{DrainWindow: 30 * time.Second}.withDefaults())
	if err != nil {
		t.Fatalf("startSignalLoopback: %v", err)
	}
	t.Cleanup(lb.close)

	resp := callbackGet(t, lb.redirectURI())
	if page := readPage(t, resp); resp.StatusCode != http.StatusOK || !strings.Contains(page, "<h1>Вы вошли в\u00a0Tetiva</h1>") {
		t.Fatalf("status = %d, page:\n%s", resp.StatusCode, page)
	}
	assertPageHeaders(t, resp)
	<-lb.signal()

	again := callbackGet(t, lb.redirectURI())
	if page := readPage(t, again); again.StatusCode != http.StatusGone || !strings.Contains(page, "<h1>Эта ссылка уже использована</h1>") {
		t.Errorf("status = %d, page:\n%s", again.StatusCode, page)
	}
}

func TestEachAuthCodeFlowAnswersInTheLanguageItStartedWith(t *testing.T) {
	f := newFlowFixture(t, nil)
	owner := testOwner()

	for _, tt := range []struct{ locale, heading string }{
		{"ru", "<h1>Авторизация получена</h1>"},
		{"en", "<h1>Authorization received</h1>"},
	} {
		id := uuid.NewString()
		info, err := f.mgr.StartAuthCode(context.Background(), id, owner, f.codeConfig(), "0", tt.locale)
		if err != nil {
			t.Fatalf("StartAuthCode(%s): %v", tt.locale, err)
		}

		q := authorizeQuery(t, info.AuthorizeURL)
		callback := url.Values{"code": {"the-code"}, "state": {q.Get("state")}}
		if page := readPage(t, callbackGet(t, q.Get("redirect_uri")+"?"+callback.Encode())); !strings.Contains(page, tt.heading) {
			t.Errorf("flow started with %q answered:\n%s", tt.locale, page)
		}
		f.waitState(id, FlowDone)
	}
}

func TestSignInRedirectAnswersInTheFlowsLocale(t *testing.T) {
	for _, tt := range []struct{ locale, heading string }{
		{"ru", "<h1>Вы вошли в\u00a0Tetiva</h1>"},
		{"en", "<h1>You’re signed in to Tetiva</h1>"},
	} {
		t.Run(tt.locale, func(t *testing.T) {
			client := newFakeSignInClient()
			f := newSignInClockFixture(t, client)
			if _, err := f.m.Start(context.Background(), f.id, client,
				SignInParams{ClientID: "cid", Locale: tt.locale}, f.probe.hooks()); err != nil {
				t.Fatalf("Start: %v", err)
			}

			redirectURI, _, _ := client.startArgs()
			if page := readPage(t, callbackGet(t, redirectURI)); !strings.Contains(page, tt.heading) {
				t.Errorf("page:\n%s", page)
			}
		})
	}
}
