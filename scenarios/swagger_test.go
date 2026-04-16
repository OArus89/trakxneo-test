package scenarios

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwagger_UIAccessible(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/swagger/index.html", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSwagger_JSONSpec(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/swagger/doc.json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
