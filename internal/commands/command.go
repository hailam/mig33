// Package commands contains typed application inputs shared by transport adapters.
package commands

// Field retains presence and decoding validity separately. The native adapter
// must preserve the original distinction between absent and malformed values;
// application authorization still happens before command-specific validation.
type Field[T any] struct {
	Value   T
	Present bool
	Valid   bool
}

func Value[T any](value T) Field[T] { return Field[T]{Value: value, Present: true, Valid: true} }
func (f Field[T]) Get() (T, bool)   { return f.Value, f.Present && f.Valid }

// Request carries correlation and retry identity, independent of packet layout.
// Nonce is calculated by the originating adapter using its established contract.
type Request struct {
	Correlation int16
	Nonce       string
	Input       Command
}

type (
	Ping      struct{}
	Telemetry struct{}
	Unknown   struct{ Description string }
	Login     struct {
		Username Field[string]
		Presence Field[byte]
	}
)

type (
	Authenticate struct{ Proof Field[int32] }
	Logout       struct{}
	SetPresence  struct{ Presence Field[byte] }
	SetStatus    struct{ Status Field[string] }
	Block        struct {
		Username Field[string]
		Mode     Field[byte]
	}
)

type SendMessage struct {
	Channel                         Field[byte]
	Kind                            Field[byte]
	Content                         Field[int16]
	Target, Body                    Field[string]
	Photo                           []byte
	Color                           Field[int32]
	Markup, RequestKey, RequestHash string
}

type (
	Captcha     struct{}
	Register    struct{ Username, Password, Mobile, Challenge, Code Field[string] }
	Activate    struct{ Code Field[string] }
	SetMobile   struct{ Mobile Field[string] }
	MediaToken  struct{ Name Field[string] }
	UploadPhoto struct {
		Data    []byte
		Purpose Field[byte]
	}
)
type SubscribeActivity struct{ Mode Field[byte] }
