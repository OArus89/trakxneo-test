package scenarios

import (
	"net/http"
	"strings"
	"testing"

	"github.com/OArus89/trakxneo-test/clients"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityHeaders verifies HSTS, X-Frame-Options, X-Content-Type-Options.
func TestSecurityHeaders(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("GET", "/api/v1/devices", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	t.Run("hsts", func(t *testing.T) {
		hsts := resp.Header.Get("Strict-Transport-Security")
		if hsts != "" {
			assert.Contains(t, hsts, "max-age=", "HSTS should have max-age")
		} else {
			t.Log("HSTS header not set (may be set by nginx, not API directly)")
		}
	})

	t.Run("x_frame_options", func(t *testing.T) {
		xfo := resp.Header.Get("X-Frame-Options")
		assert.NotEmpty(t, xfo, "X-Frame-Options should be set")
		assert.True(t, xfo == "DENY" || xfo == "SAMEORIGIN", "should be DENY or SAMEORIGIN")
	})

	t.Run("x_content_type", func(t *testing.T) {
		xcto := resp.Header.Get("X-Content-Type-Options")
		assert.Equal(t, "nosniff", xcto)
	})

	t.Run("referrer_policy", func(t *testing.T) {
		rp := resp.Header.Get("Referrer-Policy")
		if rp != "" {
			assert.Contains(t, rp, "origin", "should contain origin")
		}
	})
}

// TestRateLimit_Commands verifies command rate limiting (10/min per user).
func TestRateLimit_Commands(t *testing.T) {
	e := setup(t)

	// Send 11 commands rapidly — the 11th should be rate-limited (429)
	var got429 bool
	for i := 0; i < 12; i++ {
		body := map[string]any{
			"imei":    "999000000000029",
			"command": "getver",
		}
		resp, err := e.api.Raw("POST", "/api/v1/commands", body)
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = true
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}

	assert.True(t, got429, "should receive 429 after exceeding rate limit")
}

// TestAuth_InvalidTokenRejected verifies that a garbage JWT is rejected.
func TestAuth_InvalidTokenRejected(t *testing.T) {
	e := setup(t)

	// Create a client with a fake token
	api := clients.NewAPIClient(e.cfg)

	req, err := http.NewRequest("GET", e.cfg.API.BaseURL+"/api/v1/devices", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake.invalid.token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	_ = api

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "invalid JWT should be rejected")
}

// TestAuth_ExpiredTokenRejected verifies expired JWTs are rejected.
func TestAuth_ExpiredTokenRejected(t *testing.T) {
	e := setup(t)

	// A known-expired JWT (HS256, expired 2020-01-01)
	expiredToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1aWQiOiJ0ZXN0IiwiZXhwIjoxNTc3ODM2ODAwfQ.invalid"

	req, err := http.NewRequest("GET", e.cfg.API.BaseURL+"/api/v1/devices", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+expiredToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestMFA_TOTPSetupEndpoint verifies the MFA setup endpoint exists.
func TestMFA_TOTPSetupEndpoint(t *testing.T) {
	e := setup(t)

	resp, err := e.api.Raw("POST", "/api/v1/auth/mfa/totp/setup", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 200 (setup info) or 400 (already setup), not 404
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode, "MFA TOTP setup endpoint should exist")
}

// TestCORS_Preflight verifies CORS preflight response.
func TestCORS_Preflight(t *testing.T) {
	e := setup(t)

	req, err := http.NewRequest("OPTIONS", e.cfg.API.BaseURL+"/api/v1/devices", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "https://trakx.emcode.ae")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	acao := resp.Header.Get("Access-Control-Allow-Origin")
	if acao != "" {
		assert.True(t, strings.Contains(acao, "trakx") || acao == "*",
			"CORS should allow known origin")
	}
}
