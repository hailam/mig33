package commands

type GroupOperation string

const (
	GroupCreate       GroupOperation = "create"
	GroupInvite       GroupOperation = "invite"
	GroupLeave        GroupOperation = "leave"
	GroupParticipants GroupOperation = "participants"
)

type Group struct {
	Operation             GroupOperation
	Conversation, Invitee Field[string]
	Service               Field[byte]
}
