package repositories

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func sampleReport() ReportSubmission {
	return ReportSubmission{Reporter: "alice", Kind: "room", Context: "Lobby", Target: "bob", Category: "spam", Details: "Repeated spam & flooding + 50% noise = test", Nonce: strings.Repeat("a", 48)}
}

func TestReportPersistenceIdempotencyAndReview(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reports.sqlite")
	s, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	input := sampleReport()
	r, err := s.SubmitReport(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	if r.ID <= 0 || r.Status != "pending" || r.Reporter != "alice" || r.Details != input.Details {
		t.Fatalf("report: %+v", r)
	}
	repeated, err := s.SubmitReport(t.Context(), input)
	if err != nil || repeated.ID != r.ID {
		t.Fatalf("repeat: %+v %v", repeated, err)
	}
	input.Details = "Changed payload"
	if _, err := s.SubmitReport(t.Context(), input); !errors.Is(err, ErrConflict) {
		t.Fatalf("nonce reused: %v", err)
	}
	if err := s.ReviewReport(t.Context(), r.ID, "reviewed", "Examined chat context"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReviewReport(t.Context(), r.ID, "reviewed", "Examined chat context"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReviewReport(t.Context(), r.ID, "dismissed", "No actionable abuse found"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err = s.Report(t.Context(), r.ID)
	if err != nil || r.Status != "dismissed" || r.ReviewNote != "No actionable abuse found" {
		t.Fatalf("restart: %+v %v", r, err)
	}
	reviews, err := s.ReportReviews(t.Context(), r.ID)
	if err != nil || len(reviews) != 2 || reviews[1].CreatedAt <= reviews[0].CreatedAt {
		t.Fatalf("review trail: %+v %v", reviews, err)
	}
	pending, err := s.Reports(t.Context(), "pending", 100)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending: %+v %v", pending, err)
	}
	all, err := s.Reports(t.Context(), "", 100)
	if err != nil || len(all) != 1 {
		t.Fatalf("all: %+v %v", all, err)
	}
	if err := s.ReviewReport(t.Context(), r.ID, "pending", "Cannot reset review history"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid transition: %v", err)
	}
}

func TestConcurrentReportDuplicateCreatesOneRow(t *testing.T) {
	s, err := Open(t.Context(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var wg sync.WaitGroup
	ids := make(chan int64, 8)
	failures := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.SubmitReport(t.Context(), sampleReport())
			if err != nil {
				failures <- err
			} else {
				ids <- r.ID
			}
		}()
	}
	wg.Wait()
	close(ids)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	var first int64
	for id := range ids {
		if first == 0 {
			first = id
		}
		if id != first {
			t.Fatal("duplicate forms created different reports")
		}
	}
	reports, err := s.Reports(t.Context(), "", 100)
	if err != nil || len(reports) != 1 {
		t.Fatalf("rows: %+v %v", reports, err)
	}
}

func TestReportValidationGroupMembershipAndAtomicReview(t *testing.T) {
	s, err := Open(t.Context(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, change := range []func(*ReportSubmission){
		func(r *ReportSubmission) { r.Details = " " }, func(r *ReportSubmission) { r.Details = strings.Repeat("x", 2001) },
		func(r *ReportSubmission) { r.Details = "bad\x00" }, func(r *ReportSubmission) { r.Category = "invented" },
		func(r *ReportSubmission) { r.Context = "missing" }, func(r *ReportSubmission) { r.Kind = "private" },
		func(r *ReportSubmission) { r.Nonce = "invalid" }, func(r *ReportSubmission) { r.Target = "alice" },
	} {
		r := sampleReport()
		change(&r)
		if _, err := s.SubmitReport(t.Context(), r); err == nil {
			t.Fatalf("accepted invalid report: %+v", r)
		}
	}
	if _, err := s.CreateGroup(t.Context(), "g", "alice", []string{"bob"}); err != nil {
		t.Fatal(err)
	}
	r := sampleReport()
	r.Kind = "group"
	r.Context = "g"
	r.Reporter = "carol"
	if _, err := s.SubmitReport(t.Context(), r); !errors.Is(err, ErrNotMember) {
		t.Fatalf("outsider report: %v", err)
	}
	r.Reporter = "alice"
	r.Target = "carol"
	if _, err := s.SubmitReport(t.Context(), r); !errors.Is(err, ErrNotMember) {
		t.Fatalf("outsider target: %v", err)
	}
	r.Target = "bob"
	report, err := s.SubmitReport(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_report_review BEFORE INSERT ON abuse_report_reviews BEGIN SELECT RAISE(ABORT,'review failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := s.ReviewReport(t.Context(), report.ID, "reviewed", "Attempt"); err == nil {
		t.Fatal("expected review failure")
	}
	report, err = s.Report(t.Context(), report.ID)
	if err != nil || report.Status != "pending" || report.ReviewNote != "" {
		t.Fatalf("partial review commit: %+v %v", report, err)
	}
}

func TestReportMigrationV5PreservesExistingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v5.sqlite")
	createV1Fixture(t, path)
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := migrateV2(t.Context(), tx); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(schemaV3 + schemaV4 + schemaV5); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO personal_blocks(owner,target) VALUES ('alice','bob'); INSERT INTO room_mutes(room,username) VALUES ('Lobby','bob');`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	assertIDs(t, history(t, s, "alice", 1, "bob"), 7)
	if blocked, err := s.IsBlocked(t.Context(), "alice", "bob"); err != nil || !blocked {
		t.Fatalf("block lost: %v %v", blocked, err)
	}
	if muted, err := s.IsRoomMuted(t.Context(), "Lobby", "bob"); err != nil || !muted {
		t.Fatalf("mute lost: %v %v", muted, err)
	}
	if err := s.EnsureContacts(t.Context(), "alice", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SubmitReport(t.Context(), sampleReport()); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 18 {
		t.Fatalf("version: %d %v", version, err)
	}
}
