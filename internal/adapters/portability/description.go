package portability

import (
	"unicode/utf8"

	"github.com/tetiva-app/client/internal/domain"
)

func ClampDescription(subject Subject, s string) (string, []Warning) {
	if len(s) <= domain.MaxDescriptionLen {
		return s, nil
	}
	cut := domain.MaxDescriptionLen
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], []Warning{Warn(WarnDescriptionTruncated, subject, "max", domain.MaxDescriptionLen)}
}
