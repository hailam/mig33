package chat

import (
	"time"
)

type Message struct {
	ID           int64     `json:"id"`
	Kind         byte      `json:"kind"`
	Conversation string    `json:"conversation"`
	Sender       string    `json:"sender"`
	Body         string    `json:"body"`
	Markup       string    `json:"markup,omitempty"`
	Action       bool      `json:"action,omitempty"`
	PhotoID      int64     `json:"photo_id,omitempty"`
	Color        *int32    `json:"color,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	RequestKey   string    `json:"-"`
	RequestHash  string    `json:"-"`
}

type Group struct {
	ID      string   `json:"id"`
	Creator string   `json:"creator"`
	Members []string `json:"members"`
}

// PageQuery describes a cursor window. Callers supply an explicit limit.
type PageQuery struct {
	AfterID int64
	Limit   int
}

type HistoryQuery struct {
	PageQuery
	Kind   string
	Target string
}

type MessagePage struct {
	Messages    []Message
	NextAfterID int64
	HasMore     bool
}

type HistoryPage struct {
	MessagePage
	ReadThroughID int64
}

// MessageInput's JSON field order is retained for persisted idempotency hashes.
// Changing its representation would invalidate retries of existing requests.
type MessageInput struct {
	Kind    string `json:"kind"`
	Target  string `json:"target"`
	Body    string `json:"body"`
	Color   *int32 `json:"color,omitempty"`
	PhotoID int64  `json:"photo_id,omitempty"`
	Markup  string `json:"markup,omitempty"`
}

type MessageResult struct {
	Message  Message
	Replayed bool
}

type ReadInput struct {
	Kind      string
	Target    string
	ThroughID int64
}
