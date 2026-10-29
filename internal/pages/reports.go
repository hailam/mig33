package pages

import (
	"net/http"
	"strconv"
)

type reportView struct {
	Action, Context string
	Hidden          []HiddenField
	Targets         []string
}

// ReportForm is a replacement for unavailable historical server HTML. Details
// is deliberately the final named field for the native legacy body decoder.
func ReportForm(action, context, csrf, nonce, binding string, targets []string) Result {
	return Render(http.StatusOK, "Report Abuse", legacyReportView(reportView{
		Action:  action,
		Context: context,
		Targets: targets,
		Hidden:  []HiddenField{{Name: "csrf", Value: csrf}, {Name: "nonce", Value: nonce}, {Name: "binding", Value: binding}},
	}))
}

func ReportNotice(status int, text string) Result {
	return Render(status, "Report Abuse", Paragraph(text))
}

func ReportSaved(id int64) Result {
	return Render(http.StatusOK, "Report Submitted", Stack(
		Paragraph("Report #"+strconv.FormatInt(id, 10)+" saved for local review."),
		Paragraph("No moderation action has been taken automatically.")))
}
