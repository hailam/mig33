package review

type OperatorReview struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	ItemID    int64  `json:"item_id"`
	Actor     string `json:"actor"`
	Status    string `json:"status"`
	Note      string `json:"note"`
	Nonce     string `json:"-"`
	CreatedAt int64  `json:"created_at"`
}

type ReportSubmission struct {
	Reporter string
	Kind     string
	Context  string
	Target   string
	Category string
	Details  string
	Nonce    string
}

type AbuseReport struct {
	ID         int64  `json:"id"`
	Reporter   string `json:"reporter"`
	Kind       string `json:"kind"`
	Context    string `json:"context"`
	Target     string `json:"target,omitempty"`
	Category   string `json:"category"`
	Details    string `json:"details"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
	ReviewNote string `json:"review_note,omitempty"`
}

type ReportReview struct {
	ID        int64  `json:"id"`
	Status    string `json:"status"`
	Note      string `json:"note"`
	CreatedAt int64  `json:"created_at"`
}

func ValidReportCategory(category string) bool {
	return category == "harassment" || category == "spam" || category == "content" || category == "other"
}
