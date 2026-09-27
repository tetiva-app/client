package example

import (
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/secrets"
)

func SuspectedSecrets(headers []entities.HeaderItem, body string) []string {
	findings := secrets.Scan(body)
	for _, h := range secrets.RedactHeaders(headers) {
		findings = append(findings, secrets.Scan(h.Value)...)
	}
	return secrets.Labels(findings)
}
