// Package slashcommands describes commands supported by this server for client
// suggestions. This is local product metadata, not a recovered wire registry or
// an authorization policy: the existing handlers remain authoritative.
package slashcommands

import "slices"

type Arguments string

const (
	NoArguments       Arguments = "none"
	OptionalArguments Arguments = "optional"
	RequiredArguments Arguments = "required"
)

type Scope string

const (
	AllConversations Scope = "all"
	JoinedRoom       Scope = "room"
	CommunityRoom    Scope = "community"
)

// Command contains presentation and insertion metadata. Command and Aliases
// omit the slash; Usage and Insert include it. Parent refers to a top-level ID.
// AllowMuted describes the explicit room mute gate, not every speaking policy.
// Requirements unavailable to the client must still be checked by the server.
type Command struct {
	ID          string    `json:"id"`
	Command     string    `json:"command"`
	Usage       string    `json:"usage"`
	Description string    `json:"description"`
	Insert      string    `json:"insert"`
	Arguments   Arguments `json:"arguments"`
	Scope       Scope     `json:"scope"`
	Requirement string    `json:"requirement,omitempty"`
	AllowMuted  bool      `json:"allowMuted"`
	Parent      string    `json:"parent,omitempty"`
	Aliases     []string  `json:"aliases,omitempty"`
}

const communityModerator = "Requires this community's owner or moderator"

// Catalog returns independent values in a stable display order. Its supported
// names come from chat.ExpandCommand, rooms.ParseControl and realtime.giftCommand.
func Catalog() []Command {
	result := slices.Clone(commands)
	for i := range result {
		result[i].Aliases = slices.Clone(result[i].Aliases)
	}
	return result
}

var commands = []Command{
	{ID: "me", Command: "me", Usage: "/me <action text>", Description: "Describe an action in this conversation.", Insert: "/me ", Arguments: RequiredArguments, Scope: AllConversations},
	{ID: "hug", Command: "hug", Usage: "/hug [username]", Description: "Hug everyone or someone in this conversation.", Insert: "/hug ", Arguments: OptionalArguments, Scope: AllConversations},
	{ID: "slap", Command: "slap", Usage: "/slap [username]", Description: "Slap your forehead or someone in this conversation.", Insert: "/slap ", Arguments: OptionalArguments, Scope: AllConversations},
	{ID: "wave", Command: "wave", Usage: "/wave [username]", Description: "Wave, optionally to someone in this conversation.", Insert: "/wave ", Arguments: OptionalArguments, Scope: AllConversations},
	{ID: "smile", Command: "smile", Usage: "/smile [username]", Description: "Smile, optionally at someone in this conversation.", Insert: "/smile ", Arguments: OptionalArguments, Scope: AllConversations},
	{ID: "laugh", Command: "laugh", Usage: "/laugh [username]", Description: "Laugh, optionally with someone in this conversation.", Insert: "/laugh ", Arguments: OptionalArguments, Scope: AllConversations},
	{ID: "cry", Command: "cry", Usage: "/cry [username]", Description: "Cry, optionally on someone's shoulder in this conversation.", Insert: "/cry ", Arguments: OptionalArguments, Scope: AllConversations},
	{ID: "brb", Command: "brb", Usage: "/brb [username]", Description: "Say you will be right back.", Insert: "/brb ", Arguments: OptionalArguments, Scope: AllConversations},
	{ID: "roll", Command: "roll", Usage: "/roll", Description: "Roll a random number from 1 to 100.", Insert: "/roll", Arguments: NoArguments, Scope: AllConversations},
	{ID: "fortune", Command: "fortune", Usage: "/fortune", Description: "Share a random fortune from the local response bank.", Insert: "/fortune", Arguments: NoArguments, Scope: AllConversations},
	{ID: "8ball", Command: "8ball", Usage: "/8ball", Description: "Share a random answer from the local response bank.", Insert: "/8ball", Arguments: NoArguments, Scope: AllConversations},
	{ID: "gift", Command: "gift", Usage: "/gift [subcommand or username gift]", Description: "Browse, rate or send gifts; send /gift alone for help.", Insert: "/gift ", Arguments: OptionalArguments, Scope: AllConversations, AllowMuted: true},
	{ID: "silence", Command: "silence", Usage: "/silence [on|off]", Description: "Restrict ordinary speakers in this community room.", Insert: "/silence ", Arguments: OptionalArguments, Scope: CommunityRoom, Requirement: communityModerator},
	{ID: "unsilence", Command: "unsilence", Usage: "/unsilence", Description: "Allow ordinary speakers in this community room again.", Insert: "/unsilence", Arguments: NoArguments, Scope: CommunityRoom, Requirement: communityModerator},
	{ID: "lock", Command: "lock", Usage: "/lock [on|off]", Description: "Restrict new entry to approved community members and moderators.", Insert: "/lock ", Arguments: OptionalArguments, Scope: CommunityRoom, Requirement: communityModerator},
	{ID: "unlock", Command: "unlock", Usage: "/unlock", Description: "Remove this community room's entry restriction.", Insert: "/unlock", Arguments: NoArguments, Scope: CommunityRoom, Requirement: communityModerator},
	{ID: "announce", Command: "announce", Usage: "/announce <text>", Description: "Announce up to 1000 UTF-8 bytes in this room.", Insert: "/announce ", Arguments: RequiredArguments, Scope: CommunityRoom, Requirement: communityModerator},
	{ID: "broadcast", Command: "broadcast", Usage: "/broadcast <text>", Description: "Announce up to 1000 UTF-8 bytes in all linked community rooms.", Insert: "/broadcast ", Arguments: RequiredArguments, Scope: CommunityRoom, Requirement: communityModerator},
	{ID: "ban", Command: "ban", Usage: "/ban <username>", Description: "Ban another user from all linked community rooms.", Insert: "/ban ", Arguments: RequiredArguments, Scope: CommunityRoom, Requirement: communityModerator},
	{ID: "unban", Command: "unban", Usage: "/unban <username>", Description: "Remove another user's ban from linked community rooms.", Insert: "/unban ", Arguments: RequiredArguments, Scope: CommunityRoom, Requirement: communityModerator},
	{ID: "description", Command: "description", Usage: "/description <text>", Description: "Set this room's description, up to 1000 UTF-8 bytes.", Insert: "/description ", Arguments: RequiredArguments, Scope: CommunityRoom, Requirement: communityModerator},
	{ID: "invite", Command: "invite", Usage: "/invite <username>", Description: "Invite another registered user to this community room by Mail.", Insert: "/invite ", Arguments: RequiredArguments, Scope: CommunityRoom, Requirement: communityModerator},
	{ID: "bot", Command: "bot", Usage: "/bot <dice|8ball|stop>", Description: "Start or stop a room bot; games last 20 rounds or 30 minutes.", Insert: "/bot ", Arguments: RequiredArguments, Scope: CommunityRoom, Requirement: communityModerator},
	{ID: "botstop", Command: "botstop", Usage: "/botstop", Description: "Stop this community room's bot.", Insert: "/botstop", Arguments: NoArguments, Scope: CommunityRoom, Requirement: communityModerator, Aliases: []string{"bot stop"}},
	{ID: "play", Command: "play", Usage: "/play [question]", Description: "Play dice without arguments, or ask the room's 8ball a question.", Insert: "/play ", Arguments: OptionalArguments, Scope: CommunityRoom, Requirement: "Requires an active room bot and permission to speak"},
	{ID: "scores", Command: "scores", Usage: "/scores", Description: "View this community room's local dice leaderboard.", Insert: "/scores", Arguments: NoArguments, Scope: CommunityRoom},
	{ID: "games", Command: "games", Usage: "/games", Description: "Browse the local games catalog in any joined room.", Insert: "/games", Arguments: NoArguments, Scope: JoinedRoom, AllowMuted: true},

	{ID: "gift-list", Command: "gift list", Usage: "/gift list", Description: "List gifts in local alphabetical order.", Insert: "/gift list", Arguments: NoArguments, Scope: AllConversations, AllowMuted: true, Parent: "gift"},
	{ID: "gift-new", Command: "gift new", Usage: "/gift new", Description: "Browse the local catalog; historical release rankings are unavailable.", Insert: "/gift new", Arguments: NoArguments, Scope: AllConversations, AllowMuted: true, Parent: "gift"},
	{ID: "gift-featured", Command: "gift featured", Usage: "/gift featured", Description: "Browse the local catalog; historical featured rankings are unavailable.", Insert: "/gift featured", Arguments: NoArguments, Scope: AllConversations, AllowMuted: true, Parent: "gift"},
	{ID: "gift-popular", Command: "gift popular", Usage: "/gift popular", Description: "Browse the local catalog; historical popularity rankings are unavailable.", Insert: "/gift popular", Arguments: NoArguments, Scope: AllConversations, AllowMuted: true, Parent: "gift"},
	{ID: "gift-search", Command: "gift search", Usage: "/gift search <text>", Description: "Find gifts by name or description, using up to 120 UTF-8 bytes.", Insert: "/gift search ", Arguments: RequiredArguments, Scope: AllConversations, AllowMuted: true, Parent: "gift"},
	{ID: "gift-display", Command: "gift display", Usage: "/gift display <gift name or ID>", Description: "View a gift's details, price and local rating.", Insert: "/gift display ", Arguments: RequiredArguments, Scope: AllConversations, AllowMuted: true, Parent: "gift"},
	{ID: "gift-category", Command: "gift category", Usage: "/gift category [name]", Description: "List gift categories, or browse a named category.", Insert: "/gift category ", Arguments: OptionalArguments, Scope: AllConversations, AllowMuted: true, Parent: "gift"},
	{ID: "gift-gifts", Command: "gift gifts", Usage: "/gift gifts", Description: "View your received gifts, newest first.", Insert: "/gift gifts", Arguments: NoArguments, Scope: AllConversations, AllowMuted: true, Parent: "gift"},
	{ID: "gift-rate", Command: "gift rate", Usage: "/gift rate <gift name or ID> <1-5>", Description: "Rate a gift on the local 1 to 5 scale.", Insert: "/gift rate ", Arguments: RequiredArguments, Scope: AllConversations, AllowMuted: true, Parent: "gift", Aliases: []string{"gift rating"}},
	{ID: "gift-send", Command: "gift", Usage: "/gift <username> <gift name or ID> [-m <message>]", Description: "Send a gift using test credits; an optional note may contain 320 UTF-8 bytes.", Insert: "/gift ", Arguments: RequiredArguments, Scope: AllConversations, AllowMuted: true, Parent: "gift"},
	{ID: "bot-dice", Command: "bot dice", Usage: "/bot dice", Description: "Start the local dice bot for 20 rounds or 30 minutes.", Insert: "/bot dice", Arguments: NoArguments, Scope: CommunityRoom, Requirement: communityModerator, Parent: "bot"},
	{ID: "bot-8ball", Command: "bot 8ball", Usage: "/bot 8ball", Description: "Start the local 8ball bot for 20 rounds or 30 minutes.", Insert: "/bot 8ball", Arguments: NoArguments, Scope: CommunityRoom, Requirement: communityModerator, Parent: "bot"},
	{ID: "bot-stop", Command: "bot stop", Usage: "/bot stop", Description: "Stop this community room's bot.", Insert: "/bot stop", Arguments: NoArguments, Scope: CommunityRoom, Requirement: communityModerator, Parent: "bot", Aliases: []string{"botstop"}},
}
