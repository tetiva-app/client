package publication_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

func TestSelectors_AreUniquePerEntityAndPosition(t *testing.T) {
	f := newFixture()
	folder := f.folder(f.in.Root, "Folder")
	a := f.request(folder, "Get user")
	b := f.request(folder, "Get user")
	for _, r := range []*entities.Request{a, b} {
		r.Headers = []entities.HeaderItem{
			{Key: "X-Api-Key", Value: "one", Enabled: true},
			{Key: "X-Api-Key", Value: "two", Enabled: true},
		}
	}

	_, report := f.build(t)

	headers := redactionsOf(report, "header")
	require.Len(t, headers, 4)
	seen := map[string]bool{}
	for _, rd := range headers {
		assert.False(t, seen[rd.Selector], "duplicate selector %s", rd.Selector)
		seen[rd.Selector] = true
		assert.Equal(t, "Folder / Get user / headers / X-Api-Key", rd.Path)
	}
	assert.Equal(t, f.opaque(a.ID)+"/header/0", headers[0].Selector)
	assert.Equal(t, f.opaque(a.ID)+"/header/1", headers[1].Selector)
	assert.Equal(t, f.opaque(b.ID)+"/header/0", headers[2].Selector)
}

func TestOverride_ReferencedVariableIsPublished(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.Headers = []entities.HeaderItem{{Key: "Authorization", Value: "Bearer {{demo}}", Enabled: true}}
	v := f.variable("demo", "public-demo-value", false)

	s, report := f.build(t)
	hv := hiddenVar(report, "demo")
	require.NotNil(t, hv)
	assert.Equal(t, f.varSelector(v), hv.Selector)
	assert.Equal(t, v.ID, hv.VariableID)
	assert.Equal(t, &publication.Variable{Key: "demo", Secret: true}, findVariable(s, "demo"))
	vars := redactionsOf(report, "var")
	require.Len(t, vars, 1)
	assert.True(t, vars[0].Overridable)
	assert.Equal(t, "referenced from a secret field", vars[0].Reason)

	f.in.PublishAsIs = []string{hv.Selector, "stale/var/0"}
	s, report = f.build(t)

	assert.Equal(t, &publication.Variable{Key: "demo", Value: "public-demo-value"}, findVariable(s, "demo"))
	assert.True(t, hiddenVar(report, "demo").Overridden)
	assert.True(t, redactionsOf(report, "var")[0].Overridden)
	assert.Equal(t, []string{hv.Selector}, report.AcceptedOverrides)
	assert.Equal(t, []string{"stale/var/0"}, report.IgnoredOverrides)
	assert.Contains(t, report.PublishedVars, "demo")
}

func TestOverride_VariableSelectorFollowsTheValue(t *testing.T) {
	f := newFixture()
	f.request(f.in.Root, "list").URL = "https://api.example.com/x?k={{apiKeyDemo}}"
	v := f.variable("apiKeyDemo", "demo", false)
	_, report := f.build(t)
	sel := hiddenVar(report, "apiKeyDemo").Selector
	assert.Equal(t, f.varSelector(v), sel)

	f.in.PublishAsIs = []string{sel}
	_, accepted := f.build(t)
	assert.Equal(t, []string{sel}, accepted.AcceptedOverrides)

	v.Value = "REALopaqueKEY9f8e7d6c5b4a"
	out, changed := f.marshal(t)
	assert.NotContains(t, out, "REALopaqueKEY")
	assert.Empty(t, changed.AcceptedOverrides)
	assert.Equal(t, []string{sel}, changed.IgnoredOverrides)
	assert.False(t, hiddenVar(changed, "apiKeyDemo").Overridden)
	assert.NotEqual(t, publication.ReportDigest(accepted), publication.ReportDigest(changed))
}

func TestOverride_SuspiciousVariableIsPublished(t *testing.T) {
	f := newFixture()
	f.variable("apiToken", "demo", false)
	_, report := f.build(t)
	hv := hiddenVar(report, "apiToken")
	require.NotNil(t, hv)
	assert.Equal(t, "suspicious", hv.Reason)

	f.in.PublishAsIs = []string{hv.Selector}
	s, report := f.build(t)
	assert.Equal(t, "demo", findVariable(s, "apiToken").Value)
	assert.Equal(t, []string{hv.Selector}, report.AcceptedOverrides)
}

func TestOverride_ScanWarningUnmasks(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.Description = "demo key AKIAIOSFODNN7EXAMPLE for the sandbox"

	s, report := f.build(t)
	aws := warningsOf(report, "aws-access-key")
	require.Len(t, aws, 1)
	assert.Equal(t, "AKIA…", aws[0].Excerpt)
	assert.Equal(t, "r / description", aws[0].Path)
	assert.False(t, aws[0].Overridden)
	assert.Equal(t, "demo key <redacted> for the sandbox", findRequest(s.Collection.Items, "r").Description)

	f.in.PublishAsIs = []string{aws[0].Selector}
	s, report = f.build(t)
	assert.Equal(t, r.Description, findRequest(s.Collection.Items, "r").Description)
	require.Len(t, warningsOf(report, "aws-access-key"), 1)
	assert.True(t, warningsOf(report, "aws-access-key")[0].Overridden)
	assert.Equal(t, []string{aws[0].Selector}, report.AcceptedOverrides)
}

func TestOverride_ScanSelectorFollowsTheContent(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.Description = "AKIAIOSFODNN7EXAMPLE"
	_, before := f.build(t)

	r.Description = "see AKIAIOSFODNN7EXAMPLE"
	_, moved := f.build(t)
	assert.Equal(t, before.Warnings[0].Selector, moved.Warnings[0].Selector, "same field, same secret")

	r.Description = "AKIAIOSFODNN7EXAMPL2"
	_, changed := f.build(t)
	assert.NotEqual(t, before.Warnings[0].Selector, changed.Warnings[0].Selector)
}

func TestOverride_ForbiddenCategoriesAreIgnored(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.Headers = []entities.HeaderItem{
		{Key: "Cookie", Value: "sid=abc", Enabled: true},
		{Key: "X-Api-Key", Value: "literal", Enabled: true},
	}
	r.AuthType, r.AuthData = entities.AuthTypeBearer, `{"token":"literal-token"}`
	r.BodyType, r.Body = entities.BodyTypeBinary, "/home/ivan/data.bin"
	r.PreScript = "console.log(1)"
	secret := f.variable("token", "s3cr3t", true)

	_, report := f.build(t)
	var forbidden []string
	for _, rd := range report.Redactions {
		assert.False(t, rd.Overridable, rd.Selector)
		forbidden = append(forbidden, rd.Selector)
	}
	forbidden = append(forbidden, hiddenVar(report, "token").Selector)
	require.Len(t, forbidden, 6)

	f.in.PublishAsIs = forbidden
	s, report := f.build(t)
	out := findRequest(s.Collection.Items, "r")

	assert.Len(t, out.HTTP.Headers, 1)
	assert.Equal(t, "<redacted>", out.HTTP.Headers[0].Value)
	assert.Equal(t, "", out.Auth.Fields["token"])
	assert.Equal(t, "data.bin", out.HTTP.Body.FileName)
	assert.Nil(t, out.Scripts)
	assert.Equal(t, &publication.Variable{Key: "token", Secret: true}, findVariable(s, "token"))
	assert.Empty(t, report.AcceptedOverrides)
	assert.ElementsMatch(t, forbidden, report.IgnoredOverrides)
	assert.IsIncreasing(t, report.IgnoredOverrides)
	assert.Equal(t, f.varSelector(secret), hiddenVar(report, "token").Selector)
}

func TestReportDigest_ChangesWithWarningsAndOverrides(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.Headers = []entities.HeaderItem{{Key: "Authorization", Value: "Bearer {{demo}}", Enabled: true}}
	f.variable("demo", "x", false)

	_, base := f.build(t)
	_, again := f.build(t)
	assert.Equal(t, publication.ReportDigest(base), publication.ReportDigest(again))

	r.Description = "ghp_" + "abcdefghijklmnopqrstuvwxyz0123456789"
	_, warned := f.build(t)
	assert.NotEqual(t, publication.ReportDigest(base), publication.ReportDigest(warned))

	f.in.PublishAsIs = []string{hiddenVar(warned, "demo").Selector}
	_, overridden := f.build(t)
	assert.NotEqual(t, publication.ReportDigest(warned), publication.ReportDigest(overridden))
}

func TestSuspiciousVariableHidesWhatItReferences(t *testing.T) {
	f := newFixture()
	f.variable("apiToken", "{{prefix}}-{{plain}}", false)
	plain := f.variable("plain", "CANARY-token", false)
	f.variable("prefix", "tk", false)
	f.variable("other", "value", false)

	out, report := f.marshal(t)

	assert.NotContains(t, out, "CANARY")
	require.NotNil(t, hiddenVar(report, "plain"))
	assert.Equal(t, "referenced", hiddenVar(report, "plain").Reason)
	assert.True(t, hiddenVar(report, "plain").Overridable)
	assert.Equal(t, []string{"other"}, report.PublishedVars)

	sel := hiddenVar(report, "apiToken").Selector
	f.in.PublishAsIs = []string{sel}
	_, report = f.build(t)
	assert.ElementsMatch(t, []string{"apiToken", "plain", "prefix", "other"}, report.PublishedVars,
		"published as is, it no longer hides what it references")

	plain.Value = "REALopaqueKEY9f8e7d6c5b4a"
	out, report = f.marshal(t)
	assert.NotContains(t, out, "REALopaqueKEY")
	assert.Equal(t, []string{sel}, report.IgnoredOverrides)
	assert.False(t, hiddenVar(report, "apiToken").Overridden)
}

func TestOverride_VariableSelectorFollowsWhatItReferences(t *testing.T) {
	f := newFixture()
	f.variable("apiToken", "{{a}}", false)
	f.variable("a", "{{b}}-{{a}}", false)
	b := f.variable("b", "demo", false)
	_, report := f.build(t)
	sel := hiddenVar(report, "apiToken").Selector
	f.in.PublishAsIs = []string{sel}
	_, report = f.build(t)
	require.Equal(t, []string{sel}, report.AcceptedOverrides)

	b.Value = "REALopaqueKEY9f8e7d6c5b4a"
	out, report := f.marshal(t)
	assert.NotContains(t, out, "REALopaqueKEY")
	assert.Empty(t, report.AcceptedOverrides)
	assert.NotEqual(t, sel, hiddenVar(report, "apiToken").Selector)
}
