package review

import (
	"context"

	"github.com/hailam/mig33/internal/activity"
)

// Repository is the persistence capability required by review.
// Implementations own transactions and must not expose driver or generated query types.
type Repository interface {
	ReviewWorkItem(ctx context.Context, review OperatorReview, expected string) error
	OperatorReviews(ctx context.Context, kind string, id int64) ([]OperatorReview, error)
	OperatorReports(ctx context.Context, before int64) ([]AbuseReport, error)
	OperatorSupport(ctx context.Context, before int64) ([]activity.ServiceEntry, error)
	SubmitReport(ctx context.Context, input ReportSubmission) (AbuseReport, error)
	Reports(ctx context.Context, status string, limit int) ([]AbuseReport, error)
	Report(ctx context.Context, id int64) (AbuseReport, error)
	ReviewReport(ctx context.Context, id int64, status, note string) error
	ReportReviews(ctx context.Context, id int64) ([]ReportReview, error)
}
