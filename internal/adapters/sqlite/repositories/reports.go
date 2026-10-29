package repositories

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/review"
)

// ReportSubmission is supplied by an authenticated adapter. Room membership
// is session state and must be checked by that adapter; group membership is
// also checked transactionally here. Nonce identifies one submitted form.
type ReportSubmission = review.ReportSubmission

type AbuseReport = review.AbuseReport

type ReportReview = review.ReportReview

func ValidReportCategory(category string) bool { return review.ValidReportCategory(category) }

func validReportText(value string, max int, required bool) bool {
	return len(value) <= max && utf8.ValidString(value) && !strings.ContainsRune(value, '\x00') && (!required || strings.TrimSpace(value) != "")
}

func reportNonce(value string) bool {
	if len(value) != 48 {
		return false
	}

	for _, c := range value {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}

	return true
}

// SubmitReport persists a pending report and never treats it as adjudicated.
// Resubmitting the same form is idempotent; changing its content is rejected.
func (s *Store) SubmitReport(ctx context.Context, input ReportSubmission) (AbuseReport, error) {
	var err error
	input.Reporter, err = username(input.Reporter)
	if err != nil {
		return AbuseReport{}, queryError(ctx, err)
	}
	if input.Target != "" {
		input.Target, err = username(input.Target)
		if err != nil || input.Target == input.Reporter {
			return AbuseReport{}, ErrInvalid
		}
	}
	if (input.Kind != "room" && input.Kind != "group") ||
		!validReportText(input.Context, 256, true) ||
		!ValidReportCategory(input.Category) ||
		!validReportText(input.Details, 2000, true) ||
		!reportNonce(input.Nonce) {
		return AbuseReport{}, ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AbuseReport{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	existingRow, err := queries(tx).ReportByNonce(ctx, sqlc.ReportByNonceParams{Reporter: input.Reporter, Nonce: input.Nonce})
	existing := reportModel(existingRow)
	err = queryError(ctx, err)
	if err == nil {
		if existing.Kind != input.Kind || existing.Context != input.Context || existing.Target != input.Target || existing.Category != input.Category || existing.Details != input.Details {
			return AbuseReport{}, ErrConflict
		}

		return existing, queryError(ctx, tx.Commit())
	}
	if !errors.Is(err, ErrNotFound) {
		return AbuseReport{}, queryError(ctx, err)
	}
	if input.Kind == "room" {
		room, err := readRoom(ctx, tx, input.Context)
		if err != nil {
			return AbuseReport{}, queryError(ctx, err)
		}
		if room.Name != input.Context {
			return AbuseReport{}, ErrInvalid
		}
	} else {
		if err := authorizeGroup(ctx, tx, input.Context, input.Reporter); err != nil {
			return AbuseReport{}, queryError(ctx, err)
		}
		if input.Target != "" {
			if err := authorizeGroup(ctx, tx, input.Context, input.Target); err != nil {
				return AbuseReport{}, queryError(ctx, err)
			}
		}
	}

	now := time.Now().UnixMilli()
	id, err := queries(tx).CreateReport(ctx, sqlc.CreateReportParams{
		Reporter: input.Reporter,
		Kind:     input.Kind,
		Context:  input.Context,
		Target:   input.Target,
		Category: input.Category,
		Details:  input.Details,
		Nonce:    input.Nonce,
		Now:      now,
	})
	if err != nil {
		return AbuseReport{}, queryError(ctx, err)
	}

	report, err := readReport(ctx, tx, id)
	if err != nil {
		return AbuseReport{}, queryError(ctx, err)
	}

	return report, queryError(ctx, tx.Commit())
}

// Reports and review methods are trusted operator APIs, not user endpoints.
func (s *Store) Reports(ctx context.Context, status string, limit int) ([]AbuseReport, error) {
	if status != "" && status != "pending" && status != "reviewed" && status != "dismissed" {
		return nil, ErrInvalid
	}

	rows, err := queries(s.db).ListReports(ctx, sqlc.ListReportsParams{Status: status, Count: int64(boundedLimit(limit))})
	if err != nil {
		return nil, queryError(ctx, err)
	}

	return reportModels(rows), nil
}

func (s *Store) Report(ctx context.Context, id int64) (AbuseReport, error) {
	if id <= 0 {
		return AbuseReport{}, ErrInvalid
	}

	return readReport(ctx, s.db, id)
}

func (s *Store) ReviewReport(ctx context.Context, id int64, status, note string) error {
	if id <= 0 || (status != "reviewed" && status != "dismissed") || !validReportText(note, 2000, true) {
		return ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	report, err := readReport(ctx, tx, id)
	if err != nil {
		return queryError(ctx, err)
	}
	if report.Status == status && report.ReviewNote == note {
		return queryError(ctx, tx.Commit())
	}

	now := time.Now().UnixMilli()
	if now <= report.UpdatedAt {
		now = report.UpdatedAt + 1
	}
	if err := queries(tx).UpdateReportReview(ctx, sqlc.UpdateReportReviewParams{Status: status, Note: note, Now: now, ID: id}); err != nil {
		return queryError(ctx, err)
	}
	if err := queries(tx).AddReportReview(ctx, sqlc.AddReportReviewParams{ReportID: id, Status: status, Note: note, Now: now}); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func (s *Store) ReportReviews(ctx context.Context, id int64) ([]ReportReview, error) {
	if _, err := s.Report(ctx, id); err != nil {
		return nil, queryError(ctx, err)
	}

	rows, err := queries(s.db).ListReportReviews(ctx, id)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	reviews := make([]ReportReview, 0, len(rows))
	for _, row := range rows {
		reviews = append(reviews, ReportReview{ID: row.ID, Status: row.Status, Note: row.Note, CreatedAt: row.CreatedAt})
	}

	return reviews, nil
}

func reportModel(r sqlc.AbuseReport) AbuseReport {
	return AbuseReport{
		ID:         r.ID,
		Reporter:   r.Reporter,
		Kind:       r.Kind,
		Context:    r.Context,
		Target:     r.Target,
		Category:   r.Category,
		Details:    r.Details,
		Status:     r.Status,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
		ReviewNote: r.ReviewNote,
	}
}

func reportModels(rows []sqlc.AbuseReport) []AbuseReport {
	result := make([]AbuseReport, 0, len(rows))
	for _, row := range rows {
		result = append(result, reportModel(row))
	}

	return result
}

func readReport(ctx context.Context, db sqlc.DBTX, id int64) (AbuseReport, error) {
	row, err := queries(db).GetReport(ctx, id)
	return reportModel(row), queryError(ctx, err)
}
