# Original-client message mirrors and external contacts

This focused acceptance run starts three separate, unchanged v4.62 JVMs: Alice's sending client over TCP, Alice's second client over HTTP, and Bob over TCP. Each has isolated in-memory RMS and its own traces. `V4MirrorIntegration` uses the original login flow, chat editor, Send command, packet builders, receiver, chat models and Canvas renderer.

Use a fresh isolated database and a separate server:

```sh
go run ./cmd/omni -http 127.0.0.1:38080 -tcp 127.0.0.1:39119 \
  -public-url http://127.0.0.1:38080 -db /tmp/mig33-mirror-test.sqlite
MIG33_TEST_HTTP_PORT=38080 MIG33_TEST_TCP_PORT=39119 \
  sh tools/applets/v4/integration/run-mirrors.sh
```

The harness rejects remote connections. The original configured default addresses are redirected only to these explicit loopback test ports. It verifies the original JAR hash before compiling and does not edit the JAR. The shared MicroEmulator RMS compatibility shim preserves nested-open reference counts; it does not change application contact or chat logic.

## Assertions

- The sending Alice enters private text through the original editor. Her original model contains exactly one local echo, and no duplicate server500 arrives for that text.
- Alice's second client receives exactly one private500 with peer `bob` in tag2, content6 and explicit `alice: …` in the body. The message opens Bob's tab; it does not create a self-addressed Alice tab.
- Bob receives one ordinary content1 private message with sender `alice` and the original text.
- Alice creates a private group with Bob and Carol through the original751 builder. Carol stays offline. The sender's group message appears once locally; the second Alice session and Bob each receive one ordinary group copy attributed to Alice.
- Bob adds a phone-only contact and an external identity through the original405 builder. Correlated402 replies populate the original phone and external-account fields; the external service remains presence99.
- Contact rendering selects dynamic provider icons for external identities. The harness invokes the original roster renderer directly, in addition to the Canvas, to detect swallowed drawing exceptions. Both phone and external labels must be rendered.
- The original IM menu visibly names the adapters as saved contacts `(local)`. The client's four automatic206 requests receive acknowledgments without startup errors or207 provider connection events. An explicit206 acknowledgment states that the external provider is disconnected and no external sign-in occurred.302/907 settings operations preserve the rendered offline saved contacts.
- All three clients receive correlated logout acknowledgments and exit.

## Evidence and limits

`mirror-out/` contains original binary frames, event logs, original-model text, Canvas screenshots and `evidence-summary.json`. The principal chat/contact captures wait for two identical populated bodies after the original tab-paint delay. Initial/menu captures are supplemental.

The native private mirror uses the recovered content6 action presentation because the original incoming private-message field2 serves as both the tab address and sender label. Consequently, the mirror uses the action color and an explicit author prefix. Stored messages and the JSON transcript retain their original body, action flag and RGB. This is a documented adapter behavior, not a claim that the historical service used this format.

Phone/IM rows represent saved address-book identities only. Descriptor state2 is required for the original client's dynamic icon renderer and represents the explicitly named local directory adapter. Every external identity remains presence99; no207 connected packet or external sign-in occurs. The test neither connects to historical providers nor asserts phone/call delivery. Group rename/delete convergence for another native session remains a separate original-client limitation documented in [contacts.md](../../../docs/contacts.md).
