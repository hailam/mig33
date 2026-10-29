# Backend documentation

Start with [architecture and running the backend](backend.md). [Configuration](../configs/README.md) covers overlays and environment settings; [OpenAPI](../api/openapi.yaml) describes the HTTP contract.

[Development and Go style](development.md) covers pinned formatters, readability conventions and CI checks.

| Area | Guides |
| --- | --- |
| Accounts and contacts | [Identity](identity.md), [contacts](contacts.md), [friend discovery](friend-discovery.md), [blocking](blocking.md) |
| Messaging | [Chat delivery/history](chat.md), [slash commands](slash-commands.md), [text/profile formatting](text-and-page-formatting.md) |
| Public rooms | [Rooms](rooms.md), [discovery](room-discovery.md), [room administration and bots](room-completion.md) |
| Moderation | [Reports and votes](reports.md), [operator review](operator-review.md) |
| Embedded services | [Page catalog](service-pages.md), [community and commerce](community-commerce.md), [activity updates](activity-updates.md), [native media/services](native-services.md) |
| Scope and validation | [Target and gaps](target-and-gaps.md), [core flow audit](core-flow-audit.md), [reconstruction milestones](reconstruction-notes.md) |

Historical applet protocol/UI references and architecture research live under [research/](../research/README.md). Reusable Java/emulator tooling lives under [tools/applets/](../tools/applets/README.md).
