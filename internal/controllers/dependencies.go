package controllers

import (
	"context"

	"github.com/hailam/mig33/internal/adapters/legacyweb"
	"github.com/hailam/mig33/internal/adapters/nativecodec"
	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/ports"
	"github.com/hailam/mig33/internal/session"
)

type Sessions interface {
	Session(context.Context, string) (session.View, error)
	LoginBrowser(context.Context, session.BrowserLogin) (session.View, error)
	LogoutBrowser(context.Context, session.BrowserLogout) error
}

type Chat interface {
	ChatGroups(context.Context, string) ([]chat.Group, error)
	ChatHistory(context.Context, string, chat.HistoryQuery) (chat.HistoryPage, error)
	SyncChat(context.Context, string, chat.PageQuery) (chat.MessagePage, error)
	SendChatMessage(context.Context, string, string, string, chat.MessageInput) (chat.MessageResult, error)
	MarkChatRead(context.Context, string, string, chat.ReadInput) (int64, error)
}

type Contacts interface {
	ContactsSnapshot(context.Context, string) (contacts.ContactsSnapshot, error)
}

type Pages interface {
	FeaturePage(context.Context, string, legacyweb.Request) (legacyweb.Result, error)
	BrowserPage(context.Context, string, legacyweb.Request) (legacyweb.Result, error)
	OperatorDashboard(context.Context, session.OperatorDashboardRequest) (legacyweb.Result, error)
}

type Media interface {
	Media(context.Context, legacyweb.MediaRequest) (legacyweb.MediaResult, error)
}

type Language interface {
	LanguageResource(context.Context, string, string) (nativecodec.Resource, error)
}

type Operator interface {
	OperatorsEnabled(context.Context) bool
	LoginOperator(context.Context, session.OperatorLogin) (session.View, error)
	OperatorSession(context.Context, session.OperatorSessionRequest) (session.View, error)
	LogoutOperator(context.Context, session.OperatorSessionRequest) error
	ReviewOperatorItem(context.Context, session.OperatorReviewRequest) error
	ChangeOperatorCategory(context.Context, session.OperatorCategoryRequest) error
}

type Catalog func(context.Context) (catalog.CatalogSnapshot, error)

// Dependencies makes each transport dependency explicit. No controller receives
// a repository or a service locator containing unrelated application features.
type Dependencies struct {
	Sessions  Sessions
	Chat      Chat
	Contacts  Contacts
	Pages     Pages
	Media     Media
	Language  Language
	Operator  Operator
	Native    ports.NativeService
	Catalog   Catalog
	PublicURL string
}
