package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"

	"github.com/hailam/mig33/cmd/internal/command"
	"github.com/hailam/mig33/cmd/internal/wiring"
	"github.com/hailam/mig33/internal/review"
)

func main() { command.Main(run) }

func run(ctx context.Context, arguments []string, stdout, stderr io.Writer) (resultErr error) {
	flags := command.NewFlags("cli", stderr)
	var action action
	flags.Set.StringVar(&action.moderator, "room-moderator", "", "grant this account a room moderator role")
	flags.Set.StringVar(&action.room, "room", "Lobby", "room for the moderator action")
	flags.Set.BoolVar(&action.revoke, "revoke", false, "revoke the moderator role")
	flags.Set.BoolVar(&action.reports, "reports", false, "list abuse reports")
	flags.Set.Int64Var(&action.reportID, "report-id", 0, "show an abuse report and review history")
	flags.Set.Int64Var(&action.reviewID, "review-report", 0, "review or dismiss an abuse report")
	flags.Set.StringVar(&action.status, "report-status", "pending", "report filter or review result")
	flags.Set.StringVar(&action.note, "report-note", "", "required note when reviewing a report")
	flags.Set.IntVar(&action.limit, "report-limit", 100, "maximum reports to list")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if err := action.validate(); err != nil {
		return err
	}
	configuration, err := flags.Load()
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	graph, cleanup, err := wiring.Open(ctx, configuration, slog.New(slog.NewTextHandler(stderr, nil)))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, cleanup()) }()
	return execute(ctx, action, graph.Review, graph.Core, stdout)
}

func execute(ctx context.Context, action action, reports ReportAdministration, moderation RoomAdministration, stdout io.Writer) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if action.reports {
		status := action.status
		if status == "all" {
			status = ""
		}
		reports, err := reports.ListReports(ctx, status, action.limit)
		if err != nil {
			return err
		}
		return encoder.Encode(reports)
	}
	if action.reviewID != 0 {
		if err := reports.ReviewReport(ctx, action.reviewID, action.status, action.note); err != nil {
			return err
		}
		action.reportID = action.reviewID
	}
	if action.reportID != 0 {
		report, reviews, err := reports.ReportDetails(ctx, action.reportID)
		if err != nil {
			return err
		}
		return encoder.Encode(map[string]any{"report": report, "reviews": reviews})
	}
	if err := moderation.SetRoomModerator(ctx, action.room, action.moderator, !action.revoke); err != nil {
		return err
	}
	return encoder.Encode(map[string]any{"room": action.room, "username": action.moderator, "moderator": !action.revoke})
}

// CLI capabilities are trusted local operations and are supplied separately.
type ReportAdministration interface {
	ListReports(context.Context, string, int) ([]review.AbuseReport, error)
	ReviewReport(context.Context, int64, string, string) error
	ReportDetails(context.Context, int64) (review.AbuseReport, []review.ReportReview, error)
}

type RoomAdministration interface {
	SetRoomModerator(context.Context, string, string, bool) error
}
