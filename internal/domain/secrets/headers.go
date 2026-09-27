package secrets

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/tetiva-app/client/internal/domain/entities"
)

const redactedValue = "<redacted>"

var sensitiveHeaders = map[string]struct{}{
	"authorization":             {},
	"proxy-authorization":       {},
	"cookie":                    {},
	"set-cookie":                {},
	"x-api-key":                 {},
	"api-key":                   {},
	"apikey":                    {},
	"x-auth-token":              {},
	"x-access-token":            {},
	"x-csrf-token":              {},
	"x-session-token":           {},
	"x-amz-security-token":      {},
	"x-amz-content-sha256":      {},
	"private-token":             {},
	"x-vault-token":             {},
	"x-goog-api-key":            {},
	"ocp-apim-subscription-key": {},
	"x-rapidapi-key":            {},
	"x-api-token":               {},
	"x-shopify-access-token":    {},
	// A Digest challenge carries the nonce a replay would need.
	"www-authenticate":   {},
	"proxy-authenticate": {},
}

var sensitiveQueryParams = map[string]struct{}{
	"token":                {},
	"access_token":         {},
	"refresh_token":        {},
	"api_key":              {},
	"apikey":               {},
	"key":                  {},
	"secret":               {},
	"password":             {},
	"sig":                  {},
	"signature":            {},
	"code":                 {},
	"code_verifier":        {},
	"client_secret":        {},
	"assertion":            {},
	"id_token":             {},
	"oauth_verifier":       {},
	"x-amz-signature":      {},
	"x-amz-credential":     {},
	"x-amz-security-token": {},
}

var sensitiveTokens = map[string]struct{}{
	"key": {}, "keys": {}, "auth": {}, "authorization": {}, "authentication": {}, "session": {}, "private": {},
}

// net/http turns "X-ApiToken" into "X-Apitoken"; "key" stays out to spare "monkey".
var sensitiveSuffixes = []string{
	"token", "tokens", "secret", "secrets", "password", "passwords", "passwd",
	"credential", "credentials", "signature", "signatures", "cookie", "cookies",
}

var sensitiveCompounds = []string{
	"apikey", "accesstoken", "refreshtoken", "clientsecret", "sessionid", "xsrf", "csrf",
	"authtoken", "secretkey", "privatekey", "accesskey", "authkey", "privkey",
}

var sensitiveWords = []string{
	"token", "secret", "key", "auth", "session", "signature", "password", "passwd", "passphrase", "pwd",
	"credential", "cookie", "private",
}

// No "auth", "key", "session": they also name Postman auth blocks, JWKS, session records.
var secretContainerWords = []string{"token", "secret", "password", "passwd", "passphrase", "pwd", "credential", "private"}

var sensitiveWholeWords = map[string]struct{}{"pass": {}}

var ordinaryWords = map[string]struct{}{
	"author": {}, "authors": {}, "authored": {}, "authority": {}, "authorities": {},
	"keyword": {}, "keywords": {}, "keyboard": {}, "keyboards": {}, "keynote": {}, "keystone": {}, "keystroke": {},
	"keyframe": {}, "keyframes": {}, "hotkey": {}, "hotkeys": {},
	"monkey": {}, "monkeys": {}, "donkey": {}, "turkey": {}, "hockey": {}, "jockey": {}, "whiskey": {},
	"tokenizer": {}, "tokenizers": {}, "tokenize": {}, "tokenized": {}, "tokenization": {},
	"secretary": {}, "secretariat": {},
}

var ordinaryNames = map[string]struct{}{
	"idempotency key": {}, "page token": {}, "next token": {}, "next page token": {}, "sort key": {}, "key id": {}, "token type": {},
}

// CORS headers name other headers ("Allow-Headers: Authorization"), never credentials.
const corsHeaderPrefix = "access-control-"

var varRef = regexp.MustCompile(`\{\{[^}]+\}\}`)

var (
	authScheme     = regexp.MustCompile(`(?i)^(?:bearer|basic|token|digest|api-?key)[ \t]+`)
	bareAuthScheme = regexp.MustCompile(`(?i)^(?:bearer|basic|token|digest|api-?key)$`)
)

// IsSensitiveHeader flags a {{…}} name too: what it resolves to is unknown here.
func IsSensitiveHeader(name string) bool {
	if varRef.MatchString(name) {
		return true
	}
	if IsCORSHeader(name) {
		return false
	}
	return isSensitive(strings.TrimSpace(name), sensitiveHeaders)
}

// IsSensitiveQueryParam decodes first: receivers read "access%5Ftoken" as "access_token".
func IsSensitiveQueryParam(name string) bool {
	if decoded, err := url.QueryUnescape(name); err == nil {
		name = decoded
	}
	return isSensitive(strings.TrimSpace(name), sensitiveQueryParams)
}

func IsCORSHeader(name string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(name)), corsHeaderPrefix)
}

func ContainsSensitiveWord(name string) bool {
	return containsWord(name, sensitiveWords, sensitiveWholeWords)
}

func NamesSecretContainer(name string) bool {
	return containsWord(name, secretContainerWords, nil)
}

func containsWord(name string, words []string, wholeWords map[string]struct{}) bool {
	if decoded, err := url.QueryUnescape(name); err == nil {
		name = decoded
	}
	for _, tok := range nameTokens(name) {
		word := strings.TrimRight(tok, "0123456789")
		if _, ok := ordinaryWords[word]; ok {
			continue
		}
		if _, ok := wholeWords[word]; ok {
			return true
		}
		for _, w := range words {
			if strings.Contains(word, w) {
				return true
			}
		}
	}
	return false
}

func IsOrdinaryName(name string) bool {
	if decoded, err := url.QueryUnescape(name); err == nil {
		name = decoded
	}
	tokens := nameTokens(name)
	if len(tokens) > 1 && tokens[0] == "x" {
		tokens = tokens[1:]
	}
	if len(tokens) > 1 && tokens[len(tokens)-1] == "endpoint" {
		return true
	}
	_, ok := ordinaryNames[strings.Join(tokens, " ")]
	return ok
}

func isSensitive(name string, exact map[string]struct{}) bool {
	if _, ok := exact[strings.ToLower(name)]; ok {
		return true
	}
	for _, tok := range nameTokens(name) {
		word := strings.TrimRight(tok, "0123456789")
		if _, ok := sensitiveTokens[word]; ok {
			return true
		}
		for _, suf := range sensitiveSuffixes {
			if strings.HasSuffix(word, suf) {
				return true
			}
		}
		for _, c := range sensitiveCompounds {
			if strings.Contains(tok, c) {
				return true
			}
		}
	}
	return false
}

func nameTokens(name string) []string {
	var (
		tokens []string
		cur    []rune
	)
	flush := func() {
		if len(cur) > 0 {
			tokens = append(tokens, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	runes := []rune(name)
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
	return tokens
}

func RedactValue(value string) string {
	literals := strings.TrimSpace(varRef.ReplaceAllString(value, ""))
	if literals == "" || bareAuthScheme.MatchString(literals) {
		return value
	}

	scheme := authScheme.FindString(value)
	rest := value[len(scheme):]
	var b strings.Builder
	b.WriteString(scheme)
	last := 0
	for _, ref := range varRef.FindAllStringIndex(rest, -1) {
		writeLiteral(&b, rest[last:ref[0]])
		b.WriteString(rest[ref[0]:ref[1]])
		last = ref[1]
	}
	writeLiteral(&b, rest[last:])
	return b.String()
}

func writeLiteral(b *strings.Builder, run string) {
	if strings.TrimSpace(run) == "" {
		b.WriteString(run)
		return
	}
	b.WriteString(redactedValue)
}

func RedactHeaders(h []entities.HeaderItem) []entities.HeaderItem {
	if h == nil {
		return nil
	}
	out := make([]entities.HeaderItem, len(h))
	for i, item := range h {
		if IsSensitiveHeader(item.Key) {
			item.Value = RedactValue(item.Value)
		} else if redact, ok := urlHeaders[strings.ToLower(strings.TrimSpace(item.Key))]; ok {
			item.Value = redact(item.Value)
		}
		out[i] = item
	}
	return out
}
