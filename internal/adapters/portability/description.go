package portability

import (
	"fmt"
	"unicode/utf8"

	"github.com/tetiva-app/client/internal/domain"
)

func ClampDescription(label, s string) (text, warning string) {
	if len(s) <= domain.MaxDescriptionLen {
		return s, ""
	}
	cut := domain.MaxDescriptionLen
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], fmt.Sprintf("%s: description longer than %d bytes was truncated", label, domain.MaxDescriptionLen)
}
