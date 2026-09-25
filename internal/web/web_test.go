package web

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Regression: a typo in any embedded template (layout, a page, a fragment,
// or a data-testid function that is not registered) only surfaces on the
// first request that renders it. This makes it fail at test time instead.
func TestParseAllTemplates(t *testing.T) {
	_, err := Parse()
	require.NoError(t, err)
}
