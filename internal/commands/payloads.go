package commands

// Command is the closed set of typed inputs accepted at this boundary.
type Command interface{ command() }

func (Activate) command()                {}
func (Authenticate) command()            {}
func (Block) command()                   {}
func (Captcha) command()                 {}
func (ChooseLanguage) command()          {}
func (ConnectDirectory) command()        {}
func (Contact) command()                 {}
func (DisconnectDirectory) command()     {}
func (Group) command()                   {}
func (InviteMobile) command()            {}
func (Login) command()                   {}
func (Logout) command()                  {}
func (MediaToken) command()              {}
func (Ping) command()                    {}
func (ProviderIcons) command()           {}
func (RecordCallback) command()          {}
func (Register) command()                {}
func (RemoveDirectorySettings) command() {}
func (Room) command()                    {}
func (SendMessage) command()             {}
func (SetAvatar) command()               {}
func (SetMobile) command()               {}
func (SetPresence) command()             {}
func (SetStatus) command()               {}
func (SubscribeActivity) command()       {}
func (Telemetry) command()               {}
func (Unknown) command()                 {}
func (UploadPhoto) command()             {}
