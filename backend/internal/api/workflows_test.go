package api

import (
	"net/http/httptest"
	"testing"

	"github.com/reekta92/wad_project/backend/internal/config"
)

func TestWorkflowOperationsRequireAuthentication(t *testing.T) {
	app := New(config.Config{Issuer: "https://identity.example/realms/exams", StaticDir: t.TempDir()}, nil)
	for _, operation := range []struct{ method, path string }{
		{"GET", "schedule/export"}, {"POST", "backups"}, {"GET", "dashboard"}, {"GET", "reports/workload"}, {"GET", "reports/availability"}, {"GET", "me/duties"}, {"GET", "me/workload"},
		{"GET", "audit-logs"}, {"GET", "audit-logs/1"}, {"POST", "unavailability-requests/1/decision"},
		{"POST", "schedule/automatic-planning"}, {"POST", "invigilator-assignments/automatic"}, {"DELETE", "schedule"}, {"PUT", "schedule/publication"},
	} {
		response, err := app.Test(httptest.NewRequest(operation.method, "/api/v1/"+operation.path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 401 {
			t.Fatalf("unprotected %s %s: %d", operation.method, operation.path, response.StatusCode)
		}
	}
}

func TestReportFiltersRejectOwnerOverridesAndBadAuditInput(t *testing.T) {
	for _, tc := range []struct{ resource, query string }{{"me/duties", "personnel_id=2"}, {"me/workload", "department_id=1"}, {"audit-logs", "actor_user_id=0"}, {"audit-logs", "action=%00"}, {"audit-logs", "entity=%ff"}, {"reports/availability", "active=true"}} {
		if _, err := resourceFilter(tc.resource, tc.query); err == nil {
			t.Errorf("accepted %s?%s", tc.resource, tc.query)
		}
	}
	f, err := resourceFilter("audit-logs", "actor_user_id=1&entity=exams&action=clear&date_from=2026-11-01&date_to=2026-11-02")
	if err != nil || f.ActorID != 1 || f.Entity != "exams" || f.Action != "clear" {
		t.Fatalf("lost audit filters: %+v %v", f, err)
	}
}
