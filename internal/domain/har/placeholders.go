package har

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultTokenPrefix = "tetivaph"
	portTokenBase      = 59990
	portTokenSlots     = 10
)

var (
	encodedVarPattern = regexp.MustCompile(`(?i)%7B%7B(.+?)%7D%7D`)
	portPattern       = regexp.MustCompile(`:(\d+)`)
	// urlUpToPort matches text that is exactly scheme://authority:port, the port digits or a kept reference.
	urlUpToPort = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://[^/?#\s]+:(\d+|\{\{[^}]+\}\})$`)
)

// Placeholders swaps unresolved {{name}} references for URL-safe tokens before
// url.Parse-based code sees them, and swaps them back afterwards.
type Placeholders struct {
	prefix    string
	tokenRe   *regexp.Regexp
	names     []string
	index     map[string]int
	freePorts []string
	ports     map[string]string // name → port token
	portNames map[string]string // port token → name
}

// NewPlaceholders picks a token prefix and port tokens absent from all texts.
func NewPlaceholders(texts ...string) *Placeholders {
	lower := make([]string, len(texts))
	for i, t := range texts {
		lower[i] = strings.ToLower(t)
	}
	prefix := defaultTokenPrefix + strings.Repeat("z", longestZRun(lower)+1)
	var free []string
	for j := range portTokenSlots {
		if port := strconv.Itoa(portTokenBase + j); !containsAny(lower, port) {
			free = append(free, port)
		}
	}
	return &Placeholders{
		prefix:    prefix,
		tokenRe:   regexp.MustCompile(`(/?)` + regexp.QuoteMeta(prefix) + `(\d+)([xs])`),
		index:     map[string]int{},
		freePorts: free,
		ports:     map[string]string{},
		portNames: map[string]string{},
	}
}

// Protect replaces every {{name}} in s with a token unless keep(name) is true;
// a nil keep protects every reference.
func (p *Placeholders) Protect(s string, keep func(name string) bool) string {
	matches := varPattern.FindAllStringSubmatchIndex(s, -1)
	if matches == nil {
		return s
	}
	var b strings.Builder
	last := 0
	for _, m := range matches {
		b.WriteString(s[last:m[0]])
		last = m[1]
		name := s[m[2]:m[3]]
		if keep != nil && keep(name) {
			b.WriteString(s[m[0]:m[1]])
			continue
		}
		before := b.String()
		if inPortPosition(before, s[m[1]:]) {
			if port, ok := p.portToken(name); ok {
				b.WriteString(port)
				continue
			}
		}
		// url.Parse reads everything between the port and the first '/' as the port.
		if urlUpToPort.MatchString(before) {
			b.WriteString(p.pathToken(name))
			continue
		}
		b.WriteString(p.token(name))
	}
	b.WriteString(s[last:])
	return b.String()
}

// Restore turns tokens, port tokens and percent-encoded references to known
// names back into {{name}}.
func (p *Placeholders) Restore(s string) string {
	if len(p.names) == 0 && len(p.ports) == 0 {
		return s
	}
	s = p.restorePorts(s)
	s = p.tokenRe.ReplaceAllStringFunc(s, func(tok string) string {
		m := p.tokenRe.FindStringSubmatch(tok)
		i, err := strconv.Atoi(m[2])
		if err != nil || i >= len(p.names) {
			return tok
		}
		if m[3] == "s" {
			return "{{" + p.names[i] + "}}"
		}
		return m[1] + "{{" + p.names[i] + "}}"
	})
	return encodedVarPattern.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[len("%7B%7B") : len(m)-len("%7D%7D")]
		if p.known(inner) {
			return "{{" + inner + "}}"
		}
		if name, err := url.PathUnescape(inner); err == nil && p.known(name) {
			return "{{" + name + "}}"
		}
		return m
	})
}

// RestoreRequest restores every string field of r in place.
func (p *Placeholders) RestoreRequest(r *Request) {
	r.Method = p.Restore(r.Method)
	r.URL = p.Restore(r.URL)
	r.HTTPVersion = p.Restore(r.HTTPVersion)
	p.restoreNameValues(r.Headers)
	p.restoreNameValues(r.QueryString)
	if r.PostData != nil {
		r.PostData.MimeType = p.Restore(r.PostData.MimeType)
		r.PostData.Text = p.Restore(r.PostData.Text)
		for i := range r.PostData.Params {
			prm := &r.PostData.Params[i]
			prm.Name = p.Restore(prm.Name)
			prm.Value = p.Restore(prm.Value)
			prm.FileName = p.Restore(prm.FileName)
			prm.ContentType = p.Restore(prm.ContentType)
		}
	}
	r.BinaryFile = p.Restore(r.BinaryFile)
	r.AuthNote = p.Restore(r.AuthNote)
}

func (p *Placeholders) restoreNameValues(nvs []NameValue) {
	for i := range nvs {
		nvs[i].Name = p.Restore(nvs[i].Name)
		nvs[i].Value = p.Restore(nvs[i].Value)
	}
}

func (p *Placeholders) token(name string) string {
	return p.prefix + strconv.Itoa(p.nameIndex(name)) + "x"
}

// pathToken brings a '/' of its own to end the authority; Restore drops it.
func (p *Placeholders) pathToken(name string) string {
	return "/" + p.prefix + strconv.Itoa(p.nameIndex(name)) + "s"
}

func (p *Placeholders) nameIndex(name string) int {
	i, ok := p.index[name]
	if !ok {
		i = len(p.names)
		p.names = append(p.names, name)
		p.index[name] = i
	}
	return i
}

// portToken hands out numeric tokens because url.Parse rejects a non-numeric port.
func (p *Placeholders) portToken(name string) (string, bool) {
	if port, ok := p.ports[name]; ok {
		return port, true
	}
	if len(p.freePorts) == 0 {
		return "", false
	}
	port := p.freePorts[0]
	p.freePorts = p.freePorts[1:]
	p.ports[name] = port
	p.portNames[port] = name
	return port, true
}

func (p *Placeholders) restorePorts(s string) string {
	if len(p.portNames) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, m := range portPattern.FindAllStringSubmatchIndex(s, -1) {
		name, ok := p.portNames[s[m[2]:m[3]]]
		if !ok || !inPortPosition(s[:m[2]], s[m[1]:]) {
			continue
		}
		b.WriteString(s[last:m[2]])
		b.WriteString("{{" + name + "}}")
		last = m[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

func (p *Placeholders) known(name string) bool {
	_, regular := p.index[name]
	_, port := p.ports[name]
	return regular || port
}

// inPortPosition reports whether the text between before and after is the port of a URL authority:
// right after "host:" and followed by the end of the authority or by another reference.
func inPortPosition(before, after string) bool {
	if !strings.HasSuffix(before, ":") ||
		(after != "" && !strings.ContainsRune("/?#", rune(after[0])) && !strings.HasPrefix(after, "{{")) {
		return false
	}
	i := strings.LastIndex(before, "://")
	if i < 0 {
		return false
	}
	host := before[i+len("://") : len(before)-1]
	return host != "" && !strings.ContainsAny(host, "/?# \t\r\n")
}

// longestZRun is the longest z run right after defaultTokenPrefix in texts, or -1 if it never occurs.
func longestZRun(texts []string) int {
	run := -1
	for _, t := range texts {
		for i := strings.Index(t, defaultTokenPrefix); i >= 0; {
			end := i + len(defaultTokenPrefix)
			for end < len(t) && t[end] == 'z' {
				end++
			}
			run = max(run, end-i-len(defaultTokenPrefix))
			next := strings.Index(t[i+1:], defaultTokenPrefix)
			if next < 0 {
				break
			}
			i += 1 + next
		}
	}
	return run
}

func containsAny(texts []string, sub string) bool {
	for _, t := range texts {
		if strings.Contains(t, sub) {
			return true
		}
	}
	return false
}
