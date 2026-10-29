package rooms

import (
	"strings"
	"unicode"
)

// ParseControl recognizes the recovered room control names and local aliases.
// Authorization and queue admission still precede side effects in the caller.
func ParseControl(body string) (name, args string, handled bool) {
	text := strings.TrimSpace(body)
	if !strings.HasPrefix(text, "/") {
		return "", "", false
	}
	name = text[1:]
	if split := strings.IndexFunc(name, unicode.IsSpace); split >= 0 {
		name, args = name[:split], strings.TrimSpace(name[split:])
	}
	name = strings.ToLower(name)
	if name == "bot" && strings.EqualFold(args, "stop") {
		name, args = "botstop", ""
	}
	switch name {
	case "silence", "unsilence", "lock", "unlock", "announce", "broadcast", "ban", "unban", "description", "invite", "bot", "botstop", "play", "scores", "games":
		return name, args, true
	}
	return name, args, false
}
