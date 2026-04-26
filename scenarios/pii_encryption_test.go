package scenarios

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var piiPhoneCounter int64

// uniquePhone returns a UAE-format mobile that's unique within the process —
// previously the PII tests reused literals like +971500099001, so re-runs
// hit a UNIQUE constraint and the suite stayed FAIL forever after the
// first run.
func uniquePhone() string {
	n := atomic.AddInt64(&piiPhoneCounter, 1)
	ms := time.Now().UnixMilli() % 1_000_000
	return fmt.Sprintf("+97150%07d", (ms*7+n)%10_000_000)
}

// TestPII_EncryptionRoundtrip verifies that PII fields (national_id, license)
// are encrypted at rest but returned decrypted via API.
func TestPII_EncryptionRoundtrip(t *testing.T) {
	e := setup(t)

	nationalID := "784-1985-1234567-1"
	licenseNum := "DL-2024-TESTPII"

	// 1. Create person with PII fields
	var personID string
	t.Run("create_person_with_pii", func(t *testing.T) {
		body := map[string]any{
			"phone":              uniquePhone(),
			"first_name":         "PII",
			"last_name":          "Test",
			"national_id_type":   "emirates_id",
			"national_id_number": nationalID,
			"license_number":     licenseNum,
			"license_country":    "ARE",
			"license_class":      "B",
		}
		resp, err := e.api.Raw("POST", "/api/v1/rental/persons", body)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Less(t, resp.StatusCode, 300, "should create person")

		var r map[string]any
		b, _ := io.ReadAll(resp.Body)
		json.Unmarshal(b, &r)
		personID = r["id"].(string)

		// API should return decrypted values
		assert.Equal(t, nationalID, r["national_id_number"], "national_id should be decrypted in response")
		assert.Equal(t, licenseNum, r["license_number"], "license should be decrypted in response")
	})

	// 2. GET should also return decrypted
	t.Run("get_person_decrypted", func(t *testing.T) {
		if personID == "" {
			t.Skip("no person")
		}
		resp, err := e.api.Raw("GET", "/api/v1/rental/persons/"+personID, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var r map[string]any
		b, _ := io.ReadAll(resp.Body)
		json.Unmarshal(b, &r)

		assert.Equal(t, nationalID, r["national_id_number"], "GET should decrypt national_id")
		assert.Equal(t, licenseNum, r["license_number"], "GET should decrypt license")
	})

	// 3. Verify encrypted at rest in PostgreSQL (raw column is BYTEA, not plaintext)
	t.Run("encrypted_at_rest", func(t *testing.T) {
		if personID == "" {
			t.Skip("no person")
		}
		rows, err := e.pg.Query(
			"SELECT national_id_enc IS NOT NULL as has_enc, national_id_hash, license_hash FROM rental.persons WHERE id = $1::uuid",
			personID)
		require.NoError(t, err)
		require.Len(t, rows, 1)

		// Encrypted column should exist
		assert.Equal(t, true, rows[0]["has_enc"], "national_id_enc should be non-null")

		// Hash should be 64-char hex (HMAC-SHA256)
		natHash, _ := rows[0]["national_id_hash"].(string)
		assert.Len(t, natHash, 64, "national_id_hash should be 64-char hex")

		licHash, _ := rows[0]["license_hash"].(string)
		assert.Len(t, licHash, 64, "license_hash should be 64-char hex")

		// Hash must NOT equal the plaintext
		assert.NotEqual(t, nationalID, natHash, "hash must not be the plaintext")
	})

	// 4. Update PII fields
	t.Run("update_pii", func(t *testing.T) {
		if personID == "" {
			t.Skip("no person")
		}
		newLicense := "DL-2025-UPDATED"
		body := map[string]any{
			"first_name":     "PII",
			"last_name":      "Updated",
			"license_number": newLicense,
		}
		resp, err := e.api.Raw("PUT", "/api/v1/rental/persons/"+personID, body)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Less(t, resp.StatusCode, 300)

		var r map[string]any
		b, _ := io.ReadAll(resp.Body)
		json.Unmarshal(b, &r)

		assert.Equal(t, newLicense, r["license_number"], "updated license should be returned decrypted")
	})

	// Cleanup
	t.Cleanup(func() {
		if personID != "" {
			// No delete endpoint for persons — leave it (test phone is unique)
		}
	})
}

// TestPII_EmptyFieldsHandled verifies nil PII fields don't cause errors.
func TestPII_EmptyFieldsHandled(t *testing.T) {
	e := setup(t)

	body := map[string]any{
		"phone":      uniquePhone(),
		"first_name": "NoPII",
		"last_name":  "Test",
	}
	resp, err := e.api.Raw("POST", "/api/v1/rental/persons", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Less(t, resp.StatusCode, 300, "should create person without PII")

	var r map[string]any
	b, _ := io.ReadAll(resp.Body)
	json.Unmarshal(b, &r)

	assert.Equal(t, "", r["national_id_number"], "empty national_id should return empty string")
	assert.Equal(t, "", r["license_number"], "empty license should return empty string")
}
