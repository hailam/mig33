package repositories

import (
	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/community"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/media"
	"github.com/hailam/mig33/internal/review"
	"github.com/hailam/mig33/internal/rooms"
)

// The SQLite adapter implements each capability independently. Atomic workflows
// retain one adapter-owned transaction and one generated DBTX query package.
var (
	_ accounts.Repository  = (*Store)(nil)
	_ activity.Repository  = (*Store)(nil)
	_ chat.Repository      = (*Store)(nil)
	_ commerce.Repository  = (*Store)(nil)
	_ community.Repository = (*Store)(nil)
	_ contacts.Repository  = (*Store)(nil)
	_ media.Repository     = (*Store)(nil)
	_ review.Repository    = (*Store)(nil)
	_ rooms.Repository     = (*Store)(nil)
)
