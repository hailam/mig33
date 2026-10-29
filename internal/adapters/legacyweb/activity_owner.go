package legacyweb

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/hailam/mig33/internal/faults"
)

// Original Buzz/Lookout links use the viewer's contact ID. The advisory pres
// value never overrides current presence or privacy.
func (s *Adapter) activityPageOwner(ctx context.Context, x *actor, u *url.URL, fallback string) (string, error) {
	q := u.Query()
	if len(q["cid"]) > 1 || len(q["username"]) > 1 || len(q["cid"]) != 0 && len(q["username"]) != 0 {
		return "", faults.ErrInvalid
	}
	if len(q["cid"]) == 0 {
		if len(q["username"]) == 1 {
			return strings.ToLower(q.Get("username")), nil
		}

		return fallback, nil
	}

	id, err := strconv.ParseInt(q.Get("cid"), 10, 32)
	if err != nil || id <= 0 {
		return "", faults.ErrInvalid
	}

	contact, err := s.contacts.Contact(ctx, x.username, int32(id))
	if err != nil {
		return "", err
	}

	return contact.Username, nil
}
