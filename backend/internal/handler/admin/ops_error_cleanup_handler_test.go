package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type opsErrorCleanupCaptureRepo struct {
	service.OpsRepository
	filter *service.OpsErrorLogFilter
}

func (r *opsErrorCleanupCaptureRepo) DeleteErrorLogs(_ context.Context, filter *service.OpsErrorLogFilter) (int64, error) {
	r.filter = filter
	return 2, nil
}

func newOpsErrorCleanupTestRouter(repo *opsErrorCleanupCaptureRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.DELETE("/errors", NewOpsHandler(svc).DeleteErrorLogs)
	return router
}

func TestOpsHandlerDeleteErrorLogsAppliesErrorFacetFilters(t *testing.T) {
	repo := &opsErrorCleanupCaptureRepo{}
	router := newOpsErrorCleanupTestRouter(repo)
	params := url.Values{
		"start_time":   {"2026-06-01T00:00:00Z"},
		"end_time":     {"2026-06-02T00:00:00Z"},
		"view":         {"all"},
		"phase":        {"request"},
		"category":     {"cyber"},
		"status_codes": {"200"},
	}
	req := httptest.NewRequest(http.MethodDelete, "/errors?"+params.Encode(), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, repo.filter)
	require.Equal(t, "request", repo.filter.Phase)
	require.Equal(t, []string{"request"}, repo.filter.ErrorPhasesAny)
	require.Equal(t, []string{"cyber_policy"}, repo.filter.ErrorTypesAny)
	require.Equal(t, []int{200}, repo.filter.StatusCodes)
}

func TestOpsHandlerDeleteErrorLogsRejectsInvalidFacetFilters(t *testing.T) {
	for _, tc := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "category", key: "category", value: "unknown"},
		{name: "status", key: "status_codes", value: "abc"},
		{name: "empty status", key: "status_codes", value: ","},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &opsErrorCleanupCaptureRepo{}
			router := newOpsErrorCleanupTestRouter(repo)
			params := url.Values{
				"start_time": {"2026-06-01T00:00:00Z"},
				"end_time":   {"2026-06-02T00:00:00Z"},
			}
			params.Set(tc.key, tc.value)
			req := httptest.NewRequest(http.MethodDelete, "/errors?"+params.Encode(), nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Nil(t, repo.filter)
		})
	}
}
