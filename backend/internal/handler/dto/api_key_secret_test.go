package dto

import (
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAPIKeySecretMaskedByDefaultAndNested(t *testing.T) {
	for _, secret := range []string{"short", "sk-a-long-secret-that-must-not-leak"} {
		key := service.APIKey{ID: 1, Key: secret}
		out := APIKeyFromService(&key)
		require.NotEqual(t, secret, out.Key)
		require.Contains(t, out.Key, "********")
		user := UserFromService(&service.User{ID: 1, APIKeys: []service.APIKey{key}})
		raw, err := json.Marshal(user)
		require.NoError(t, err)
		require.NotContains(t, string(raw), secret)
		require.Equal(t, secret, APIKeyWithSecretFromService(&key).Key)
		require.Equal(t, secret, key.Key, "mapping must not mutate the stored key")
	}
	require.Nil(t, APIKeyWithSecretFromService(nil))
	require.Empty(t, MaskAPIKey(""))
}
