package store

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reekta92/wad_project/backend/internal/config"
)

func TestPendingRequestServiceAuditsOwnershipDatesAndRollback(t *testing.T) {
	f := isolatedScheduler(t)
	f.exec(t, `UPDATE app_users SET personnel_id = 1 WHERE id = $1`, f.actor)
	url, err := config.DatabaseURL("exam_invigilator", "EXAM_INVIGILATOR_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	settings.MaxConns = 1
	settings.ConnConfig.RuntimeParams["search_path"] = f.owner.Config().ConnConfig.RuntimeParams["search_path"]
	pool, err := pgxpool.NewWithConfig(f.ctx, settings)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	personnelID, sessionID := int64(1), int64(1)
	r := Requests{Pool: pool, PersonnelID: &personnelID}
	input := RequestInput{PersonnelID: 1, Date: "2026-11-03", SessionID: &sessionID, Type: "leave", Reason: "A pending request"}
	id, err := r.Create(f.ctx, f.actor, input)
	if err != nil {
		t.Fatal(err)
	}
	body, err := r.Get(f.ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	var value struct{ Date, Reason, Status string }
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	if value.Date != "2026-11-02" || value.Status != "pending" {
		t.Fatalf("session date or lifecycle was not derived: %s", body)
	}
	f.exec(t, `INSERT INTO unavailability_requests (id, personnel_id, date, type, reason) VALUES (900, 2, '2026-11-03', 'leave', 'Another owner')`)
	if _, err := r.Get(f.ctx, 900); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner read: %v", err)
	}
	if err := r.Delete(f.ctx, f.actor, 900); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner delete: %v", err)
	}
	page, err := r.List(f.ctx, ResourceFilter{Page: 99, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Data  []any
		Total int
	}
	if err := json.Unmarshal(page, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 0 || result.Total != 1 {
		t.Fatalf("incorrect scoped empty page: %s", page)
	}
	input.Reason = "Updated pending request"
	if err := r.Update(f.ctx, f.actor, id, input); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.owner.QueryRow(f.ctx, `SELECT count(*) FROM audit_logs WHERE actor_user_id = $1 AND entity = 'unavailability_requests' AND entity_id = $2`, f.actor, id).Scan(&count); err != nil || count != 2 {
		t.Fatalf("missing request audit: %d %v", count, err)
	}
	f.exec(t, `CREATE FUNCTION reject_request_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$; CREATE TRIGGER reject_request_audit BEFORE INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION reject_request_audit()`)
	before := f.fingerprint(t)
	if err := r.Delete(f.ctx, f.actor, id); err == nil {
		t.Fatal("audit failure did not stop the delete")
	}
	if f.fingerprint(t) != before {
		t.Fatal("failed audit left partial request changes")
	}
	f.exec(t, `DROP TRIGGER reject_request_audit ON audit_logs`)
	f.exec(t, `UPDATE app_users SET active = false WHERE id = $1`, f.actor)
	if err := r.Update(f.ctx, f.actor, id, input); !errors.Is(err, ErrInactiveActor) {
		t.Fatalf("disabled request actor accepted: %v", err)
	}
	f.exec(t, `UPDATE app_users SET active = true WHERE id = $1`, f.actor)
	f.exec(t, `UPDATE unavailability_requests SET status = 'rejected', decided_by = $1, decided_at = now() WHERE id = $2`, f.actor, id)
	if err := r.Delete(f.ctx, f.actor, id); err == nil {
		t.Fatal("decided request was withdrawn")
	}
	if err := pool.QueryRow(f.ctx, `SELECT count(*) FROM unavailability_requests`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("request identity leaked through pool: %d %v", count, err)
	}
}
