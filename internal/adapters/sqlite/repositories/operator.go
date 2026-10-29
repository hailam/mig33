package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/review"
)

// OperatorReview is an append-only decision log shared by reports and support.
type OperatorReview = review.OperatorReview

// ReviewWorkItem requires an already-authorized operator. Expected status
// prevents an old review form from overwriting another operator's decision.
// The work item, legacy report history and actor audit commit atomically.
func (s *Store) ReviewWorkItem(ctx context.Context, review OperatorReview, expected string) error {
	actor, err := username(review.Actor)
	if err != nil || actor != review.Actor || review.ItemID <= 0 || !reportNonce(review.Nonce) || !validReportText(review.Note, 2000, true) {
		return ErrInvalid
	}
	if (review.Kind == "report" && review.Status != "pending" && review.Status != "reviewed" && review.Status != "dismissed") ||
		(review.Kind == "support" && review.Status != "open" && review.Status != "completed" && review.Status != "dismissed") ||
		(review.Kind != "report" && review.Kind != "support") {
		return ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	old, err := queries(tx).OperatorReviewByNonce(ctx, sqlc.OperatorReviewByNonceParams{Actor: review.Actor, Nonce: review.Nonce})
	if err == nil {
		if old.Kind != review.Kind || old.ItemID != review.ItemID || old.Status != review.Status || old.Note != review.Note {
			return ErrConflict
		}

		return queryError(ctx, tx.Commit())
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return queryError(ctx, err)
	}

	now := time.Now().UnixMilli()
	if review.Kind == "report" {
		r, err := readReport(ctx, tx, review.ItemID)
		if err != nil {
			return queryError(ctx, err)
		}
		if r.Status != expected {
			return ErrConflict
		}
		if now <= r.UpdatedAt {
			now = r.UpdatedAt + 1
		}
		if err = queries(tx).UpdateReportReview(ctx, sqlc.UpdateReportReviewParams{Status: review.Status, Note: review.Note, Now: now, ID: review.ItemID}); err != nil {
			return queryError(ctx, err)
		}
		if err = queries(tx).AddReportReview(ctx, sqlc.AddReportReviewParams{ReportID: review.ItemID, Status: review.Status, Note: review.Note, Now: now}); err != nil {
			return queryError(ctx, err)
		}
	} else {
		state, err := queries(tx).SupportItemState(ctx, review.ItemID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return queryError(ctx, err)
		}
		if state != expected {
			return ErrConflict
		}
		if err = queries(tx).UpdateSupportItemState(ctx, sqlc.UpdateSupportItemStateParams{State: review.Status, ID: review.ItemID}); err != nil {
			return queryError(ctx, err)
		}
	}
	if err = queries(tx).AddOperatorReview(ctx, sqlc.AddOperatorReviewParams{
		Kind:   review.Kind,
		ItemID: review.ItemID,
		Actor:  review.Actor,
		Status: review.Status,
		Note:   review.Note,
		Nonce:  review.Nonce,
		Now:    now,
	}); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func (s *Store) OperatorReviews(ctx context.Context, kind string, id int64) ([]OperatorReview, error) {
	if (kind != "report" && kind != "support") || id <= 0 {
		return nil, ErrInvalid
	}

	rows, err := queries(s.db).ListOperatorReviews(ctx, sqlc.ListOperatorReviewsParams{Kind: kind, ID: id})
	if err != nil {
		return nil, queryError(ctx, err)
	}

	out := make([]OperatorReview, 0, len(rows))
	for _, r := range rows {
		out = append(out, OperatorReview{
			ID:        r.ID,
			Kind:      r.Kind,
			ItemID:    r.ItemID,
			Actor:     r.Actor,
			Status:    r.Status,
			Note:      r.Note,
			CreatedAt: r.CreatedAt,
		})
	}

	return out, nil
}

func (s *Store) OperatorReports(ctx context.Context, before int64) ([]AbuseReport, error) {
	if before < 0 {
		return nil, ErrInvalid
	}

	rows, err := queries(s.db).ListOperatorReports(ctx, before)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	return reportModels(rows), nil
}

func (s *Store) OperatorSupport(ctx context.Context, before int64) ([]ServiceEntry, error) {
	if before < 0 {
		return nil, ErrInvalid
	}

	rows, err := queries(s.db).ListOperatorSupport(ctx, before)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	out := make([]ServiceEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, ServiceEntry(r))
	}

	return out, nil
}
