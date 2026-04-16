package scenarios

import (
	"net/http"
	"testing"

	"github.com/OArus89/trakxneo-test/clients"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuth_LoginSuccess(t *testing.T) {
	e := setup(t)

	api := clients.NewAPIClient(e.cfg)
	err := api.Login(e.cfg.Auth.Username, e.cfg.Auth.Password)
	require.NoError(t, err, "login with valid credentials should succeed")
}

func TestAuth_LoginInvalidPassword(t *testing.T) {
	e := setup(t)

	api := clients.NewAPIClient(e.cfg)
	err := api.Login(e.cfg.Auth.Username, "wrong-password")
	assert.Error(t, err, "login with wrong password should fail")
}

func TestAuth_UnauthorizedWithoutToken(t *testing.T) {
	e := setup(t)

	// Create client without logging in
	api := clients.NewAPIClient(e.cfg)
	resp, err := api.Raw("GET", "/api/v1/devices", nil)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_PermissionsList(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/admin/permissions", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAuth_RolePermissions(t *testing.T) {
	e := setup(t)

	// Get roles to find a valid role ID
	t.Run("get_role_permissions", func(t *testing.T) {
		// Role ID 1 is typically superadmin
		resp, err := e.api.Raw("GET", "/api/v1/admin/roles/1/permissions", nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
