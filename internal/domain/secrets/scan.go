package secrets

import (
	"regexp"
	"sort"
)

// Finding is one likely credential in scanned text; text[Start:End] is the secret itself.
type Finding struct {
	Rule  string
	Label string
	Start int
	End   int
}

type scanRule struct {
	rule, label string
	re          *regexp.Regexp
	// group is the submatch holding the secret, 0 for the whole match.
	group int
}

var (
	pemBegin = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	pemEnd   = regexp.MustCompile(`-----END [A-Z ]*PRIVATE KEY-----`)
)

var scanRules = []scanRule{
	{"jwt", "JWT", regexp.MustCompile(`\beyJ[\w-]+\.[\w-]+\.[\w-]+`), 0},
	{"aws-access-key", "AWS access key", regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), 0},
	{"github-token", "GitHub token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36}\b`), 0},
	{"slack-token", "Slack token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}`), 0},
	{"stripe-key", "Stripe secret key", regexp.MustCompile(`\bsk_live_[A-Za-z0-9]{16,}`), 0},
	{"google-api-key", "Google API key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}`), 0},
	// No leading \b: the token sits right after "bot" in Telegram URLs.
	{"telegram-bot-token", "Telegram bot token", regexp.MustCompile(`\d{8,10}:[A-Za-z0-9_-]{35}`), 0},
	{"bearer-token", "bearer token", regexp.MustCompile(`(?i)\bbearer[ \t]+([A-Za-z0-9._~+/-]{20,}=*)`), 1},
	{"oauth-token-field", "OAuth token", regexp.MustCompile(`(?i)"(?:access|refresh|id)_?token"\s*:\s*"((?:[^"\\]|\\.)*)"`), 1},
}

// Scan reports likely credentials in text, ordered by position. A value that is only {{…}}
// references or an existing mask is not one.
func Scan(text string) []Finding {
	out := scanPrivateKeys(text)
	for _, r := range scanRules {
		for _, m := range r.re.FindAllStringSubmatchIndex(text, -1) {
			start, end := m[2*r.group], m[2*r.group+1]
			if start < 0 || isPlaceholder(text[start:end]) {
				continue
			}
			out = append(out, Finding{Rule: r.rule, Label: r.label, Start: start, End: end})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// scanPrivateKeys pairs each BEGIN with the next END in one pass; a single regex with an
// optional END rescanned the rest of the text for every unclosed header.
func scanPrivateKeys(text string) []Finding {
	ends := pemEnd.FindAllStringIndex(text, -1)
	var out []Finding
	next := 0
	for _, b := range pemBegin.FindAllStringIndex(text, -1) {
		if n := len(out); n > 0 && b[0] < out[n-1].End {
			continue
		}
		for next < len(ends) && ends[next][0] < b[1] {
			next++
		}
		end := b[1]
		if next < len(ends) {
			end = ends[next][1]
		}
		out = append(out, Finding{Rule: "private-key", Label: "private key", Start: b[0], End: end})
	}
	return out
}

// Labels names each kind of finding once, in the order they first appear.
func Labels(findings []Finding) []string {
	out := []string{}
	seen := make(map[string]bool)
	for _, f := range findings {
		if !seen[f.Label] {
			seen[f.Label] = true
			out = append(out, f.Label)
		}
	}
	return out
}

func isPlaceholder(v string) bool {
	return onlyRefs(v) || v == redactedValue || v == "[redacted]"
}
