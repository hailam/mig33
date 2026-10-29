# Activity pages and the native Updates tab

Buzz and Lookout now combine persistent updates and check-ins, ordered by creation time and ID, with a maximum of 100 records per page. Check-ins therefore appear to other users instead of being stored only on the author's Check In page.

The original contact-context URLs append cid=CONTACT_ID and pres=PRESENCE to the Buzz or Lookout route. cid is resolved against the signed-in viewer's contact roster; it is not a global username or a community Group ID. The local username selector is also accepted, but combining selectors or repeating either is rejected. pres is advisory and does not change permissions or stored presence.

The local Activity Updates setting hides activity from other users when switched off. Profile privacy and personal blocks are also applied. Owners retain access to their own records. This controls Buzz, Lookout, check-in activity and leaderboard visibility; separately authored blog pages continue to use profile privacy. These are explicit local policies, not recovered historical server rules.

## Native protocol

| Packet | Fields and behavior |
| --- | --- |
| 921 request | Tag 1 BYTE: 1 subscribes, 0 unsubscribes |
| 1 reply | Correlated acknowledgment |
| 422 event | Tag 1 INT age in seconds; tag 2 BYTE item type 1; optional tag 3 TEXT real avatar URL; tag 4 TEXT summary with an inline link |

An initial subscription returns up to 20 visible recent updates. Successful new Buzz/check-in publications send events to subscribed sessions. An identical publication retry does not broadcast a duplicate. Unsubscribed sessions receive no live events. This native stream is transient: the persistent web pages remain the complete local record, and a fresh session can load recent events again. It is not a durable notification or read-receipt protocol.

The server checks privacy, blocks, record existence and subscription again before writing a buffered activity packet. Deleting a post, hiding activity, blocking its author or unsubscribing prevents a queued packet from being delivered. Session-local packet metadata is released after transport completion.

Source evidence is in research/applets/v4/artifacts/normal-flow/bc.java, case 422, and av.java, method a(int,byte,String,String). The original renderer accepts anchor markup but does not decode text entities; u.java's tokenizer preserves entity spellings. Consequently the event uses a fixed summary such as “alice posted an update” and a link to the full page, where user text is properly escaped and decoded. It does not insert arbitrary user text into activity markup.

When there is no display picture, tag 3 is absent: av accepts a null avatar key. Sending a fake key such as 0 would cause an unnecessary image request. Age is clamped to 2,147,483 seconds because av multiplies its Java integer by 1000 before promoting it to a long; older ages would otherwise overflow.

## Rendering and validation

Activity markup is rendered by internal/pages/activity.templ through the shared templ renderer. Community and commerce pages also compose typed templ components for headings, text, links, forms and controls. The shared renderer performs the single legacy URL-attribute adaptation required by the applet; feature handlers no longer concatenate HTML.

Tests exercise original contact IDs, merged check-ins, activity/profile privacy, initial subscription, retry suppression, unsubscribe behavior and cancellation of buffered hidden updates. Separate media tests cover private photo audiences, blocked downloads, opcode 913/914 correlation and static-image type, thumbnail aspect ratio, and chat progress during a slow image response.
