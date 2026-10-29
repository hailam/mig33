package events

type (
	GroupCreated      struct{ ID, Peer string }
	GroupParticipants struct{ Members []string }
)
