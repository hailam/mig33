package events

type (
	EmoticonResource     struct{ Token string }
	ProviderIcons        struct{ ID byte }
	ProviderRequestSaved struct{ Message, Title string }
	OpenFeature          struct{ Controller, Action string }
	ColorPalette         struct{}
	Popup                struct{ Title, Message string }
)
