# Original-client account acceptance

Run against a dedicated disposable database on HTTP8080/TCP9119:

```sh
sh tools/applets/v4/integration/run-identity.sh
```

The harness uses the unchanged v4.62 JAR, original login/registration/activation forms, original network builders, receive dispatcher, embedded browser parser and form serializer. It supplies field values through original form-model setters, focuses original controls and invokes their FIRE/Continue handlers. The verification code also goes through the real native TextBox and commit path. Canvas screenshots are240×320 using MicroEmulator desktop fonts.

Two sequential processes have isolated in-memory RMS:

1. **Register / TCP:** opens the signed-out Login form, follows Register, submits the original username availability request and101. The loopback Connector narrowly rewrites the hardcoded official registration-check URL.102 supplies the visible123456 development captcha. The original Step2 Submit sends103; its ACK stores credentials in the applet. Login sends200/202 and receives an unactivated203. The actual Step3 editor/Activate path sends204 and the applet switches its activated flag. The registered account has an empty Friends address book. Original browser forms verify the current password and save a password containing literal`&`, `=`, `+`, `%` and spaces. Its raw final field is checked against the captured922 frame.
2. **Return / HTTP:** a fresh applet logs in with the changed password. It receives the persisted mobile and activated flag in203, renders its empty address book and its own User Profile through922/923.

Both processes send300 and observe the correlated ACK before exiting. The script creates a unique username/mobile per run and saves captures, rendered HTML, frame evidence and`evidence-summary.json` under`identity-out/`. Captured103 and password-form frames intentionally contain **local test credentials**; no real account credentials or provider requests are used.

This test does not claim that the recovered historical registration service used a static captcha or these reconstructed HTML password-page layouts. The native form designs are directly rendered from the original binary. The HTML forms are local service implementations exercised in its original renderer.
