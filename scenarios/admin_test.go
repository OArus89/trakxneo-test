package scenarios

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdmin_ListOrgs(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/admin/organizations", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAdmin_ListUsers(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/admin/users", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAdmin_ListDevices(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/devices", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	b, _ := io.ReadAll(resp.Body)
	var devices []map[string]any
	json.Unmarshal(b, &devices)
	assert.Greater(t, len(devices), 0, "should have at least one device")
}

func TestAdmin_ListRoles(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/admin/roles", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAdmin_ListPermissions(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/admin/permissions", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	b, _ := io.ReadAll(resp.Body)
	var perms []map[string]any
	json.Unmarshal(b, &perms)
	assert.GreaterOrEqual(t, len(perms), 37, "should have at least 37 permissions")
}

func TestAdmin_Tags(t *testing.T) {
	e := setup(t)

	t.Run("list_tags", func(t *testing.T) {
		resp, err := e.api.Raw("GET", "/api/v1/admin/tags", nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
