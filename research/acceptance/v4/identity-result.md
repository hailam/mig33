# Identity acceptance result

Run date:2026-10-27. Original JAR SHA-256:`dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

**PASS after the templ refactor:** two fresh original-client processes, TCP registration/activation/password change followed by HTTP login using the changed password. The final run used a dedicated schema12 database with pages rendered by `identity_views.templ` and shared components. The full original JAR digest is recorded authoritatively in[`identity-out/evidence-summary.json`](runs/identity-out/evidence-summary.json).

| Client | Screenshots | Binary frames | Logout ACK |
|---|---:|---:|---|
|Register / TCP|20|31|Observed|
|Return / HTTP|6|21|Observed|
|Total|26|52|Both observed|

The original username availability path returned a two-byte`[]`, then101/102 rendered the development123456 captcha inside Step2. Original103 created the account,203 reported unactivated state, and204 from the native verification editor activated it. The native browser successfully submitted both password steps with a punctuation-heavy final password value. A fresh HTTP client authenticated with the changed password and received the saved mobile, activation and profile.

The captcha screenshot was visually inspected and clearly shows123456. The final templ-rendered Current Password and New Password pages were also visually inspected; form actions, input width and native password submission remain correct. Evidence includes the original forms, the activation steps, rendered local password pages, both client logs and every captured binary frame. Go tests separately cover restart persistence, public web registration/recovery, challenge replay and attempt limits, session invalidation, CSRF and enforced profile/DM/group-invitation privacy.
