package commerce

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// These parsing cases move with the amount rule from the former combined page
// and commerce test; route and HTML behavior remain adapter tests.
func TestCreditAmountDecimalContract(t *testing.T) {
	for _, raw := range []string{"", "1.001", "-1", "1e2", "1+2", ".1", "1.", "NaN", "1000000.01"} {
		if _, err := ParseAmount(raw); err == nil {
			t.Fatalf("invalid credit amount accepted: %q", raw)
		}
	}
	for raw, want := range map[string]int64{"1": 100, "1.2": 120, "0.01": 1, "1000000.00": 100000000} {
		got, err := ParseAmount(raw)
		if err != nil || got != want {
			t.Fatalf("%s: got %d, %v; want %d", raw, got, err, want)
		}
	}
}

func TestGiftDiscoveryLongUnicodeDetailsRemainBounded(t *testing.T) {
	text := GiftCommandText("Gift", []string{strings.Repeat("🌹", 5000)})
	if !utf8.ValidString(text) || len([]rune(text)) > GiftCommandTextLimit || !strings.Contains(text, "More details in Menu > Store") {
		t.Fatal("long catalog details exceeded the output bound or broke Unicode")
	}
}
