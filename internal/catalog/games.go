package catalog

import "strings"

func Games() []CatalogGame {
	items := []CatalogGame{
		{
			ID:           "local-roll",
			Name:         "Dice Roll",
			Availability: "local_replacement",
			Evidence:     "inferred_replacement",
			Description:  "A free local roll from 1 to 100. The saved result is retained when the same form is retried.",
			PlayTarget:   "roll",
		},
		{
			ID:           "local-8ball",
			Name:         "Magic 8 Ball",
			Availability: "local_replacement",
			Evidence:     "inferred_replacement",
			Description:  "A free local answer from a replacement response bank. The saved result is retained when the same form is retried.",
			PlayTarget:   "8ball",
		},
	}
	for _, item := range []struct{ id, name, charging string }{
		{"migwars", "MigWars", "reported_free"},
		{"bollywood-trivia", "Bollywood Trivia", "reported_free"},
		{"boy-friend", "Boy Friend", "reported_free"},
		{"girl-friend", "Girl Friend", "reported_free"},
		{"heads-or-tails", "Heads or Tails", "conflicting_reports"},
		{"music-trivia", "Music Trivia", "reported_free"},
		{"one", "One", "reported_free"},
		{"sports-trivia", "Sports Trivia", "reported_free"},
		{"star-trivia", "Star Trivia", "reported_free"},
		{"trivia", "Trivia", "reported_free"},
		{"vampire", "Vampire", "reported_free"},
		{"werewolf", "Werewolf", "conflicting_reports"},
		{"lowcards", "LowCards", "reported_paid"},
		{"baccarat", "Baccarat", "reported_paid"},
		{"blackjack", "Blackjack", "reported_paid"},
		{"knock-out", "Knock Out", "reported_paid"},
		{"dice", "Dice", "reported_paid"},
		{"rock-paper-scissors", "Rock Paper Scissors", "reported_paid"},
		{"russian-roulette", "Russian Roulette", "reported_paid"},
		{"football", "FootBall", "reported_paid"},
	} {
		items = append(items, CatalogGame{
			ID:                 item.id,
			Name:               item.name,
			Availability:       "reference_placeholder",
			Evidence:           "recovered_name",
			HistoricalCharging: item.charging,
			Description:        "Historical catalog reference. The original complete game rules and service are unavailable; this entry cannot be played and does not charge credits.",
			SourceURL:          "https://mig33-news.blogspot.com/p/mig33-games.html",
		})
	}
	for _, item := range []struct{ id, name, description string }{
		{"moonbase", "Moonbase", "Blue Leaf Games' multiplayer moon strategy game."},
		{"club-wars", "Club Wars", "TheMobileGamer's club-management game."},
		{"kooky-pets", "Kooky Pets", "Kooky Panda's pet-care game."},
	} {
		items = append(items, CatalogGame{
			ID:                 item.id,
			Name:               item.name,
			Availability:       "reference_placeholder",
			Evidence:           "first_party_announcement",
			HistoricalCharging: "unknown",
			Description:        item.description + " Historical partner-game reference; the external game is unavailable here and no credits are charged.",
			SourceURL:          "https://www.prnewswire.com/news-releases/mobile-community-mig33-launches-game-developer-program-announces-new-social-game-partnerships-106642023.html",
		})
	}
	return items
}

func GameNotice() string {
	names := make([]string, 0, 23)
	for _, item := range Games() {
		if item.Availability == "reference_placeholder" {
			names = append(names, item.Name)
		}
	}
	return "Playable local replacements: /bot dice and /bot 8ball in Group-linked rooms; use /play. /bot stop ends the active bot. Historical references (not playable): " + strings.Join(names, ", ") + ". Open Menu > Games for details and free local page games."
}
