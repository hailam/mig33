# Original v4.62 media and native services: PASS

Validated on **2026-10-27** against an isolated schema16 database, `/tmp/mig33-native-media-complete-v16.sqlite`, with HTTP28080/TCP29119. The main user database was not used.

| Transport | Protocol frames | Screenshots | Logout acknowledgment |
| --- | ---: | ---: | --- |
| TCP |117|3|PASS|
| HTTP |149|3|PASS|

Both native runs passed all [media acceptance checks](MEDIA.md):914 types1–4 through original caches/decoders, local service icons, PNG/JPEG uploads/gallery, raw/reference photo messages,602 anomaly/reset, native SMS/mail/callback/invitations, and original English language catalog/download/selection. Total: **266 protocol frames and6 screenshots**. Generated decoded-media fixtures are excluded from the frame count. Runtime logs contain no Java exceptions or assertion failures.

Both startups also verified four automatic206 local-directory acknowledgments, zero startup errors and zero207 external connection-state packets. The descriptor labels explicitly say “saved contacts (local)”. The only later0 response is the intentional native SMS pending-provider receipt, avoiding the applet's hardcoded false “SMS sent” acknowledgment.

The final gallery screenshots contain actual decoded photos. Explicit image-cache assertions cover3 images in the final TCP gallery and7 in the final HTTP gallery, including earlier uploads. This catches the premature expiry bug that a successful200 PNG response alone did not reveal.

- [TCP gallery](runs/media-out/tcp/gallery-source-1.png)
- [HTTP gallery](runs/media-out/http/gallery-source-1.png)
- [Language catalog](runs/media-out/tcp/language-catalog.png)
- [Machine-readable evidence](runs/media-out/evidence-summary.json)

Original JAR SHA256: `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

Physical camera acquisition, sound output and vibration require hardware acceptance. Historical downloadable catalogs, additional translations and provider connectivity remain separate from these working local contracts.
