# Original client acceptance result

**PASS — 2026-10-25.** Two unchanged mig33 v4.62 client JVMs completed the local Go backend acceptance sequence. Alice used TCP; Bob used the HTTP tunnel. This is runtime compatibility evidence from the original application classes, not an execution of the decompiler output.

| Check | Result |
| --- | --- |
| Original200→201→202→203 login | Passed over TCP and HTTP |
| Original400→401/402/420/403 roster flow | Passed; rendered original contact model |
| Original713→714/715 category flow | Passed; rendered original category model |
| HTTP empty GET200/image-gif polling | Passed, followed by subsequent successful message delivery |
| Alice500 → Bob original chat | Passed; incoming text found in original wrapped chat rows and screenshot |
| Bob500 → Alice original chat | Passed; incoming text found in original wrapped chat rows and screenshot |
| New922→923 profile fetch | Passed; original page parser produced title `User Profile` and rendered local account data |
| Original JAR integrity | SHA-256 verified by run script |
| External-network guard | Invalid-host self-test rejected; legacy analytics attempts rejected |

The preserved result files are [Alice](runs/out/alice/result.txt) and [Bob](runs/out/bob/result.txt). The corresponding [Alice log](runs/out/alice/runtime.log), [Bob log](runs/out/bob/runtime.log), binary frame files and screenshots are from the latest `run-pair.sh` execution. See [reproduction and boundaries](README.md) for the exact invocation and scope.

The latest private-chat regression ran after the [public-room test](room-report.md), against the same database with group-restoration notifications present. It passed with 6 roster rows and 2 category rows. The current room category starts collapsed, so its unloaded footer is absent. Empty polls used chunked HTTP responses; detection checks the actual empty body instead of requiring a Content-Length header. Both original clients also completed 300 logout and received its acknowledgment.

Runtime: Homebrew OpenJDK26.0.1, MicroEmulator2.0.4, English resources, 240×320 Canvas. The executable path used retains the Homebrew `openjdk@21` directory name; the installed runtime reports26.0.1. This run does not establish other JDK or handset compatibility.

The original raw-field probe produced these values (NUL bytes shown as `\\0`):

```text
payload length2, bytes0000: int=null, short=0, string=\\0\\0
payload length4, bytes00000000: int=0, short=0, string=\\0\\0\\0\\0
payload length0: int=null, short=null, string=""
```

That behavior explains the successful203 fixture's two-byte tag31 and empty-string tag32. The later original login, roster and UI execution provides the stronger end-to-end check of that fixture.

Assertions cover protocol exchange, selected original models and rendering. Login, message submission and tab navigation are invoked through reflection; no claim is made for automated keypad input, all APIs, native handset UI, or the missing historical server payloads.
