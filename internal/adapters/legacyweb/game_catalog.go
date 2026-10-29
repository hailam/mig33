package legacyweb

import (
	"github.com/a-h/templ"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
)

// GameCatalogItem separates recovered names from executable local games.
// Historical charging is descriptive evidence, never a billing instruction.

func (s *Adapter) gameCatalogContent(x *actor, selected string) (templ.Component, error) {
	action := s.featureURL("bot", "bot_list")
	if selected != "" {
		for _, item := range catalog.Games() {
			if item.ID != selected {
				continue
			}

			body := pages.Stack(pages.Heading(item.Name), serviceText(item.Description))
			if item.PlayTarget != "" {
				body = pages.Stack(body, serviceForm(action, x.csrf, "play", item.PlayTarget, "", "Play "+item.Name))
			} else {
				body = pages.Stack(body, serviceText("Reference only. Open All games to play Dice Roll or Magic 8 Ball."))
			}

			return pages.Stack(body, serviceLink(action, "All games")), nil
		}

		return nil, faults.ErrNotFound
	}

	body := pages.Stack(pages.Heading("Play now"), serviceText("Free local replacements with saved results. Room bots use /bot dice or /bot 8ball in Group-linked rooms."))
	for _, item := range catalog.Games() {
		if item.PlayTarget != "" {
			label := "Roll Dice"
			if item.PlayTarget == "8ball" {
				label = "Ask Magic 8 Ball"
			}

			body = pages.Stack(body, serviceLink(action+"&game="+item.ID, item.Name+" details"), serviceForm(action, x.csrf, "play", item.PlayTarget, "", label))
		}
	}

	body = pages.Stack(body, pages.Heading("Historical games"), serviceText("Recovered catalog names. These reference entries are unavailable for play; no credits are charged."))
	for _, item := range catalog.Games() {
		if item.PlayTarget == "" {
			body = pages.Stack(body, serviceLink(action+"&game="+item.ID, item.Name+" (reference)"))
		}
	}

	return body, nil
}
