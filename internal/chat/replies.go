package chat

// EightBallResponses returns a copy of the replacement response bank shared by
// message actions and room bots. No historical response bank was recovered.
func EightBallResponses() []string { return append([]string(nil), eightBallReplies...) }
