package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const testAdminKeySecret = "sk-sensitive-key-never-in-audit-or-list"

type secretKeyRepo struct {
	service.APIKeyRepository
	reads  int
	params pagination.PaginationParams
	search string
	owner  int64
}

func (r *secretKeyRepo) GetByID(_ context.Context, id int64) (*service.APIKey, error) {
	r.reads++
	if id != 7 {
		return nil, service.ErrAPIKeyNotFound
	}
	return &service.APIKey{ID: 7, UserID: 42, Key: testAdminKeySecret, Name: "Team key"}, nil
}
func (r *secretKeyRepo) ListForAdmin(_ context.Context, p pagination.PaginationParams, search string, owner int64, status string) ([]service.APIKey, *pagination.PaginationResult, error) {
	r.params = p
	r.search = search
	r.owner = owner
	return []service.APIKey{{ID: 7, UserID: 42, Key: testAdminKeySecret}}, &pagination.PaginationResult{Total: 1}, nil
}

type secretUserRepo struct {
	service.UserRepository
	enabled bool
}

func (r *secretUserRepo) GetByID(context.Context, int64) (*service.User, error) {
	return &service.User{ID: 3, Role: service.RoleAdmin, TotpEnabled: r.enabled}, nil
}

func (r *secretUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

type secretTotpCache struct {
	service.TotpCache
	granted bool
	err     error
	session string
}

func (c *secretTotpCache) HasStepUpGrant(_ context.Context, _ int64, session string) (bool, error) {
	c.session = session
	return c.granted, c.err
}

type secretAuditRepo struct {
	service.AuditLogRepository
	entries []*service.AuditLog
	err     error
}

func (r *secretAuditRepo) Insert(_ context.Context, entry *service.AuditLog) error {
	if r.err != nil {
		return r.err
	}
	r.entries = append(r.entries, entry)
	return nil
}

func secretHandler(t *testing.T, enabled, granted bool, method, role string) (*gin.Engine, *secretKeyRepo, *secretTotpCache, *secretAuditRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	keys := &secretKeyRepo{}
	cache := &secretTotpCache{granted: granted}
	audit := &secretAuditRepo{}
	users := service.NewUserService(&secretUserRepo{enabled: enabled}, nil, nil, nil)
	totp := service.NewTotpService(nil, nil, cache, nil, nil, nil)
	h := NewAdminAPIKeyHandler(nil, keys, users, totp, service.NewAuditLogService(audit, nil))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 3})
		c.Set(string(middleware.ContextKeyUserRole), role)
		c.Set(middleware.ContextKeyAuthEmail, "actor@example.com")
		c.Set(middleware.ContextKeySessionID, "current-session")
		c.Set("auth_method", method)
	})
	r.GET("/api/v1/admin/api-keys", h.List)
	r.POST("/api/v1/admin/api-keys/:id/reveal", h.Reveal)
	return r, keys, cache, audit
}
func requestSecret(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestAdminAPIKeySecretRequiresVerifiedAdminSession(t *testing.T) {
	for _, tt := range []struct {
		name, method, role, code string
		enabled, granted         bool
	}{
		{"no totp", "jwt", "admin", "STEP_UP_TOTP_NOT_ENABLED", false, true},
		{"no grant", "jwt", "admin", "STEP_UP_REQUIRED", true, false},
		{"machine credential", "admin_api_key", "admin", "STEP_UP_ADMIN_API_KEY_FORBIDDEN", true, true},
		{"ordinary user", "jwt", "user", "Administrator required", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, keys, _, audit := secretHandler(t, tt.enabled, tt.granted, tt.method, tt.role)
			w := requestSecret(r, "POST", "/api/v1/admin/api-keys/7/reveal", `{"purpose":"view"}`)
			require.Equal(t, http.StatusForbidden, w.Code)
			require.Contains(t, w.Body.String(), tt.code)
			require.NotContains(t, w.Body.String(), testAdminKeySecret)
			require.Zero(t, keys.reads)
			require.Empty(t, audit.entries)
		})
	}
}
func TestAdminAPIKeySecretAuditsViewAndCopyBeforeDisclosure(t *testing.T) {
	for _, purpose := range []string{"view", "copy"} {
		t.Run(purpose, func(t *testing.T) {
			r, _, cache, audit := secretHandler(t, true, true, "jwt", "admin")
			w := requestSecret(r, "POST", "/api/v1/admin/api-keys/7/reveal", `{"purpose":"`+purpose+`"}`)
			require.Equal(t, 200, w.Code)
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			require.Contains(t, w.Body.String(), testAdminKeySecret)
			require.Equal(t, "current-session", cache.session)
			require.Len(t, audit.entries, 1)
			entry := audit.entries[0]
			require.Equal(t, "admin.api_keys."+purpose, entry.Action)
			require.Equal(t, int64(3), *entry.ActorUserID)
			require.Equal(t, int64(42), entry.Extra["target_user_id"])
			require.Equal(t, int64(7), entry.Extra["api_key_id"])
			raw, err := json.Marshal(entry)
			require.NoError(t, err)
			require.NotContains(t, string(raw), testAdminKeySecret)
		})
	}
}
func TestAdminAPIKeySecretFailsClosed(t *testing.T) {
	r, keys, cache, audit := secretHandler(t, true, true, "jwt", "admin")
	cache.err = errors.New("redis unavailable")
	w := requestSecret(r, "POST", "/api/v1/admin/api-keys/7/reveal", `{"purpose":"view"}`)
	require.Equal(t, 503, w.Code)
	require.Zero(t, keys.reads)
	cache.err = nil
	audit.err = errors.New("audit unavailable")
	w = requestSecret(r, "POST", "/api/v1/admin/api-keys/7/reveal", `{"purpose":"copy"}`)
	require.Equal(t, 503, w.Code)
	require.NotContains(t, w.Body.String(), testAdminKeySecret)
}
func TestAdminAPIKeySecretValidatesTargetAndPurpose(t *testing.T) {
	r, keys, _, _ := secretHandler(t, true, true, "jwt", "admin")
	for _, body := range []string{`{}`, `{"purpose":"download"}`} {
		require.Equal(t, 400, requestSecret(r, "POST", "/api/v1/admin/api-keys/7/reveal", body).Code)
	}
	require.Equal(t, 400, requestSecret(r, "POST", "/api/v1/admin/api-keys/0/reveal", `{"purpose":"view"}`).Code)
	require.Zero(t, keys.reads)
	require.Equal(t, 404, requestSecret(r, "POST", "/api/v1/admin/api-keys/9/reveal", `{"purpose":"view"}`).Code)
}
func TestAdminAPIKeyListMasksAndFilters(t *testing.T) {
	r, keys, _, _ := secretHandler(t, false, false, "jwt", "admin")
	w := requestSecret(r, "GET", "/api/v1/admin/api-keys?page=2&page_size=10&user_id=42&search=team", "")
	require.Equal(t, 200, w.Code)
	require.NotContains(t, w.Body.String(), testAdminKeySecret)
	require.Contains(t, w.Body.String(), "********")
	require.Equal(t, 2, keys.params.Page)
	require.Equal(t, 10, keys.params.PageSize)
	require.Equal(t, int64(42), keys.owner)
	require.Equal(t, "team", keys.search)
	require.Equal(t, 400, requestSecret(r, "GET", "/api/v1/admin/api-keys?user_id=-1", "").Code)
	r, _, _, _ = secretHandler(t, true, true, "jwt", "user")
	require.Equal(t, 403, requestSecret(r, "GET", "/api/v1/admin/api-keys", "").Code)
}
