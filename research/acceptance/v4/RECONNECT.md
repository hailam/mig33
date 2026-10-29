# Retained-tab reconnect acceptance

`V4ReconnectIntegration` runs three unchanged original v4.62 clients against an isolated backend. Defaults are HTTP18080/TCP19119; it does not use the main8080/9119 listeners or main database.

```sh
# Start a fresh isolated backend advertising its actual HTTP origin first.
./bin/omni -http 127.0.0.1:18080 -tcp 127.0.0.1:19119 \
  -public-url http://127.0.0.1:18080 -db /tmp/mig33-reconnect.sqlite
sh tools/applets/v4/integration/run-reconnect.sh
```

Use `MIG33_TEST_HTTP_PORT` and `MIG33_TEST_TCP_PORT` to select different isolated ports. The Connector shim only remaps configured loopback listeners. The original JAR and response dispatcher are unchanged.

Alice/HTTP creates a room through original browser forms, sets capacity2 and joins it. Bob/TCP exercises the original `aj.c(true)` connection-loss routine and `bc.e()` reconnect action. That routine retains the room tab, clears readiness, and successful203 causes the original client to issue703 automatically. The harness observes this request rather than issuing its own replacement join. It sets the reconnect delay to120 seconds before disconnection so peer fixtures can change capacity or bans, then invokes the original Reconnect Now action, which cancels that timer and starts login.

Scenarios:

1. Normal reconnect automatically rejoins and accepts a message.
2. While Bob is disconnected, Carol/HTTP fills the remaining spot. The automatic rejoin returns a full-room error, the original retained tab displays it, and subsequent sending is rejected.
3. Alice bans Bob through the original Room Settings form. Automatic rejoin fails with the ban error and the retained tab remains unavailable.
4. Alice unbans Bob. A subsequent native reconnect restores membership.
5. Alice uses native706 to kick Bob. Notification5 displays through the original popup; the room tab remains. Sending fails until Bob reconnects and the automatic703 succeeds, demonstrating the implemented distinction between a kick and a ban.

The three clients coordinate only through local marker files. Results include original binary frames, parsed room state, screenshots, and correlated logout ACKs. Frame filenames use a global sequence so a newly opened TCP socket cannot overwrite its first connection's evidence.

Captures invoke the original Canvas paint method. For embedded pages, painting itself may clear the original deferred-tab flag; readiness requires two stable populated frames after that native path. No paint flags, room-readiness flags or server membership are changed by the harness.

The runner's `evidence-summary.json` records observed completion. A script existing or compiling is not evidence that all scenarios passed; consult the actual run result.
