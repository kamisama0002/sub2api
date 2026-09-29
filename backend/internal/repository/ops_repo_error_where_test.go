package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestBuildOpsErrorLogsWhere_QueryUsesQualifiedColumns(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		Query: "ACCESS_DENIED",
	}

	where, args := buildOpsErrorLogsWhere(filter)
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if !strings.Contains(where, "e.request_id ILIKE $") {
		t.Fatalf("where should include qualified request_id condition: %s", where)
	}
	if !strings.Contains(where, "e.client_request_id ILIKE $") {
		t.Fatalf("where should include qualified client_request_id condition: %s", where)
	}
	if !strings.Contains(where, "e.error_message ILIKE $") {
		t.Fatalf("where should include qualified error_message condition: %s", where)
	}
}

func TestBuildOpsErrorLogsWhere_UserQueryUsesExistsSubquery(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		UserQuery: "admin@",
	}

	where, args := buildOpsErrorLogsWhere(filter)
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if !strings.Contains(where, "EXISTS (SELECT 1 FROM users u WHERE u.id = e.user_id AND u.email ILIKE $") {
		t.Fatalf("where should include EXISTS user email condition: %s", where)
	}
}

func TestOpsRepositoryDeleteErrorLogs_RequiresTimeRange(t *testing.T) {
	db, _ := newSQLMock(t)
	repo := &opsRepository{db: db}

	_, err := repo.DeleteErrorLogs(context.Background(), &service.OpsErrorLogFilter{})
	if err == nil {
		t.Fatalf("expected missing time range error")
	}
}

func TestOpsRepositoryDeleteErrorLogs_UsesExistingFilterSemantics(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}

	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	userID := int64(12)
	accountID := int64(34)
	groupID := int64(56)
	filter := &service.OpsErrorLogFilter{
		StartTime: &start,
		EndTime:   &end,
		View:      "all",
		UserID:    &userID,
		AccountID: &accountID,
		GroupID:   &groupID,
		Model:     "gpt-5.3-codex",
	}

	mock.ExpectExec("DELETE FROM ops_error_logs e WHERE").
		WithArgs(start, end, groupID, accountID, userID, "gpt-5.3-codex").
		WillReturnResult(sqlmock.NewResult(0, 7))

	deleted, err := repo.DeleteErrorLogs(context.Background(), filter)
	if err != nil {
		t.Fatalf("DeleteErrorLogs returned error: %v", err)
	}
	if deleted != 7 {
		t.Fatalf("deleted = %d, want 7", deleted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOpsRepositoryDeleteErrorLogs_UsesErrorFacetFilters(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	filter := &service.OpsErrorLogFilter{
		StartTime:      &start,
		EndTime:        &end,
		View:           "all",
		Phase:          "request",
		StatusCodes:    []int{200},
		ErrorPhasesAny: []string{"request"},
		ErrorTypesAny:  []string{"cyber_policy"},
	}

	mock.ExpectExec(`(?s)DELETE FROM ops_error_logs e WHERE.*e.error_phase = \$3.*COALESCE\(e.upstream_status_code, e.status_code, 0\) = ANY\(\$4\).*e.error_phase = ANY\(\$5\).*e.error_type = ANY\(\$6\)`).
		WithArgs(start, end, "request", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))

	deleted, err := repo.DeleteErrorLogs(context.Background(), filter)
	if err != nil || deleted != 2 {
		t.Fatalf("DeleteErrorLogs = (%d, %v), want (2, nil)", deleted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
