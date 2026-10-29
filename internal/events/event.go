// Package events contains immutable application notifications. Native codecs
// translate these typed values; business code never constructs tagged frames.
package events

import (
	"github.com/hailam/mig33/internal/chat"
)

// Handle identifies a particular delivery reservation, not merely a message.
type Handle uint64

type Event struct {
	Correlation int16
	Handle      Handle
	Data        Payload
}

func New(correlation int16, data Payload) *Event { return &Event{Correlation: correlation, Data: data} }
func Ack(correlation int16) *Event               { return New(correlation, Acknowledged{}) }
func Failure(correlation int16, message string) *Event {
	return New(correlation, Rejected{Message: message})
}

func Successful(e *Event) bool {
	if e == nil {
		return false
	}
	_, ok := e.Data.(Acknowledged)
	return ok
}

func Error(e *Event) string {
	if e == nil {
		return ""
	}
	if r, ok := e.Data.(Rejected); ok {
		return r.Message
	}
	return ""
}

type (
	Acknowledged  struct{ Message *string }
	Rejected      struct{ Message string }
	Alert         struct{ Message string }
	Pong          struct{}
	Challenge     struct{ Challenge, SessionID string }
	Authenticated struct {
		Mobile         string
		Activated      bool
		RosterRevision int32
		Status         string
	}
)

type Captcha struct {
	Challenge string
	Image     []byte
}

type (
	RegistrationComplete struct{ Username, Message string }
	Status               struct{ Text string }
	Profile              struct{ Status, Avatar *string }
	Avatar               struct {
		ContactID int32
		Key       string
	}
)

type Message struct {
	OwnMirror bool
	Message   chat.Message
	Viewer    string
	ContactID *int32
	Avatar    string
	Emoticons []string
}

type Notice struct {
	Kind         byte
	Target, Text string
}

type Resource struct {
	Token, Label string
	Type         byte
	Data         []byte
}

type (
	Emoticons struct{ Tokens []string }
	Theme     struct{ ID string }
	Language  struct{ URL string }
	Languages struct{ IDs []byte }
	Provider  struct {
		ID, Channel byte
		Name        string
	}
)

type Activity struct {
	ID, Timestamp, ImageID    int64
	Actor, Display, Text, URL string
	Icon                      []byte
}
