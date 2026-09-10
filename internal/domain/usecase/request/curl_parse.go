package request

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// Returned only when there is nothing to import; everything softer becomes a warning.
var (
	ErrCurlEmpty   = errors.New("empty command")
	ErrCurlNotCurl = errors.New("command must start with curl")
	ErrCurlNoURL   = errors.New("command has no URL")
)

// ParsedCurl.Warnings are user-facing English strings; the UI translates them.
type ParsedCurl struct {
	Method   string
	URL      string
	Headers  []entities.HeaderItem
	BodyType entities.BodyType
	Body     string
	AuthType entities.AuthType
	AuthData string
	Warnings []string
}

// JSON tags describe the AuthData wire format shared with the frontend,
// same rationale as formField: these types never escape this package.
type basicAuthJSON struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type bearerAuthJSON struct {
	Token  string `json:"token"`
	Prefix string `json:"prefix"`
}

type awsAuthJSON struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken,omitempty"`
	Region          string `json:"region"`
	Service         string `json:"service"`
}

// ParseCurl turns a pasted curl command into request fields. No shell is ever
// involved: $(...), backticks and ${VAR} survive as literal text.
func ParseCurl(text string) (*ParsedCurl, error) {
	const funcName = "request.ParseCurl"

	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("%s: %w", funcName, ErrCurlEmpty)
	}

	args, warnings := lexCurl(text)
	if len(args) == 0 || (!isCurlWord(args[0]) && !isCurlWord(firstRawToken(text))) {
		return nil, fmt.Errorf("%s: %w", funcName, ErrCurlNotCurl)
	}

	p := &curlParser{args: args[1:], warnings: warnings}
	p.parseArgs()
	if p.url == "" {
		return nil, fmt.Errorf("%s: %w", funcName, ErrCurlNoURL)
	}
	return p.build(), nil
}

// firstRawToken is the leading word before lexing: the lexer reads the
// backslashes of an unquoted Windows path as shell escapes and eats them.
func firstRawToken(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// isCurlWord accepts the bare name and the paths curl is invoked by, but only
// after the token is ruled out as a URL: https://host/curl is not a command.
func isCurlWord(arg string) bool {
	name := strings.ToLower(strings.TrimSpace(arg))
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		if !isProgramPath(name) {
			return false
		}
		name = name[i+1:]
	}
	return name == "curl" || name == "curl.exe"
}

// isProgramPath tells a local path from a URL or a network location; the
// argument is lower-cased and holds at least one separator.
func isProgramPath(s string) bool {
	if strings.Contains(s, "://") || len(s) > 1 && isPathSep(s[0]) && isPathSep(s[1]) {
		return false
	}
	if isPathSep(s[0]) || s[0] == '~' || s[0] == '.' {
		return true
	}
	// Windows drive letter: c:\tools\curl.exe.
	return len(s) > 2 && s[0] >= 'a' && s[0] <= 'z' && s[1] == ':' && isPathSep(s[2])
}

func isPathSep(c byte) bool { return c == '/' || c == '\\' }

const (
	lexPlain = iota
	lexSingle
	lexDouble
	lexAnsi
)

// lexCurl splits a command into arguments like a shell would, plus the Windows caret style. The URL bar
// collapses newlines into spaces first, so an escaped space is ambiguous — see isLineContinuation.
func lexCurl(text string) ([]string, []string) {
	var (
		args     []string
		warnings []string
		buf      strings.Builder
		started  bool
	)
	state := lexPlain

	flush := func() {
		if started {
			args = append(args, buf.String())
			buf.Reset()
			started = false
		}
	}

	r := []rune(text)
	for i := 0; i < len(r); i++ {
		c := r[i]

		switch state {
		case lexSingle:
			if c == '\'' {
				state = lexPlain
				continue
			}
			buf.WriteRune(c)

		case lexAnsi:
			switch {
			case c == '\'':
				state = lexPlain
			case c == '\\' && i+1 < len(r):
				text, consumed := ansiEscape(r, i+1)
				buf.WriteString(text)
				i += consumed
			default:
				buf.WriteRune(c)
			}

		case lexDouble:
			switch {
			case c == '"':
				state = lexPlain
			case c == '\\' && i+1 < len(r):
				i++
				switch r[i] {
				case '"', '\\', '$', '`':
					buf.WriteRune(r[i])
				case '\n':
				case '\r':
					if i+1 < len(r) && r[i+1] == '\n' {
						i++
					}
				default:
					buf.WriteRune(c)
					buf.WriteRune(r[i])
				}
			case c == '^' && i+1 < len(r):
				i++
				if r[i] == '"' {
					state = lexPlain
				} else {
					buf.WriteRune(r[i])
				}
			default:
				buf.WriteRune(c)
			}

		default:
			switch {
			case c == '\'':
				state, started = lexSingle, true
			case c == '"':
				state, started = lexDouble, true
			// Chrome emits $'...' (ANSI-C quoting) for values with quotes or newlines.
			case c == '$' && i+1 < len(r) && r[i+1] == '\'':
				i++
				state, started = lexAnsi, true
			case c == '$' && i+1 < len(r) && r[i+1] == '"':
				i++
				state, started = lexDouble, true
			case c == '\\' && i+1 < len(r):
				i++
				switch {
				case !isLexSpace(r[i]):
					buf.WriteRune(r[i])
					started = true
				case isLineContinuation(r, i, started):
					flush()
				default:
					buf.WriteRune(r[i])
					started = true
				}
			case c == '^' && i+1 < len(r):
				i++
				switch {
				case isLexSpace(r[i]):
					if isLineContinuation(r, i, started) {
						flush()
					} else {
						buf.WriteRune(r[i])
						started = true
					}
				case r[i] == '"':
					state, started = lexDouble, true
				case r[i] == '\'':
					state, started = lexSingle, true
				default:
					buf.WriteRune(r[i])
					started = true
				}
			case isLexSpace(c):
				flush()
			default:
				buf.WriteRune(c)
				started = true
			}
		}
	}

	if state != lexPlain {
		warnings = append(warnings, "unterminated quote in the command; parsed to the end of the input")
	}
	flush()
	return args, warnings
}

// isLineContinuation tells a collapsed "\<newline>" apart from an escaped space
// inside a value; r[i] is the escaped whitespace rune.
func isLineContinuation(r []rune, i int, started bool) bool {
	if r[i] == '\n' || r[i] == '\r' || !started {
		return true
	}
	for j := i + 1; j < len(r); j++ {
		if !isLexSpace(r[j]) {
			return r[j] == '-'
		}
	}
	return true
}

// ansiEscape decodes the $'...' escape that starts at r[j], just past the
// backslash, and reports how many runes it consumed.
func ansiEscape(r []rune, j int) (string, int) {
	switch c := r[j]; c {
	case 'n':
		return "\n", 1
	case 't':
		return "\t", 1
	case 'r':
		return "\r", 1
	case 'a':
		return "\a", 1
	case 'b':
		return "\b", 1
	case 'f':
		return "\f", 1
	case 'v':
		return "\v", 1
	case 'e', 'E':
		return "\x1b", 1
	case '\\', '\'', '"', '?':
		return string(c), 1
	case 'x':
		if v, n := readDigits(r, j+1, 16, 2); n > 0 {
			return string(rune(v)), n + 1
		}
	case 'u':
		if v, n := readDigits(r, j+1, 16, 4); n > 0 && utf8.ValidRune(rune(v)) {
			return string(rune(v)), n + 1
		}
	case 'U':
		if v, n := readDigits(r, j+1, 16, 8); n > 0 && utf8.ValidRune(rune(v)) {
			return string(rune(v)), n + 1
		}
	case '0', '1', '2', '3', '4', '5', '6', '7':
		// Bash writes a raw byte here; we widen it to a rune so the value stays valid UTF-8.
		if v, n := readDigits(r, j, 8, 3); n > 0 {
			return string(rune(v)), n
		}
	}
	return "\\" + string(r[j]), 1
}

func readDigits(r []rune, j, base, max int) (int, int) {
	v, n := 0, 0
	for ; n < max && j+n < len(r); n++ {
		d := digitValue(r[j+n])
		if d < 0 || d >= base {
			break
		}
		v = v*base + d
	}
	return v, n
}

func digitValue(c rune) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func isLexSpace(c rune) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

type curlParser struct {
	args []string
	pos  int

	method      string
	url         string
	headers     []entities.HeaderItem
	dataParts   []string
	jsonParts   []string
	formFields  []formField
	binaryPath  string
	binaryCount int
	user        string
	hasUser     bool
	digestFlag  bool
	sigv4       string
	hasSigv4    bool
	hasData     bool
	hasForm     bool
	getFlag     bool
	headFlag    bool
	jsonFlag    bool
	warnings    []string
}

func (p *curlParser) parseArgs() {
	for p.pos = 0; p.pos < len(p.args); p.pos++ {
		arg := p.args[p.pos]
		switch {
		case arg == "--" || arg == "-":
		case strings.HasPrefix(arg, "--"):
			p.longFlag(arg)
		case len(arg) > 1 && strings.HasPrefix(arg, "-"):
			p.shortFlags(arg)
		default:
			p.setURL(arg)
		}
	}
}

func (p *curlParser) longFlag(arg string) {
	name, inline := strings.TrimPrefix(arg, "--"), ""
	hasInline := false
	if i := strings.Index(name, "="); i >= 0 {
		name, inline, hasInline = name[:i], name[i+1:], true
	}
	flag := "--" + name
	value := func() (string, bool) { return p.flagValue(flag, inline, hasInline) }

	switch name {
	case "request":
		if v, ok := value(); ok {
			p.setMethod(v)
		}
	case "header":
		if v, ok := value(); ok {
			p.addHeaderArg(v)
		}
	case "data", "data-raw", "data-ascii":
		if v, ok := value(); ok {
			p.addData(flag, v, name != "data-raw")
		}
	case "data-binary":
		if v, ok := value(); ok {
			p.addBinaryData(v)
		}
	case "data-urlencode":
		if v, ok := value(); ok {
			p.addURLEncodedData(flag, v)
		}
	case "form":
		if v, ok := value(); ok {
			p.addFormField(flag, v, false)
		}
	case "form-string":
		if v, ok := value(); ok {
			p.addFormField(flag, v, true)
		}
	case "user":
		if v, ok := value(); ok {
			p.setUser(v)
		}
	case "digest":
		p.digestFlag = true
	case "aws-sigv4":
		if v, ok := value(); ok {
			p.sigv4, p.hasSigv4 = v, true
		}
	case "cookie":
		if v, ok := value(); ok {
			p.addCookie(flag, v)
		}
	case "user-agent":
		if v, ok := value(); ok {
			p.setHeader("User-Agent", v)
		}
	case "referer":
		if v, ok := value(); ok {
			p.setHeader("Referer", v)
		}
	case "json":
		if v, ok := value(); ok {
			p.addJSONData(flag, v)
		}
	case "url":
		if v, ok := value(); ok {
			p.setURL(v)
		}
	case "output":
		value()
	case "get":
		p.getFlag = true
	case "head":
		p.headFlag = true
	case "location", "location-trusted", "compressed", "silent", "verbose",
		"include", "remote-name", "progress-bar", "fail", "no-progress-meter":
	case "insecure", "http1.0", "http1.1", "http2", "http2-prior-knowledge", "http3",
		"tlsv1", "tlsv1.0", "tlsv1.1", "tlsv1.2", "tlsv1.3", "sslv2", "sslv3":
		p.warnf("%s is not supported and was ignored", flag)
	default:
		if longFlagValues[name] {
			value()
			p.warnf("%s is not supported and was ignored", flag)
			return
		}
		if !hasInline && p.consumeUnknownValue() {
			p.warnf("unknown flag %s and its value were ignored", flag)
			return
		}
		p.warnf("unknown flag %s was ignored", flag)
	}
}

// longFlagValues lists long flags we do not act on but that take a value, so the
// value is never mistaken for the URL. Anything absent falls back to a guess.
var longFlagValues = map[string]bool{
	"abstract-unix-socket": true, "cacert": true, "capath": true,
	"cert": true, "cert-type": true, "ciphers": true, "config": true,
	"connect-timeout": true, "connect-to": true, "continue-at": true, "cookie-jar": true,
	"create-file-mode": true, "crlfile": true, "dns-servers": true, "dump-header": true,
	"egd-file": true, "engine": true, "expect100-timeout": true, "ftp-port": true,
	"happy-eyeballs-timeout-ms": true, "hostpubmd5": true, "interface": true,
	"key": true, "key-type": true, "krb": true, "libcurl": true, "limit-rate": true,
	"local-port": true, "login-options": true, "mail-from": true, "mail-rcpt": true,
	"max-filesize": true, "max-redirs": true, "max-time": true, "netrc-file": true,
	"noproxy": true, "oauth2-bearer": true, "output-dir": true, "pass": true,
	"pinnedpubkey": true, "proto": true, "proto-default": true, "proto-redir": true,
	"proxy": true, "proxy-user": true, "quote": true, "random-file": true,
	"range": true, "request-target": true, "resolve": true, "retry": true,
	"retry-delay": true, "retry-max-time": true, "service-name": true,
	"socks4": true, "socks4a": true, "socks5": true, "socks5-hostname": true,
	"speed-limit": true, "speed-time": true, "stderr": true, "telnet-option": true,
	"time-cond": true, "tls13-ciphers": true, "tlspassword": true, "tlsuser": true,
	"trace": true, "trace-ascii": true, "unix-socket": true, "upload-file": true,
	"url-query": true, "write-out": true,
}

// consumeUnknownValue swallows the argument of a flag we do not know. Losing the
// URL is far worse than keeping a stray value, so a URL-looking token is left alone.
func (p *curlParser) consumeUnknownValue() bool {
	if p.pos+1 >= len(p.args) {
		return false
	}
	next := p.args[p.pos+1]
	if next == "" || strings.HasPrefix(next, "-") || looksLikeURL(next) {
		return false
	}
	p.pos++
	return true
}

func looksLikeURL(v string) bool {
	if strings.Contains(v, "://") || strings.HasPrefix(v, "//") {
		return true
	}
	host := v
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if i := strings.LastIndex(host, "@"); i >= 0 {
		host = host[i+1:]
	}
	if i := strings.LastIndex(host, ":"); i >= 0 {
		if port := host[i+1:]; port != "" && strings.Trim(port, "0123456789") == "" {
			return true
		}
		host = host[:i]
	}
	return strings.Contains(host, ".") || strings.EqualFold(host, "localhost")
}

// shortFlagValues lists the short flags that consume an argument.
var shortFlagValues = map[rune]bool{
	'X': true, 'H': true, 'd': true, 'F': true, 'u': true,
	'b': true, 'A': true, 'e': true, 'o': true, 'x': true, 'm': true,
	'T': true, 'U': true, 'r': true, 'c': true, 'w': true, 'K': true,
	'E': true, 'C': true, 'D': true, 'z': true, 'y': true, 'Y': true,
	'P': true, 'Q': true, 't': true,
}

// shortFlags handles bundles like -sSL and attached values like -XPOST.
func (p *curlParser) shortFlags(arg string) {
	chars := []rune(arg[1:])
	for i := 0; i < len(chars); i++ {
		c := chars[i]
		flag := "-" + string(c)

		var v string
		if shortFlagValues[c] {
			var ok bool
			if inline := string(chars[i+1:]); inline != "" {
				v, ok = inline, true
				i = len(chars)
			} else {
				v, ok = p.flagValue(flag, "", false)
			}
			if !ok {
				continue
			}
		}

		switch c {
		case 'X':
			p.setMethod(v)
		case 'H':
			p.addHeaderArg(v)
		case 'd':
			p.addData(flag, v, true)
		case 'F':
			p.addFormField(flag, v, false)
		case 'u':
			p.setUser(v)
		case 'b':
			p.addCookie(flag, v)
		case 'A':
			p.setHeader("User-Agent", v)
		case 'e':
			p.setHeader("Referer", v)
		case 'G':
			p.getFlag = true
		case 'I':
			p.headFlag = true
		case 'o', 'L', 's', 'v', 'i', 'O', 'f', '#':
		case 'k', 'x', 'm', 'T', 'U', 'r', 'c', 'w', 'K', 'E', 'C', 'D', 'z', 'y', 'Y', 'P', 'Q', 't':
			p.warnf("%s is not supported and was ignored", flag)
		default:
			// Only the last flag of a bundle can carry an argument, like in curl.
			if i == len(chars)-1 && p.consumeUnknownValue() {
				p.warnf("unknown flag %s and its value were ignored", flag)
				continue
			}
			p.warnf("unknown flag %s was ignored", flag)
		}
	}
}

func (p *curlParser) flagValue(flag, inline string, hasInline bool) (string, bool) {
	if hasInline {
		return inline, true
	}
	if p.pos+1 < len(p.args) {
		p.pos++
		return p.args[p.pos], true
	}
	p.warnf("%s expects a value and was ignored", flag)
	return "", false
}

func (p *curlParser) setURL(v string) {
	if p.url == "" {
		p.url = v
		return
	}
	p.warnf("extra argument %q was ignored; only the first URL is used", v)
}

func (p *curlParser) setMethod(v string) {
	m := entities.HTTPMethod(strings.ToUpper(strings.TrimSpace(v)))
	if !m.IsValid() {
		p.warnf("HTTP method %q is not supported and was ignored", v)
		return
	}
	p.method = string(m)
}

func (p *curlParser) addHeaderArg(raw string) {
	h := strings.TrimSpace(raw)
	if h == "" {
		return
	}
	idx := strings.Index(h, ":")
	if idx < 0 {
		if strings.HasSuffix(h, ";") {
			p.warnf("header %q uses the empty-header form and was skipped", h)
			return
		}
		p.warnf("header %q has no colon and was skipped", h)
		return
	}
	key := strings.TrimSpace(h[:idx])
	value := strings.TrimSpace(h[idx+1:])
	if key == "" {
		p.warnf("header %q has no name and was skipped", h)
		return
	}
	if value == "" {
		p.warnf("header %q has no value and was skipped", key)
		return
	}
	p.headers = append(p.headers, entities.HeaderItem{Key: key, Value: value, Enabled: true})
}

func (p *curlParser) setHeader(key, value string) {
	if i := headerIndex(p.headers, key); i >= 0 {
		p.headers[i].Value = value
		return
	}
	p.headers = append(p.headers, entities.HeaderItem{Key: key, Value: value, Enabled: true})
}

func (p *curlParser) addCookie(flag, raw string) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return
	}
	// curl reads a cookie jar when the value has no '=' or starts with '@'.
	if strings.HasPrefix(v, "@") || !strings.Contains(v, "=") {
		p.warnf("%s with a cookie file is not supported and was ignored", flag)
		return
	}
	if i := headerIndex(p.headers, "Cookie"); i >= 0 {
		p.headers[i].Value += "; " + v
		return
	}
	p.headers = append(p.headers, entities.HeaderItem{Key: "Cookie", Value: v, Enabled: true})
}

func (p *curlParser) setUser(v string) {
	p.user, p.hasUser = v, true
}

func (p *curlParser) addData(flag, v string, fileRef bool) {
	p.hasData = true
	// curl reads @file as text here, which we cannot do; importing it as a binary
	// body would change what gets sent.
	if fileRef && strings.HasPrefix(v, "@") {
		p.warnf("%s reads the body from a file, which is not supported; the body was left empty", flag)
		return
	}
	p.dataParts = append(p.dataParts, v)
}

func (p *curlParser) addBinaryData(v string) {
	p.hasData = true
	if !strings.HasPrefix(v, "@") {
		p.dataParts = append(p.dataParts, v)
		return
	}
	p.setBinaryPath(strings.TrimPrefix(v, "@"))
}

func (p *curlParser) setBinaryPath(path string) {
	path = strings.TrimSpace(path)
	switch path {
	case "":
		p.warnf("empty file path in the request body was ignored")
	case "-":
		p.warnf("reading the request body from stdin is not supported")
	default:
		p.warnFilePath(path)
		p.binaryPath = path
		p.binaryCount++
	}
}

// warnFilePath reports paths the send path rejects: it needs an absolute path without traversal.
func (p *curlParser) warnFilePath(path string) {
	if !filepath.IsAbs(path) {
		p.warnf("file path %q is relative; an absolute path is required to send this request", path)
	}
	if strings.Contains(path, "..") {
		p.warnf("file path %q contains \"..\"; the request will be rejected when it is sent", path)
	}
}

// addURLEncodedData follows curl: the first '=' splits name from content, and only
// the content is percent-encoded; without '=' an '@' means a file.
func (p *curlParser) addURLEncodedData(flag, v string) {
	p.hasData = true

	if i := strings.Index(v, "="); i >= 0 {
		if i == 0 {
			p.dataParts = append(p.dataParts, url.QueryEscape(v[1:]))
			return
		}
		p.dataParts = append(p.dataParts, v[:i]+"="+url.QueryEscape(v[i+1:]))
		return
	}
	if strings.Contains(v, "@") {
		p.warnf("%s with a file is not supported and was ignored", flag)
		return
	}
	p.dataParts = append(p.dataParts, url.QueryEscape(v))
}

func (p *curlParser) addJSONData(flag, v string) {
	p.hasData = true
	p.jsonFlag = true
	if strings.HasPrefix(v, "@") {
		p.warnf("%s @file is not supported and was ignored", flag)
		return
	}
	p.jsonParts = append(p.jsonParts, v)
}

func (p *curlParser) addFormField(flag, v string, textOnly bool) {
	p.hasForm = true
	idx := strings.Index(v, "=")
	if idx <= 0 {
		p.warnf("%s value %q has no field name and was skipped", flag, v)
		return
	}

	field := formField{Key: v[:idx], Value: v[idx+1:], Type: "text", Enabled: true}
	if !textOnly {
		switch {
		// '<' asks curl for a text field whose value is the file content, not an upload.
		case strings.HasPrefix(field.Value, "<"):
			p.warnf("%s field %q takes its value from a file, which is not supported, and was skipped", flag, field.Key)
			return
		case strings.HasPrefix(field.Value, "@"):
			field.Type = "file"
			field.Value = stripFormOptions(field.Value[1:])
			if field.Value == "" {
				p.warnf("%s field %q has an empty file path and was skipped", flag, field.Key)
				return
			}
			p.warnFilePath(field.Value)
		}
	}
	p.formFields = append(p.formFields, field)
}

// stripFormOptions drops the `;type=`, `;filename=` and friends curl accepts after a -F file path.
func stripFormOptions(v string) string {
	lower := strings.ToLower(v)
	cut := -1
	for _, opt := range []string{";type=", ";filename=", ";headers=", ";encoder="} {
		if i := strings.Index(lower, opt); i >= 0 && (cut < 0 || i < cut) {
			cut = i
		}
	}
	if cut >= 0 {
		return v[:cut]
	}
	return v
}

func (p *curlParser) build() *ParsedCurl {
	res := &ParsedCurl{
		URL:      p.normalizeURL(p.url),
		AuthType: entities.AuthTypeNone,
	}

	body := p.bodyText()
	res.Method = p.resolveMethod()

	if p.getFlag && body != "" {
		res.URL = appendQuery(res.URL, body)
		body = ""
	}

	if p.jsonFlag {
		p.defaultHeader("Content-Type", "application/json")
		p.defaultHeader("Accept", "application/json")
	}

	p.applyAuth(res)
	p.applyBody(res, body)

	res.Headers = p.headers
	res.Warnings = dedupeWarnings(p.warnings)
	return res
}

// bodyText joins the collected data flags: --data variants with '&' like curl,
// --json chunks by plain concatenation.
func (p *curlParser) bodyText() string {
	body := strings.Join(p.dataParts, "&")
	if len(p.jsonParts) == 0 {
		return body
	}
	jsonBody := strings.Join(p.jsonParts, "")
	if body == "" {
		return jsonBody
	}
	return body + "&" + jsonBody
}

func (p *curlParser) resolveMethod() string {
	switch {
	case p.method != "":
		return p.method
	case p.getFlag:
		return string(entities.MethodGET)
	case p.headFlag:
		return string(entities.MethodHEAD)
	case p.hasData || p.hasForm:
		return string(entities.MethodPOST)
	default:
		return string(entities.MethodGET)
	}
}

func (p *curlParser) applyBody(res *ParsedCurl, body string) {
	switch {
	case p.hasForm && len(p.formFields) > 0:
		if body != "" || p.binaryPath != "" {
			p.warnf("the request has both form fields and data; only the form fields were kept")
		}
		// Our body model has one form type: without a file field the send path
		// encodes it as urlencoded, while curl always posts multipart here.
		if !hasFormFile(p.formFields) {
			p.warnf("form fields will be sent as application/x-www-form-urlencoded, not multipart/form-data as curl does")
		}
		res.BodyType = entities.BodyTypeForm
		res.Body = jsonString(p.formFields)
	case p.binaryPath != "":
		if p.binaryCount > 1 {
			p.warnf("several file bodies were given; only %q was kept", p.binaryPath)
		}
		if body != "" {
			p.warnf("inline data and a file body were mixed; only the file %q was kept", p.binaryPath)
		}
		res.BodyType = entities.BodyTypeBinary
		res.Body = p.binaryPath
	case body == "":
		res.BodyType = entities.BodyTypeNone
	default:
		res.BodyType, res.Body = p.classifyBody(body)
	}
}

func (p *curlParser) classifyBody(body string) (entities.BodyType, string) {
	ct := strings.ToLower(headerValue(p.headers, "Content-Type"))
	switch {
	case strings.Contains(ct, "json"):
		return entities.BodyTypeJSON, body
	case strings.Contains(ct, "xml"):
		return entities.BodyTypeXML, body
	case strings.Contains(ct, "x-www-form-urlencoded"):
		return p.formOrRaw(body)
	case ct != "":
		return entities.BodyTypeRaw, body
	case looksLikeJSONBody(body):
		p.warnf("no Content-Type header: the body looks like JSON and was imported as JSON")
		return entities.BodyTypeJSON, body
	default:
		return p.formOrRaw(body)
	}
}

// formOrRaw converts a urlencoded body into the form-field JSON the send path
// expects; anything that is not key=value pairs stays raw text.
func (p *curlParser) formOrRaw(body string) (entities.BodyType, string) {
	fields, ok := urlEncodedFields(body)
	if !ok {
		p.warnf("body is not urlencoded key=value pairs and was imported as raw text")
		return entities.BodyTypeRaw, body
	}
	return entities.BodyTypeForm, jsonString(fields)
}

func hasFormFile(fields []formField) bool {
	for _, f := range fields {
		if f.Type == "file" {
			return true
		}
	}
	return false
}

func urlEncodedFields(body string) ([]formField, bool) {
	segments := strings.Split(body, "&")
	fields := make([]formField, 0, len(segments))
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		idx := strings.Index(seg, "=")
		if idx <= 0 {
			return nil, false
		}
		key, err := url.QueryUnescape(seg[:idx])
		if err != nil {
			key = seg[:idx]
		}
		value, err := url.QueryUnescape(seg[idx+1:])
		if err != nil {
			value = seg[idx+1:]
		}
		fields = append(fields, formField{Key: key, Value: value, Type: "text", Enabled: true})
	}
	if len(fields) == 0 {
		return nil, false
	}
	return fields, true
}

func (p *curlParser) applyAuth(res *ParsedCurl) {
	if p.hasUser {
		user, pass := splitCredentials(p.user)
		switch {
		case p.hasSigv4:
			res.AuthType = entities.AuthTypeAWSSigV4
			res.AuthData = jsonString(p.awsAuth(user, pass))
		case p.digestFlag:
			res.AuthType = entities.AuthTypeDigest
			res.AuthData = jsonString(basicAuthJSON{Username: user, Password: pass})
		default:
			res.AuthType = entities.AuthTypeBasic
			res.AuthData = jsonString(basicAuthJSON{Username: user, Password: pass})
		}
		// index taken after the switch: awsAuth may have dropped a header before it
		if idx := headerIndex(p.headers, "Authorization"); idx >= 0 {
			p.warnf("-u credentials replaced the Authorization header")
			p.headers = append(p.headers[:idx], p.headers[idx+1:]...)
		}
		return
	}

	if p.hasSigv4 {
		p.warnf("--aws-sigv4 has no -u credentials; AWS signing was not imported")
	}
	if p.digestFlag {
		p.warnf("--digest has no -u credentials; digest auth was not imported")
	}

	idx := headerIndex(p.headers, "Authorization")
	if idx < 0 {
		return
	}

	value := strings.TrimSpace(p.headers[idx].Value)
	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "bearer "):
		res.AuthType = entities.AuthTypeBearer
		res.AuthData = jsonString(bearerAuthJSON{
			Token:  strings.TrimSpace(value[len("bearer "):]),
			Prefix: "Bearer",
		})
	case strings.HasPrefix(lower, "basic "):
		decoded, ok := decodeBasicCredentials(strings.TrimSpace(value[len("basic "):]))
		if !ok {
			p.warnf("Authorization: Basic value could not be decoded and was kept as a header")
			return
		}
		user, pass := splitCredentials(decoded)
		res.AuthType = entities.AuthTypeBasic
		res.AuthData = jsonString(basicAuthJSON{Username: user, Password: pass})
	default:
		return
	}
	p.headers = append(p.headers[:idx], p.headers[idx+1:]...)
}

// awsAuth reads region and service from curl's provider string and folds the
// session token header into the auth config, where the signer expects it.
func (p *curlParser) awsAuth(key, secret string) awsAuthJSON {
	parts := strings.Split(p.sigv4, ":")
	data := awsAuthJSON{AccessKeyID: key, SecretAccessKey: secret}
	if len(parts) > 2 {
		data.Region = parts[2]
	}
	if len(parts) > 3 {
		data.Service = parts[3]
	}
	if data.Region == "" || data.Service == "" {
		p.warnf("--aws-sigv4 %q has no region or service; set them in the Auth tab", p.sigv4)
	}
	if i := headerIndex(p.headers, "x-amz-security-token"); i >= 0 {
		data.SessionToken = p.headers[i].Value
		p.headers = append(p.headers[:i], p.headers[i+1:]...)
	}
	return data
}

func decodeBasicCredentials(raw string) (string, bool) {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(raw)
	}
	if err != nil || !utf8.Valid(decoded) || !strings.Contains(string(decoded), ":") {
		return "", false
	}
	return string(decoded), true
}

func splitCredentials(v string) (string, string) {
	if i := strings.Index(v, ":"); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

// defaultHeader adds a header only when the command did not set it explicitly.
func (p *curlParser) defaultHeader(key, value string) {
	if headerIndex(p.headers, key) < 0 {
		p.headers = append(p.headers, entities.HeaderItem{Key: key, Value: value, Enabled: true})
	}
}

func headerIndex(headers []entities.HeaderItem, key string) int {
	for i, h := range headers {
		if strings.EqualFold(h.Key, key) {
			return i
		}
	}
	return -1
}

func headerValue(headers []entities.HeaderItem, key string) string {
	if i := headerIndex(headers, key); i >= 0 {
		return headers[i].Value
	}
	return ""
}

// normalizeURL adds the scheme curl would guess; spaces are left untouched.
func (p *curlParser) normalizeURL(raw string) string {
	u := strings.TrimSpace(raw)
	switch {
	case u == "" || strings.Contains(u, "://"):
		return u
	case strings.HasPrefix(u, "//"):
		p.warnf("the URL has no scheme; http:// was assumed, as curl does")
		return "http:" + u
	default:
		p.warnf("the URL has no scheme; http:// was assumed, as curl does")
		return "http://" + u
	}
}

func appendQuery(rawURL, query string) string {
	if query == "" {
		return rawURL
	}
	// The fragment stays last: appended after it, the query would be part of the
	// fragment and never leave the client.
	base, fragment := rawURL, ""
	if i := strings.Index(rawURL, "#"); i >= 0 {
		base, fragment = rawURL[:i], rawURL[i:]
	}
	if strings.Contains(base, "?") {
		return base + "&" + query + fragment
	}
	return base + "?" + query + fragment
}

func looksLikeJSONBody(body string) bool {
	trimmed := strings.TrimSpace(body)
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return false
	}
	return json.Valid([]byte(trimmed))
}

func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func (p *curlParser) warnf(format string, args ...any) {
	p.warnings = append(p.warnings, fmt.Sprintf(format, args...))
}

func dedupeWarnings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, w := range in {
		if seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}
