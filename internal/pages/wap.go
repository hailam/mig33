package pages

import "github.com/hailam/mig33/internal/formatting"

type WAPMessage struct {
	Sender, Body, Time, Color string
	Action, Formatted         bool
	Document                  formatting.Document
}

type WAPChatData struct {
	Action, CSRF, Nonce, User, Target, Kind, Notice, NextURL string
	Joined, IsRoom, BrowserSession                           bool
	Messages                                                 []WAPMessage
}
