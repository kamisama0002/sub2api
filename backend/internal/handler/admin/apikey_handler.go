package admin

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AdminAPIKeyHandler handles admin API key management
type AdminAPIKeyHandler struct {
	adminService service.AdminService
	keyRepo      service.APIKeyRepository
	userService  *service.UserService
	totpService  *service.TotpService
	auditService *service.AuditLogService
}

// NewAdminAPIKeyHandler creates a new admin API key handler
func NewAdminAPIKeyHandler(adminService service.AdminService, keyRepo service.APIKeyRepository, userService *service.UserService, totpService *service.TotpService, auditService *service.AuditLogService) *AdminAPIKeyHandler {
	return &AdminAPIKeyHandler{
		adminService: adminService,
		keyRepo:      keyRepo,
		userService:  userService,
		totpService:  totpService,
		auditService: auditService,
	}
}

type adminAPIKeyLister interface {
	ListForAdmin(context.Context, pagination.PaginationParams, string, int64, string) ([]service.APIKey, *pagination.PaginationResult, error)
}

func (h *AdminAPIKeyHandler) List(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if role, _ := middleware.GetUserRoleFromContext(c); role != service.RoleAdmin {
		response.Forbidden(c, "Administrator required")
		return
	}
	page, pageSize := response.ParsePagination(c)
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		response.BadRequest(c, "Search is too long")
		return
	}
	var userID int64
	if value := c.Query("user_id"); value != "" {
		var err error
		userID, err = strconv.ParseInt(value, 10, 64)
		if err != nil || userID <= 0 {
			response.BadRequest(c, "Invalid user ID")
			return
		}
	}
	lister, ok := h.keyRepo.(adminAPIKeyLister)
	if !ok {
		response.InternalError(c, "API key listing unavailable")
		return
	}
	keys, result, err := lister.ListForAdmin(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize}, search, userID, c.Query("status"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.APIKey, 0, len(keys))
	for i := range keys {
		out = append(out, *dto.APIKeyFromService(&keys[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// Reveal is intentionally a POST: each view/copy request is independently
// audited. The global optional step-up switch never disables this check.
func (h *AdminAPIKeyHandler) Reveal(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if role, _ := middleware.GetUserRoleFromContext(c); role != service.RoleAdmin {
		response.Forbidden(c, "Administrator required")
		return
	}
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || keyID <= 0 {
		response.BadRequest(c, "Invalid API key ID")
		return
	}
	var req struct {
		Purpose string `json:"purpose" binding:"required,oneof=view copy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Purpose must be view or copy")
		return
	}
	middleware.SetAuditAction(c, "admin.api_keys."+req.Purpose)
	if h.userService == nil || h.totpService == nil {
		response.InternalError(c, "Verification unavailable")
		return
	}
	if !middleware.EnforceStepUpAlways(c, h.totpService, h.userService) {
		return
	}
	key, err := h.keyRepo.GetByID(c.Request.Context(), keyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{"target_user_id": key.UserID})
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	entry := &service.AuditLog{
		ActorUserID: &subject.UserID, ActorEmail: c.GetString(middleware.ContextKeyAuthEmail),
		ActorRole: service.RoleAdmin, AuthMethod: service.AuditAuthMethodJWT,
		Action: "admin.api_keys." + req.Purpose, Method: c.Request.Method, Path: c.FullPath(),
		ClientIP: middleware.SecurityClientIP(c), UserAgent: c.Request.UserAgent(), StatusCode: 200,
		Extra: map[string]any{"api_key_id": key.ID, "target_user_id": key.UserID, "purpose": req.Purpose},
	}
	if err := h.auditService.RecordRequired(c.Request.Context(), entry); err != nil {
		response.Error(c, 503, "Audit storage unavailable; key was not disclosed")
		return
	}
	middleware.SkipAudit(c) // Successful disclosure was already durably recorded.
	response.Success(c, gin.H{"id": key.ID, "key": key.Key})
}

// AdminUpdateAPIKeyGroupRequest represents the request to update an API key.
type AdminUpdateAPIKeyGroupRequest struct {
	GroupID             *int64 `json:"group_id"`               // nil=不修改, 0=解绑, >0=绑定到目标分组
	ResetRateLimitUsage *bool  `json:"reset_rate_limit_usage"` // true=重置 5h/1d/7d 限速用量
}

// UpdateGroup handles updating an API key's admin-managed fields.
// PUT /api/v1/admin/api-keys/:id
func (h *AdminAPIKeyHandler) UpdateGroup(c *gin.Context) {
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	var req AdminUpdateAPIKeyGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	var resetKey *service.APIKey
	if req.ResetRateLimitUsage != nil && *req.ResetRateLimitUsage {
		resetKey, err = h.adminService.AdminResetAPIKeyRateLimitUsage(c.Request.Context(), keyID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	result, err := h.adminService.AdminUpdateAPIKeyGroupID(c.Request.Context(), keyID, req.GroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if resetKey != nil && req.GroupID == nil {
		result.APIKey = resetKey
	}

	resp := struct {
		APIKey                 *dto.APIKey `json:"api_key"`
		AutoGrantedGroupAccess bool        `json:"auto_granted_group_access"`
		GrantedGroupID         *int64      `json:"granted_group_id,omitempty"`
		GrantedGroupName       string      `json:"granted_group_name,omitempty"`
	}{
		APIKey:                 dto.APIKeyFromService(result.APIKey),
		AutoGrantedGroupAccess: result.AutoGrantedGroupAccess,
		GrantedGroupID:         result.GrantedGroupID,
		GrantedGroupName:       result.GrantedGroupName,
	}
	response.Success(c, resp)
}
