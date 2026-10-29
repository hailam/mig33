package review

import (
	"context"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/faults"
)

type SupportReader interface {
	ServiceEntryByID(context.Context, int64) (activity.ServiceEntry, error)
}

type Service struct {
	mu        sync.RWMutex
	repo      Repository
	support   SupportReader
	operators map[string]bool
}

// New accepts trusted configuration. HTTP/native callers cannot grant roles.
// Operator identities are validated during application initialization.
func New(repo Repository, support SupportReader, operators []string) *Service {
	configured := make(map[string]bool, len(operators))
	for _, name := range operators {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			configured[name] = true
		}
	}
	return &Service{repo: repo, support: support, operators: configured}
}

func (s *Service) IsOperator(actor string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.operators[actor]
}

// ConfigureOperators is trusted configuration, coordinated with session roles.
func (s *Service) ConfigureOperators(names []string) {
	next := map[string]bool{}
	for _, name := range names {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			next[name] = true
		}
	}

	s.mu.Lock()
	s.operators = next
	s.mu.Unlock()
}

// Conversation is the authorized live snapshot supplied by the coordinated
// report workflow. That workflow holds the participation boundary through Submit;
// SQLite additionally rechecks durable group membership in its transaction.
type Conversation struct {
	Reporter, Kind, Context string
	Targets                 []string
}

type SubmitCommand struct{ Target, Category, Details, Nonce string }

func (s *Service) Submit(ctx context.Context, conversation Conversation, command SubmitCommand) (AbuseReport, error) {
	if err := ctx.Err(); err != nil {
		return AbuseReport{}, err
	}
	if conversation.Reporter == "" || (conversation.Kind != "room" && conversation.Kind != "group") || conversation.Context == "" || len(conversation.Context) > 256 || !utf8.ValidString(conversation.Context) || strings.ContainsRune(conversation.Context, '\x00') {
		return AbuseReport{}, faults.ErrInvalid
	}
	if command.Target != "" && (command.Target == conversation.Reporter || !slices.Contains(conversation.Targets, command.Target)) {
		return AbuseReport{}, faults.ErrInvalid
	}
	if !ValidReportCategory(command.Category) || strings.TrimSpace(command.Details) == "" || len(command.Details) > 2000 || !utf8.ValidString(command.Details) || strings.ContainsRune(command.Details, '\x00') {
		return AbuseReport{}, faults.ErrInvalid
	}

	return s.repo.SubmitReport(ctx, ReportSubmission{
		Reporter: conversation.Reporter,
		Kind:     conversation.Kind,
		Context:  conversation.Context,
		Target:   command.Target,
		Category: command.Category,
		Details:  command.Details,
		Nonce:    command.Nonce,
	})
}

type Queue struct {
	Reports []AbuseReport
	Support []activity.ServiceEntry
}

type WorkItem struct {
	Kind, Status  string
	Report        *AbuseReport
	Support       *activity.ServiceEntry
	Decisions     []OperatorReview
	ReportHistory []ReportReview
}

func (s *Service) Queue(ctx context.Context, actor string, reportBefore, supportBefore int64) (Queue, error) {
	if !s.IsOperator(actor) {
		return Queue{}, faults.ErrForbidden
	}
	if reportBefore < 0 || supportBefore < 0 {
		return Queue{}, faults.ErrInvalid
	}

	reports, err := s.repo.OperatorReports(ctx, reportBefore)
	if err != nil {
		return Queue{}, err
	}

	support, err := s.repo.OperatorSupport(ctx, supportBefore)
	if err != nil {
		return Queue{}, err
	}
	return Queue{Reports: reports, Support: support}, nil
}

func (s *Service) Details(ctx context.Context, actor, kind string, id int64) (WorkItem, error) {
	if !s.IsOperator(actor) {
		return WorkItem{}, faults.ErrForbidden
	}
	if id <= 0 {
		return WorkItem{}, faults.ErrInvalid
	}

	result := WorkItem{Kind: kind}
	switch kind {
	case "report":
		report, err := s.repo.Report(ctx, id)
		if err != nil {
			return WorkItem{}, err
		}
		result.Report, result.Status = &report, report.Status
		history, err := s.repo.ReportReviews(ctx, id)
		if err != nil {
			return WorkItem{}, err
		}
		result.ReportHistory = history
	case "support":
		entry, err := s.support.ServiceEntryByID(ctx, id)
		if err != nil {
			return WorkItem{}, err
		}
		if entry.Kind != "support" {
			return WorkItem{}, faults.ErrNotFound
		}
		result.Support, result.Status = &entry, entry.State
	default:
		return WorkItem{}, faults.ErrInvalid
	}

	decisions, err := s.repo.OperatorReviews(ctx, kind, id)
	if err != nil {
		return WorkItem{}, err
	}
	result.Decisions = decisions
	return result, nil
}

func (s *Service) Decide(ctx context.Context, decision OperatorReview, expected string) error {
	if !s.IsOperator(decision.Actor) {
		return faults.ErrForbidden
	}
	return s.repo.ReviewWorkItem(ctx, decision, expected)
}

// The following methods are trusted local administration operations. They are
// supplied to the CLI composition only; public controllers use the checked APIs.
func (s *Service) ListReports(ctx context.Context, status string, limit int) ([]AbuseReport, error) {
	return s.repo.Reports(ctx, status, limit)
}

func (s *Service) ReviewReport(ctx context.Context, id int64, status, note string) error {
	return s.repo.ReviewReport(ctx, id, status, note)
}

func (s *Service) ReportDetails(ctx context.Context, id int64) (AbuseReport, []ReportReview, error) {
	report, err := s.repo.Report(ctx, id)
	if err != nil {
		return AbuseReport{}, nil, err
	}
	history, err := s.repo.ReportReviews(ctx, id)
	return report, history, err
}
