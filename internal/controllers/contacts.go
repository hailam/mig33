package controllers

import (
	"net/http"

	"github.com/hailam/mig33/internal/adapters/httpapi"
)

func (c *Controller) GetContacts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path != "/api/contacts" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Use GET", http.StatusMethodNotAllowed)
		return
	}

	snapshot, err := c.contacts.ContactsSnapshot(r.Context(), r.Header.Get("sid"))
	if err != nil {
		writeError(w, err)
		return
	}

	result := httpapi.Contacts{
		Revision:   snapshot.Revision,
		Checkpoint: snapshot.Checkpoint,
		Contacts:   make([]httpapi.Contact, 0, len(snapshot.Contacts)),
		Groups:     make([]httpapi.ContactGroup, 0, len(snapshot.Groups)),
	}
	for _, contact := range snapshot.Contacts {
		entry := httpapi.Contact{
			Id:          contact.ID,
			GroupId:     contact.GroupID,
			Kind:        contact.Kind,
			Username:    optionalString(contact.Username),
			Display:     contact.Display,
			Mobile:      optionalString(contact.Mobile),
			ShareMobile: contact.ShareMobile,
			Presence:    int64(contact.Presence),
			Status:      optionalString(contact.Status),
		}
		if len(contact.External) > 0 {
			external := make([]httpapi.ContactExternal, 0, len(contact.External))
			for _, item := range contact.External {
				external = append(external, httpapi.ContactExternal{
					Service:  int64(item.Service),
					Username: item.Username,
					Presence: int64(item.Presence),
				})
			}
			entry.External = &external
		}
		result.Contacts = append(result.Contacts, entry)
	}
	for _, group := range snapshot.Groups {
		result.Groups = append(result.Groups, httpapi.ContactGroup{Id: group.ID, Name: group.Name})
	}

	writeJSON(w, http.StatusOK, result)
}

func (c *Controller) ServeContactsAPI(w http.ResponseWriter, r *http.Request) { c.GetContacts(w, r) }
