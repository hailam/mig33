package legacyweb

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/a-h/templ"

	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
)

// ConfigureOperators is a trusted startup setting. There are no default
// operators and ordinary users cannot grant this role through an endpoint.
func operatorForm(action, csrf string, fields []pages.HiddenField, controls templ.Component, label string) templ.Component {
	fields = append([]pages.HiddenField{{Name: "csrf", Value: csrf}}, fields...)
	return pages.Form(action, fields, controls, label)
}

func (s *Adapter) operatorDashboard(ctx context.Context, x *actor, q url.Values) pages.Result {
	parts := []templ.Component{
		pages.Heading("Operator review"),
		pages.Paragraph("Signed in as " + x.username),
		pages.Link("/operator", "Queue"),
		operatorForm("/operator/logout", x.csrf, nil, nil, "Sign out"),
	}
	kind := q.Get("kind")
	if kind != "" {
		id, err := strconv.ParseInt(q.Get("id"), 10, 64)
		if err != nil || id <= 0 || (kind != "report" && kind != "support") {
			return serviceError("Review", faults.ErrInvalid)
		}

		item, err := s.review.Details(ctx, x.username, kind, id)
		if err != nil {
			return serviceError("Review", err)
		}

		status := item.Status
		if kind == "report" {
			r := item.Report
			parts = append(
				parts,
				pages.Heading(fmt.Sprintf("Report #%d", id)),
				pages.Paragraph("Reporter: "+r.Reporter+"; target: "+r.Target),
				pages.Paragraph("Conversation: "+r.Kind+" / "+r.Context),
				pages.Paragraph("Category: "+r.Category),
				pages.Paragraph(r.Details),
			)
		} else {
			r := item.Support
			parts = append(parts,
				pages.Heading(fmt.Sprintf("Support #%d", id)),
				pages.Paragraph("From: "+r.Owner),
				pages.Paragraph(r.Body),
			)
		}

		parts = append(parts,
			pages.Paragraph("Status: "+status),
			pages.Paragraph("Decisions update this queue only; they do not automatically punish users or send external messages."),
		)
		statuses := []string{"reviewed", "dismissed", "pending"}
		if kind == "support" {
			statuses = []string{"completed", "dismissed", "open"}
		}

		for _, next := range statuses {
			parts = append(parts, operatorForm(
				"/operator/review",
				x.csrf,
				[]pages.HiddenField{
					{Name: "kind", Value: kind},
					{Name: "id", Value: strconv.FormatInt(id, 10)},
					{Name: "status", Value: next},
					{Name: "expected", Value: status},
					{Name: "nonce", Value: token()},
				},
				pages.TextArea("note", "Review note", "", 2000),
				"Mark "+next,
			))
		}

		reviews := item.Decisions

		parts = append(parts, pages.Heading("Decision history"))
		for _, r := range reviews {
			parts = append(parts, pages.Paragraph(fmt.Sprintf("%s — %s — %s", time.UnixMilli(r.CreatedAt).UTC().Format(time.RFC3339), r.Actor, r.Status)), pages.Paragraph(r.Note))
		}
		if kind == "report" {
			legacy := item.ReportHistory

			parts = append(parts, pages.Heading("Complete report status history (including CLI)"))
			for _, r := range legacy {
				parts = append(parts, pages.Paragraph(fmt.Sprintf("%s: %s", r.Status, r.Note)))
			}
		}

		return pages.Render(200, "Operator review", pages.Stack(parts...))
	}

	var reportBefore, supportBefore int64
	for key, target := range map[string]*int64{"report_before": &reportBefore, "support_before": &supportBefore} {
		if raw := q.Get(key); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || value < 0 {
				return serviceError("Review", faults.ErrInvalid)
			}

			*target = value
		}
	}

	queue, err := s.review.Queue(ctx, x.username, reportBefore, supportBefore)
	if err != nil {
		return serviceError("Review", err)
	}

	reports := queue.Reports

	parts = append(parts, pages.Heading("Abuse reports — 100 per page"))
	for _, r := range reports {
		parts = append(parts, pages.Link(fmt.Sprintf("/operator?kind=report&id=%d", r.ID), fmt.Sprintf("#%d %s — %s — %s", r.ID, r.Status, r.Reporter, r.Category)))
	}
	if len(reports) == 100 {
		parts = append(parts, pages.Link(fmt.Sprintf("/operator?report_before=%d", reports[len(reports)-1].ID), "Older reports"))
	}

	support := queue.Support

	parts = append(parts, pages.Heading("Support requests — 100 per page"))
	for _, r := range support {
		parts = append(parts, pages.Link(fmt.Sprintf("/operator?kind=support&id=%d", r.ID), fmt.Sprintf("#%d %s — %s", r.ID, r.State, r.Owner)))
	}
	if len(support) == 100 {
		parts = append(parts, pages.Link(fmt.Sprintf("/operator?support_before=%d", support[len(support)-1].ID), "Older support requests"))
	}

	categories, err := s.rooms.RoomCategories(ctx)
	if err != nil {
		return serviceError("Categories", err)
	}

	parts = append(parts, pages.Heading("Room categories"))
	for _, c := range categories {
		parts = append(parts, operatorForm(
			"/operator/categories",
			x.csrf,
			[]pages.HiddenField{{Name: "id", Value: strconv.Itoa(int(c.ID))}, {Name: "op", Value: "save"}},
			pages.Input("name", fmt.Sprintf("Category %d", c.ID), "text", c.Name, 10),
			"Save category",
		))
		if c.ID > 2 {
			parts = append(parts, operatorForm("/operator/categories", x.csrf, []pages.HiddenField{{Name: "id", Value: strconv.Itoa(int(c.ID))}, {Name: "op", Value: "delete"}}, nil, "Delete empty category"))
		}
	}

	parts = append(parts, operatorForm(
		"/operator/categories",
		x.csrf,
		[]pages.HiddenField{{Name: "op", Value: "save"}},
		pages.Stack(pages.Input("id", "New category ID (3–32767)", "text", "", 10), pages.Input("name", "Name", "text", "", 10)),
		"Add category",
	))
	parts = append(parts, operatorForm(
		"/operator/categories",
		x.csrf,
		[]pages.HiddenField{{Name: "op", Value: "assign"}},
		pages.Stack(pages.Input("id", "Category ID", "text", "1", 10), pages.Input("room", "Existing room name", "text", "", 10)),
		"Assign room to category",
	))
	return pages.Render(200, "Operator review", pages.Stack(parts...))
}
