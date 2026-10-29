package pages

// IdentityProfileData contains already-authorized profile values and action
// URLs. Templates own presentation; the server owns lookup and privacy policy.
type IdentityProfileData struct {
	Username, Display, Presence, Status, Bio, Country  string
	Own                                                bool
	EditURL, ChatURL, AddFriendURL, PhotosURL, BlogURL string
}

type IdentityEditField struct {
	Label, Value, InputLabel, Action string
}
