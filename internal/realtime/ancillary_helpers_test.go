package realtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"testing"

	"github.com/hailam/mig33/internal/pages"
	"github.com/hailam/mig33/internal/testassets"
)

func fixtureAsset(t *testing.T, name string) []byte {
	t.Helper()
	return testassets.Bytes(name)
}

func fixtureReportBinding(secret, nonce, kind, target string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(nonce + "\x00" + kind + "\x00" + target))
	return hex.EncodeToString(mac.Sum(nil))
}

// Exercise the real legacy HTTP selector/form boundary; the old tests called
// mixed presentation methods directly on the application object.
func ancillaryPage(t *testing.T, app *Application, viewer *session, method string, target *url.URL, form url.Values) pages.Result {
	t.Helper()
	response := fetchReport(t, app, viewer.id, method, target.String(), form.Encode())
	return pages.Result{Status: response.Code, ContentType: response.Header().Get("Content-Type"), Body: response.Body.Bytes()}
}
