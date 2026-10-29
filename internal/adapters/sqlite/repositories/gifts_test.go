package repositories

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func seedGiftIdentity(t *testing.T, s *Store, name string, id int32) {
	t.Helper()
	if err := s.SeedIdentity(t.Context(), Identity{ID: id, Username: name, Display: name, Activated: true, PasswordSalt: make([]byte, 16), PasswordKey: make([]byte, 32), PasswordIterations: 100000, LegacyUnits: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestGiftNoteConcurrentRetryRestartAndLegacyReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gifts.sqlite")
	s, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err = s.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", strings.Repeat("a", 48), 1000); err != nil {
		t.Fatal(err)
	}
	legacyNonce, noteNonce := strings.Repeat("b", 48), strings.Repeat("c", 48)
	legacy, err := s.CreditOperation(t.Context(), "alice", "bob", "purchase", "rose", legacyNonce, 100)
	if err != nil {
		t.Fatal(err)
	}
	note := "Hello 世界 & plus + equals = <b>literal</b>"
	var wg sync.WaitGroup
	receipts := make(chan CreditReceipt, 12)
	errorsFound := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.CreditOperationWithNote(t.Context(), "alice", "bob", "purchase", "rose", noteNonce, 100, note)
			receipts <- r
			errorsFound <- err
		}()
	}
	wg.Wait()
	close(receipts)
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	var receipt CreditReceipt
	for r := range receipts {
		if receipt.ID != 0 && receipt != r {
			t.Fatal("parallel retry returned different receipts")
		}
		receipt = r
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if r, err := s.CreditOperationWithNote(t.Context(), "alice", "bob", "purchase", "rose", noteNonce, 100, note); err != nil || r != receipt {
		t.Fatalf("restart replay: %+v %v", r, err)
	}
	if r, err := s.CreditOperation(t.Context(), "alice", "bob", "purchase", "rose", legacyNonce, 100); err != nil || r != legacy {
		t.Fatalf("empty-note legacy replay: %+v %v", r, err)
	}
	for _, changedNote := range []string{"", "a changed note"} {
		if _, err := s.CreditOperationWithNote(t.Context(), "alice", "bob", "purchase", "rose", noteNonce, 100, changedNote); !errors.Is(err, ErrConflict) {
			t.Fatalf("changed note must conflict: %v", err)
		}
	}
	for _, user := range []string{"alice", "bob"} {
		if got, err := s.GiftNote(t.Context(), receipt.ID, user); err != nil || got != note {
			t.Fatalf("note lost for %s: %q %v", user, got, err)
		}
	}
	if _, err := s.GiftNote(t.Context(), receipt.ID, "carol"); !errors.Is(err, ErrNotFound) {
		t.Fatal("gift note exposed to unrelated viewer", err)
	}
	if balance, _ := s.CreditBalance(t.Context(), "alice"); balance != 800 {
		t.Fatal("retry charged again", balance)
	}
	owned, err := s.OwnedStoreItems(t.Context(), "bob")
	if err != nil || len(owned) != 2 {
		t.Fatalf("duplicate inventory: %+v %v", owned, err)
	}
	messages, err := s.Pending(t.Context(), "bob", 100)
	if err != nil || len(messages) != 2 || messages[1].Body != "alice sent bob a Rose [gift:rose] Message: "+note || messages[1].Markup != "" {
		t.Fatalf("literal durable gift message: %+v %v", messages, err)
	}
}

func TestGiftAtomicRollbackPrivacyAndValidation(t *testing.T) {
	s := testStore(t)
	seedGiftIdentity(t, s, "alice", 1)
	seedGiftIdentity(t, s, "bob", 2)
	if _, err := s.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", strings.Repeat("a", 48), 1000); err != nil {
		t.Fatal(err)
	}
	purchase := func(note string) error {
		_, err := s.CreditOperationWithNote(t.Context(), "alice", "bob", "purchase", "rose", strings.Repeat("b", 48), 100, note)
		return err
	}
	assertNoPurchase := func() {
		t.Helper()
		for _, table := range []string{"gift_notes", "owned_store_items", "messages", "message_audience"} {
			var count int
			if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
				t.Fatalf("failed gift left %s=%d: %v", table, count, err)
			}
		}
		if balance, _ := s.CreditBalance(t.Context(), "alice"); balance != 1000 {
			t.Fatal("failed gift changed wallet", balance)
		}
		if entries, _ := s.CreditHistory(t.Context(), "alice"); len(entries) != 1 {
			t.Fatal("failed gift changed ledger", entries)
		}
	}
	if _, err := s.db.Exec(`CREATE TRIGGER reject_gift BEFORE INSERT ON message_audience BEGIN SELECT RAISE(ABORT,'test rollback'); END`); err != nil {
		t.Fatal(err)
	}
	if err := purchase("atomic note"); err == nil {
		t.Fatal("audience insertion failure was accepted")
	}
	assertNoPurchase()
	if _, err := s.db.Exec("DROP TRIGGER reject_gift"); err != nil {
		t.Fatal(err)
	}
	for _, note := range []string{strings.Repeat("🌹", 81), "line\nbreak", "tab\tnote", "\x00", "[GIFT:rose]", "[photo:1]", "[pack:cool]", "hidden\u202etext", string([]byte{0xff})} {
		if err := purchase(note); !errors.Is(err, ErrInvalid) {
			t.Fatalf("unsafe note %q: %v", note, err)
		}
	}
	if err := s.SetBlocked(t.Context(), "bob", "alice", true); err != nil {
		t.Fatal(err)
	}
	if err := purchase("blocked"); !errors.Is(err, ErrBlocked) {
		t.Fatal("blocked gift allowed", err)
	}
	if err := s.SetBlocked(t.Context(), "bob", "alice", false); err != nil {
		t.Fatal(err)
	}
	for _, privacy := range []string{"nobody", "friends"} {
		if err := s.SetIdentityPreference(t.Context(), "bob", "chat_privacy", privacy); err != nil {
			t.Fatal(err)
		}
		if err := purchase("private"); !errors.Is(err, ErrForbidden) {
			t.Fatal("private gift allowed", privacy, err)
		}
	}
	assertNoPurchase()
	if err := s.EnsureContacts(t.Context(), "bob", []Contact{{ID: 1, Username: "alice", Display: "Alice", GroupID: -1}}); err != nil {
		t.Fatal(err)
	}
	if err := purchase(strings.Repeat("🌹", 80)); err != nil {
		t.Fatal("allowed friend/boundary note rejected", err)
	}
	if rows, _ := s.Pending(t.Context(), "bob", 10); len(rows) != 1 {
		t.Fatal("successful gift has no durable recipient audience")
	}
}

func TestGiftRatingOneVoteConcurrencyAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rating.sqlite")
	s, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	for i, name := range []string{"alice", "bob", "carol"} {
		seedGiftIdentity(t, s, name, int32(i+1))
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.RateGift(t.Context(), "alice", "rose", 5); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if _, err := s.RateGift(t.Context(), "bob", "rose", 1); err != nil {
		t.Fatal(err)
	}
	if rating, err := s.RateGift(t.Context(), "alice", "rose", 3); err != nil || rating != (GiftRatingSummary{Count: 2, Average: 2, OwnRating: 3}) {
		t.Fatalf("replacement vote affected count: %+v %v", rating, err)
	}
	for _, rating := range []int{-1, 0, 6} {
		if _, err := s.RateGift(t.Context(), "carol", "rose", rating); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid local scale accepted", rating, err)
		}
	}
	if _, err := s.RateGift(t.Context(), "carol", "theme-blue", 4); !errors.Is(err, ErrInvalid) {
		t.Fatal("non-gift rating accepted", err)
	}
	if _, err := s.RateGift(t.Context(), "nobody", "rose", 4); !errors.Is(err, ErrNotFound) {
		t.Fatal("unknown account rated", err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if rating, err := s.GiftRating(t.Context(), "rose", "carol"); err != nil || rating != (GiftRatingSummary{Count: 2, Average: 2}) {
		t.Fatalf("rating restart: %+v %v", rating, err)
	}
}

func TestGiftSchema18UpgradePreservesExistingReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v17.sqlite")
	s, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", strings.Repeat("a", 48), 1000); err != nil {
		t.Fatal(err)
	}
	receipt, err := s.CreditOperation(t.Context(), "alice", "bob", "purchase", "rose", strings.Repeat("b", 48), 100)
	if err != nil {
		t.Fatal(err)
	}
	messages, _ := s.Pending(t.Context(), "bob", 10)
	if _, err = s.db.Exec("DROP TABLE gift_notes; DROP TABLE gift_ratings; PRAGMA user_version=17;"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 18 {
		t.Fatal("schema18 migration", version, err)
	}
	if again, err := s.CreditOperation(t.Context(), "alice", "bob", "purchase", "rose", strings.Repeat("b", 48), 100); err != nil || again != receipt {
		t.Fatal("legacy receipt replay changed", again, err)
	}
	if current, _ := s.Pending(t.Context(), "bob", 10); fmt.Sprint(current) != fmt.Sprint(messages) {
		t.Fatal("migration changed historical gift messages")
	}
	if note, err := s.GiftNote(t.Context(), receipt.ID, "bob"); err != nil || note != "" {
		t.Fatal("legacy receipt acquired a note", note, err)
	}
}

func TestGiftConcurrentBlockAndPurchaseHaveAtomicAudience(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gift-block.sqlite")
	s, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	other, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := s.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", strings.Repeat("a", 48), 10000); err != nil {
		t.Fatal(err)
	}
	successful := 0
	for i := 0; i < 20; i++ {
		if err := other.SetBlocked(t.Context(), "bob", "alice", false); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		blockDone := make(chan error, 1)
		go func() {
			<-start
			blockDone <- other.SetBlocked(t.Context(), "bob", "alice", true)
		}()
		close(start)
		note := fmt.Sprintf("concurrent note %d", i)
		receipt, err := s.CreditOperationWithNote(t.Context(), "alice", "bob", "purchase", "rose", fmt.Sprintf("%048x", i+1), 100, note)
		if blockErr := <-blockDone; blockErr != nil {
			t.Fatal(blockErr)
		}
		if err != nil && !errors.Is(err, ErrBlocked) {
			t.Fatal("unexpected serialized gift/block result", err)
		}
		var audiences int
		if queryErr := s.db.QueryRow(`SELECT COUNT(*) FROM message_audience a JOIN messages m ON m.id=a.message_id WHERE a.username='bob' AND m.body=?`, "alice sent bob a Rose [gift:rose] Message: "+note).Scan(&audiences); queryErr != nil {
			t.Fatal(queryErr)
		}
		if err == nil {
			successful++
			if audiences != 1 || receipt.ID == 0 {
				t.Fatal("paid gift lost its immutable recipient audience", audiences, receipt)
			}
		} else if audiences != 0 {
			t.Fatal("block-first gift created a recipient audience", audiences)
		}
	}
	if balance, _ := s.CreditBalance(t.Context(), "alice"); balance != int64(10000-100*successful) {
		t.Fatal("concurrent privacy failure charged", balance, successful)
	}
	if inventory, _ := s.OwnedStoreItems(t.Context(), "bob"); len(inventory) != successful {
		t.Fatal("concurrent purchase inventory did not match charge", len(inventory), successful)
	}
}
