package portability_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
)

func TestQueryPair_EscapesLiteralsAndKeepsVariables(t *testing.T) {
	assert.Equal(t, "q+{{k}}=a%26b+c%2Bd%23%C3%A9%3D{{v}}", portability.QueryPair("q {{k}}", "a&b c+d#é={{v}}"))
}

func TestQueryPair_KeepsPercentEscapesTypedAlready(t *testing.T) {
	assert.Equal(t, "k%2fx=a%20b+c%2Fd%25zz%25{{v}}%25", portability.QueryPair("k%2fx", "a%20b c%2Fd%zz%{{v}}%"))
}

func TestWithQuery_RoundTrips(t *testing.T) {
	u := portability.WithQuery("https://e.com/p?x=0#top", []string{
		portability.QueryPair("q", "a&b c+d#é"),
		portability.QueryPair("k&=", "50%"),
	})

	assert.Equal(t, "https://e.com/p?x=0&q=a%26b+c%2Bd%23%C3%A9&k%26%3D=50%25#top", u)
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	assert.Equal(t, url.Values{"x": {"0"}, "q": {"a&b c+d#é"}, "k&=": {"50%"}}, parsed.Query())
	assert.Equal(t, "top", parsed.Fragment)
	assert.Equal(t, "https://e.com/p?a=1#top", portability.WithQuery("https://e.com/p#top", []string{"a=1"}))
	assert.Equal(t, "https://e.com/p", portability.WithQuery("https://e.com/p", nil))
}
