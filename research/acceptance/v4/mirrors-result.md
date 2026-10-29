# Original v4.62 message mirrors and saved contacts: PASS

Validated on **2026-10-27** against a fresh isolated schema16 database, `/tmp/mig33-mirrors-native-final2-20261027.sqlite`, with HTTP38080/TCP39119. The main user database was not used. The isolated backend was stopped after the run.

| Client | Transport | Protocol frames | Screenshots | Logout acknowledgment |
| --- | --- | ---: | ---: | --- |
| Sending Alice | TCP | 53 | 5 | PASS |
| Second Alice | HTTP | 48 | 3 | PASS |
| Bob | TCP | 63 | 8 | PASS |

All three clients passed the [focused acceptance checks](MIRRORS.md): **164 protocol frames and 16 screenshots**, with no Java exceptions or assertion failures.

The original sender's private and group models each contain one local echo, with no duplicate server copy. The second Alice session receives exactly one private message in Bob's tab, explicitly attributed to Alice, without opening an Alice self-chat. Bob receives the original body from Alice. The second Alice session and Bob each receive exactly one group copy from Alice.

Bob's original405 builders add a phone-only contact and an external IM identity. Both correlated402 replies populate the expected native fields, and both labels appear in the original Friends renderer. Invoking that renderer directly also succeeds, preventing a swallowed drawing exception from passing merely because part of the screen painted. The external identity stays offline at presence99.

The original IM menu visibly labels each adapter as saved contacts `(local)`. All three clients receive four automatic206 acknowledgments at startup, with zero errors and zero207 external connection events. An explicit206 acknowledgment states that only the local directory is available and no external sign-in occurred. Original302/907 builders and their ACK handlers preserve the rendered offline contact;907 uses the original selected-provider confirmation context.

- [Second Alice's private copy in Bob's tab](runs/mirror-out/mirror/private-own-mirror.png)
- [Second Alice's group copy](runs/mirror-out/mirror/group-own-mirror.png)
- [Phone and external contact rows](runs/mirror-out/bob/phone-external-contacts.png)
- [Local saved-contact adapter labels](runs/mirror-out/bob/local-directory-menu.png)
- [Contact rows after302/907](runs/mirror-out/bob/contacts-after-im-settings.png)
- [Machine-readable evidence](runs/mirror-out/evidence-summary.json)

Original JAR SHA256: `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

The own-private copy deliberately uses native content6 and an explicit author prefix because field2 also addresses the original private tab. It therefore has the action presentation rather than the sending client's local text style; stored messages and JSON transcripts retain their original body and style. Saved phone/IM contacts do not imply provider connectivity. These are documented local adapter behaviors, not claims of historical server parity.
