package dto

import "github.com/Wei-Shaw/sub2api/internal/service"

// MaskAPIKey must also hide short custom keys in their entirety.
func MaskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 16 {
		return "********"
	}
	return key[:7] + "********" + key[len(key)-4:]
}

// APIKeyWithSecretFromService is only for owner-authorized key endpoints.
// All nested DTOs and administrative list/update responses use masked keys.
func APIKeyWithSecretFromService(key *service.APIKey) *APIKey {
	out := APIKeyFromService(key)
	if out != nil {
		out.Key = key.Key
	}
	return out
}
