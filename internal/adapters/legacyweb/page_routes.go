package legacyweb

import (
	"context"
	"net/url"
	"sort"

	"github.com/hailam/mig33/internal/pages"
)

type (
	pageHandler func(*Adapter, context.Context, *actor, string, *url.URL, url.Values) pages.Result
	pageRoute   struct {
		family string
		public bool
		handle pageHandler
	}
)

// legacyPageRoutes is the authoritative registration of implemented selector
// routes. Both browser forms and native opcode922 dispatch through this table.
// Feature handlers retain their method, form and permission validation.
var legacyPageRoutes = buildPageRoutes()

func buildPageRoutes() map[string]pageRoute {
	routes := make(map[string]pageRoute)
	add := func(family string, public bool, handler pageHandler, keys ...string) {
		for _, key := range keys {
			if _, exists := routes[key]; exists {
				panic("duplicate legacy page route: " + key)
			}

			routes[key] = pageRoute{family, public, handler}
		}
	}
	add("identity", true, (*Adapter).identityPage, "registration_api/check_username", "registration/home", "account/recover")
	add(
		"identity",
		false,
		(*Adapter).identityPage,
		"profile/home",
		"profile/edit",
		"settings/change_password",
		"settings/change_mobile",
		"settings/chat_privacy",
		"settings/profile_privacy_settings",
		"settings/view_update_settings",
		"settings/call_by_name",
		"settings/change_tab_order",
	)
	optional := func(handler func(*Adapter, context.Context, *actor, string, *url.URL, url.Values) (pages.Result, bool)) pageHandler {
		return func(s *Adapter, ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
			if result, handled := handler(s, ctx, x, method, u, form); handled {
				return result
			}

			return s.catalogPage(ctx, x, method, u)
		}
	}
	add(
		"community",
		false,
		optional((*Adapter).communityPage),
		"group/home",
		"group/add_to_group",
		"group/list_chatrooms",
		"group/list_groups_home",
		"group/list_my_groups",
		"group/members",
		"group/view_forum",
	)
	add(
		"commerce",
		false,
		optional((*Adapter).commercePage),
		"account/home",
		"account/get_history",
		"account/recharge_credit",
		"account/transfer_credit",
		"store/home",
		"store/main",
		"merchant/dashboard",
	)
	for key := range serviceRoutes {
		add("service", false, optional((*Adapter).servicePage), key)
	}

	add("friends", false, (*Adapter).friendSearchPage, "profile/search")
	add("rooms", false, (*Adapter).roomPage, "chatroom/setup", "chatroom/search", "chatroom/create")
	add("reports", false, (*Adapter).reportPage, "report/report_abuse")
	add("blocking", false, func(s *Adapter, ctx context.Context, x *actor, method string, _ *url.URL, form url.Values) pages.Result {
		return s.blockListPage(ctx, x, method, form)
	}, "settings/block_list")
	add("catalog", false, func(s *Adapter, ctx context.Context, x *actor, method string, u *url.URL, _ url.Values) pages.Result {
		return s.catalogPage(ctx, x, method, u)
	}, "help/main")
	return routes
}

func lookupPageRoute(u *url.URL) (pageRoute, bool) {
	c, a, valid := identitySelectors(u)
	if !valid {
		return pageRoute{}, false
	}

	route, ok := legacyPageRoutes[c+"/"+a]
	return route, ok
}

// LegacyRoute describes a real query-based route without inventing HTTP paths.
type LegacyRoute struct {
	Selector, Family string
	Public           bool
}

func LegacyRoutes() []LegacyRoute {
	routes := make([]LegacyRoute, 0, len(legacyPageRoutes))
	for selector, route := range legacyPageRoutes {
		routes = append(routes, LegacyRoute{selector, route.family, route.public})
	}

	sort.Slice(routes, func(i, j int) bool { return routes[i].Selector < routes[j].Selector })
	return routes
}

func (s *Adapter) catalogPage(ctx context.Context, x *actor, method string, u *url.URL) pages.Result {
	a := s.account(ctx, x.username)
	if a == nil {
		return pages.RoomNotice(503, "Account", "Account could not be loaded; try again")
	}

	return pages.Handle(method, u, pages.User{Username: a.name, DisplayName: a.display, Status: a.status, Online: s.workflow.Presence(ctx, a.name) != 99}, s.pageUsers(ctx))
}
