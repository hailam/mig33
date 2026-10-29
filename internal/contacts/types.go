package contacts

type Contact struct {
	ID          int32
	GroupID     int32
	Username    string
	Display     string
	Mobile      string
	ShareMobile bool
	Kind        string
	External    []ContactIM
}

type ContactIM struct {
	Service  byte
	Username string
}

type ContactGroup struct {
	ID   int32
	Name string
}

type ContactRoster struct {
	Revision      int32
	Checkpoint    int64
	Groups        []ContactGroup
	Contacts      []Contact
	Deleted       []int32
	DeletedGroups []int32
}

func ValidContactService(service byte) bool {
	return service == 2 || service == 3 || service == 4 || service == 6
}

func ValidContactMobile(mobile string) bool {
	if len(mobile) < 5 || len(mobile) > 16 || mobile[0] != '+' {
		return false
	}
	for _, c := range mobile[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

type ContactPresence struct {
	ID, GroupID                     int32
	Kind, Username, Display, Mobile string
	ShareMobile                     bool
	External                        []ExternalPresence
	Presence                        byte
	Status                          string
}

type ExternalPresence struct {
	Service  byte
	Username string
	Presence byte
}

type ContactGroupView struct {
	ID   int32
	Name string
}

// ContactsSnapshot is authoritative, including removals and current presence.
type ContactsSnapshot struct {
	Revision, Checkpoint int64
	Groups               []ContactGroupView
	Contacts             []ContactPresence
}
