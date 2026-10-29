package rooms

import "time"

// Ballot timing remains local reconstruction policy. The occupancy bands follow
// the recovered 2010 community table documented in docs/rooms.md.
const (
	KickVoteWindow   = time.Minute
	KickVoteCooldown = time.Minute
)

func KickVotesForOccupancy(users int) int {
	switch {
	case users <= 8:
		return 3
	case users <= 11:
		return 4
	case users <= 14:
		return 5
	case users <= 17:
		return 6
	case users <= 20:
		return 7
	case users <= 24:
		return 8
	default:
		// The historical table ends at 25; larger local rooms retain its
		// final threshold instead of inventing additional historical bands.
		return 9
	}
}

func HasUser(users []string, username string) bool {
	for _, user := range users {
		if user == username {
			return true
		}
	}
	return false
}
