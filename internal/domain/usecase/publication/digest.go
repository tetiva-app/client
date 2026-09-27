package publication

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
)

// ReportDigest changes whenever what the author reviewed changes: a new warning or redaction, or a
// different override. Lines are length-prefixed because auth field names in selectors may hold anything.
func ReportDigest(r Report) string {
	lines := make([]string, 0, len(r.Warnings)+len(r.Redactions)+len(r.AcceptedOverrides))
	for _, w := range r.Warnings {
		lines = append(lines, w.Selector+"\x00"+w.Rule+"\x00"+strconv.FormatBool(w.Overridden))
	}
	for _, rd := range r.Redactions {
		lines = append(lines, rd.Selector+"\x00"+rd.Category+"\x00"+strconv.FormatBool(rd.Overridden))
	}
	for _, s := range r.AcceptedOverrides {
		lines = append(lines, s+"\x00override\x00true")
	}
	slices.Sort(lines)
	h := sha256.New()
	for _, l := range lines {
		h.Write([]byte(strconv.Itoa(len(l)) + ":" + l))
	}
	return hex.EncodeToString(h.Sum(nil))
}
