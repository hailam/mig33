package events

// Payload is the closed set of typed inputs accepted at this boundary.
type Payload interface{ notification() }

func (Acknowledged) notification()           {}
func (Activity) notification()               {}
func (Alert) notification()                  {}
func (Authenticated) notification()          {}
func (Avatar) notification()                 {}
func (Captcha) notification()                {}
func (Challenge) notification()              {}
func (ColorPalette) notification()           {}
func (Contact) notification()                {}
func (ContactCheckpoint) notification()      {}
func (ContactGroup) notification()           {}
func (ContactPresence) notification()        {}
func (ContactRemoved) notification()         {}
func (ContactStatus) notification()          {}
func (ContactsComplete) notification()       {}
func (EmoticonResource) notification()       {}
func (Emoticons) notification()              {}
func (GroupCreated) notification()           {}
func (GroupParticipants) notification()      {}
func (Language) notification()               {}
func (Languages) notification()              {}
func (Message) notification()                {}
func (Notice) notification()                 {}
func (OpenFeature) notification()            {}
func (Pong) notification()                   {}
func (Popup) notification()                  {}
func (Profile) notification()                {}
func (Provider) notification()               {}
func (ProviderIcons) notification()          {}
func (ProviderRequestSaved) notification()   {}
func (RegistrationComplete) notification()   {}
func (Rejected) notification()               {}
func (Resource) notification()               {}
func (Room) notification()                   {}
func (RoomCategoriesComplete) notification() {}
func (RoomCategory) notification()           {}
func (RoomKicked) notification()             {}
func (RoomListingComplete) notification()    {}
func (RoomParticipants) notification()       {}
func (Status) notification()                 {}
func (Theme) notification()                  {}
