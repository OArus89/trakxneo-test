package scenarios

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommands_ListEndpoint(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/commands", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCommands_ListByIMEI(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/commands?imei=864636066073087", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
