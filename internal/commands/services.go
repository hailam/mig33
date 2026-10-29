package commands

type (
	SetAvatar      struct{ Key Field[string] }
	RecordCallback struct {
		Source, Destination Field[string]
		Mode, Confirm       Field[byte]
	}
)

type (
	InviteMobile            struct{ Phone, Name Field[string] }
	ConnectDirectory        struct{ ID, Mode, Option Field[byte] }
	DisconnectDirectory     struct{ ID Field[byte] }
	RemoveDirectorySettings struct{ ID, Mode Field[byte] }
	ChooseLanguage          struct{ Code Field[string] }
	ProviderIcons           struct{ IDs []byte }
)
