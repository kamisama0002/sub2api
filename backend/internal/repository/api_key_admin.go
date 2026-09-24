package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ListForAdmin searches metadata only; searching secret material would create
// an oracle that bypasses the reveal endpoint's verification requirement.
func (r *apiKeyRepository) ListForAdmin(ctx context.Context, params pagination.PaginationParams, search string, userID int64, status string) ([]service.APIKey, *pagination.PaginationResult, error) {
	q := r.activeQuery().Where(apikey.HasUserWith(user.DeletedAtIsNil()))
	if search != "" {
		q = q.Where(apikey.Or(apikey.NameContainsFold(search), apikey.HasUserWith(user.Or(user.EmailContainsFold(search), user.UsernameContainsFold(search)))))
	}
	if userID > 0 {
		q = q.Where(apikey.UserIDEQ(userID))
	}
	if status != "" {
		q = q.Where(apikey.StatusEQ(status))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	keys, err := q.WithUser().WithGroup().Order(apikey.ByID()).Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]service.APIKey, 0, len(keys))
	for _, key := range keys {
		out = append(out, *apiKeyEntityToService(key))
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}
