package contacts

import (
	"slices"
	"strings"

	"github.com/hailam/mig33/internal/faults"
)

// NormalizeContact validates an address-book entry and returns a canonical copy.
// It never sorts or rewrites a caller-owned external-identity slice.
func NormalizeContact(owner string, contact Contact) (Contact, error) {
	owner, err := NormalizeOwner(owner)
	if err != nil {
		return Contact{}, err
	}
	if contact.Kind == "" {
		contact.Kind = "mig33"
	}
	if contact.ID < 0 || (contact.ID == 0 && contact.Kind == "mig33") || (contact.GroupID < 1 && contact.GroupID != -1) || !ValidText(contact.Display, 512) || !ValidText(contact.Mobile, 128) {
		return Contact{}, faults.ErrInvalid
	}
	switch contact.Kind {
	case "mig33":
		target, err := NormalizeOwner(contact.Username)
		if err != nil || target == owner || len(contact.External) != 0 {
			return Contact{}, faults.ErrInvalid
		}
		contact.Username = target
	case "phone":
		if contact.Username != "" || !ValidContactMobile(contact.Mobile) || len(contact.External) != 0 {
			return Contact{}, faults.ErrInvalid
		}
	case "im":
		if contact.Username != "" || len(contact.External) == 0 || len(contact.External) > 4 {
			return Contact{}, faults.ErrInvalid
		}
		contact.External = slices.Clone(contact.External)
		slices.SortFunc(contact.External, func(a, b ContactIM) int { return int(a.Service) - int(b.Service) })
		for i, identity := range contact.External {
			if !ValidContactService(identity.Service) || !ValidText(identity.Username, 320) || strings.TrimSpace(identity.Username) != identity.Username || identity.Username == "" || (i > 0 && contact.External[i-1].Service == identity.Service) {
				return Contact{}, faults.ErrInvalid
			}
		}
	default:
		return Contact{}, faults.ErrInvalid
	}
	return contact, nil
}
