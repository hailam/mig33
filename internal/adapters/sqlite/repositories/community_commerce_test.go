package repositories

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func communityTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.Context(), filepath.Join(t.TempDir(), "community.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCommunityPrivacyMembershipAndRoomOwnership(t *testing.T) {
	s := communityTestStore(t)
	g, err := s.CreateCommunity(t.Context(), "alice", "Island Friends", "Private forum & rooms", true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CommunityMembership(t.Context(), g.ID, "bob", "bob", "join"); err != nil {
		t.Fatal(err)
	}
	if role, _ := s.CommunityRole(t.Context(), g.ID, "bob"); role != "pending" {
		t.Fatal(role)
	}
	if _, err = s.CommunityMembers(t.Context(), g.ID, "bob"); !errors.Is(err, ErrForbidden) {
		t.Fatal("private member list exposed", err)
	}
	if _, err = s.CommunityPosts(t.Context(), g.ID, "bob"); !errors.Is(err, ErrForbidden) {
		t.Fatal("private forum exposed", err)
	}
	if err = s.CommunityMembership(t.Context(), g.ID, "bob", "bob", "approve"); !errors.Is(err, ErrForbidden) {
		t.Fatal("self approval", err)
	}
	if err = s.CommunityMembership(t.Context(), g.ID, "alice", "bob", "approve"); err != nil {
		t.Fatal(err)
	}
	if err = s.CommunityMembership(t.Context(), g.ID, "alice", "bob", "promote"); err != nil {
		t.Fatal(err)
	}
	if err = s.CommunityMembership(t.Context(), g.ID, "bob", "alice", "remove"); !errors.Is(err, ErrForbidden) {
		t.Fatal("moderator removed owner", err)
	}
	if err = s.CommunityMembership(t.Context(), g.ID, "alice", "alice", "leave"); !errors.Is(err, ErrForbidden) {
		t.Fatal("owner orphaned group", err)
	}
	r, err := s.CreateRoom(t.Context(), "alice", "Island Lounge", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.LinkCommunityRoom(t.Context(), g.ID, "bob", r.Name, false); !errors.Is(err, ErrForbidden) {
		t.Fatal("nonowner linked room", err)
	}
	if err = s.LinkCommunityRoom(t.Context(), g.ID, "alice", r.Name, false); err != nil {
		t.Fatal(err)
	}
	if rooms, err := s.CommunityRooms(t.Context(), g.ID, "bob"); err != nil || len(rooms) != 1 {
		t.Fatal(rooms, err)
	}
	if _, err = s.CommunityRooms(t.Context(), g.ID, "carol"); !errors.Is(err, ErrForbidden) {
		t.Fatal("private rooms exposed", err)
	}
	g2, err := s.CreateCommunity(t.Context(), "alice", "Other Friends", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.LinkCommunityRoom(t.Context(), g2.ID, "alice", r.Name, false); !errors.Is(err, ErrConflict) {
		t.Fatal("room linked twice", err)
	}
	if err = s.LinkCommunityRoom(t.Context(), g.ID, "alice", r.Name, true); err != nil {
		t.Fatal(err)
	}
	if err = s.LinkCommunityRoom(t.Context(), g2.ID, "alice", r.Name, false); err != nil {
		t.Fatal(err)
	}
}

func TestCommunityForumRetriesAndThreadIsolation(t *testing.T) {
	s := communityTestStore(t)
	g, _ := s.CreateCommunity(t.Context(), "alice", "First", "", false)
	other, _ := s.CreateCommunity(t.Context(), "bob", "Second", "", false)
	nonce := strings.Repeat("a", 48)
	post, err := s.AddCommunityPost(t.Context(), g.ID, 0, "alice", "A & B + 50% ✓", nonce)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := s.AddCommunityPost(t.Context(), g.ID, 0, "alice", "A & B + 50% ✓", nonce); err != nil || id != post {
		t.Fatal(id, err)
	}
	if _, err = s.AddCommunityPost(t.Context(), g.ID, 0, "alice", "changed", nonce); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err = s.AddCommunityPost(t.Context(), other.ID, post, "bob", "Cross-group reply", strings.Repeat("b", 48)); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = s.AddCommunityPost(t.Context(), g.ID, post, "bob", "Nonmember", strings.Repeat("b", 48)); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if err = s.CommunityMembership(t.Context(), g.ID, "bob", "bob", "join"); err != nil {
		t.Fatal(err)
	}
	reply, err := s.AddCommunityPost(t.Context(), g.ID, post, "bob", "Reply", strings.Repeat("b", 48))
	if err != nil || reply == post {
		t.Fatal(reply, err)
	}
	if _, err = s.AddCommunityPost(t.Context(), g.ID, reply, "bob", "Nested", strings.Repeat("c", 48)); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	posts, err := s.CommunityPosts(t.Context(), g.ID, "carol")
	if err != nil || len(posts) != 2 || posts[0].ParentID != post {
		t.Fatal(posts, err)
	}
}

func TestCreditLedgerConcurrentRetryOverspendAndGift(t *testing.T) {
	s := communityTestStore(t)
	_, err := s.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", strings.Repeat("a", 48), 1000)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.CreditOperation(t.Context(), "alice", "bob", "transfer", "", strings.Repeat("b", 48), 600)
			if err != nil {
				mu.Lock()
				failures = append(failures, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(failures) > 0 {
		t.Fatal(failures)
	}
	if balance, _ := s.CreditBalance(t.Context(), "alice"); balance != 400 {
		t.Fatal(balance)
	}
	if balance, _ := s.CreditBalance(t.Context(), "bob"); balance != 600 {
		t.Fatal(balance)
	}
	if _, err = s.CreditOperation(t.Context(), "alice", "bob", "transfer", "", strings.Repeat("b", 48), 500); !errors.Is(err, ErrConflict) {
		t.Fatal("nonce changed payload", err)
	}
	if _, err = s.CreditOperation(t.Context(), "alice", "bob", "transfer", "", strings.Repeat("c", 48), 500); !errors.Is(err, ErrLimit) {
		t.Fatal("overspend", err)
	}
	for i := 0; i < 2; i++ {
		if _, err = s.CreditOperation(t.Context(), "alice", "bob", "purchase", "rose", strings.Repeat("d", 48), 100); err != nil {
			t.Fatal(err)
		}
	}
	items, err := s.OwnedStoreItems(t.Context(), "bob")
	if err != nil || len(items) != 1 || items[0].Sender != "alice" {
		t.Fatal(items, err)
	}
	if balance, _ := s.CreditBalance(t.Context(), "alice"); balance != 300 {
		t.Fatal(balance)
	}
	history, err := s.CreditHistory(t.Context(), "alice")
	if err != nil || len(history) != 3 {
		t.Fatal(history, err)
	}
	var sum int64
	for _, entry := range history {
		sum += entry.Delta
	}
	if sum != 300 {
		t.Fatal(sum)
	}
	if err = s.SelectStoreTheme(t.Context(), "bob", "theme-blue"); !errors.Is(err, ErrForbidden) {
		t.Fatal("unowned theme", err)
	}
	if _, err = s.CreditOperation(t.Context(), "bob", "bob", "purchase", "theme-blue", strings.Repeat("e", 48), 300); err != nil {
		t.Fatal(err)
	}
	if err = s.SelectStoreTheme(t.Context(), "bob", "theme-blue"); err != nil {
		t.Fatal(err)
	}
	if theme, _ := s.SelectedStoreTheme(t.Context(), "bob"); theme != "theme-blue" {
		t.Fatal(theme)
	}
	if _, err = s.CreditOperation(t.Context(), "bob", "alice", "purchase", "theme-blue", strings.Repeat("f", 48), 300); !errors.Is(err, ErrInvalid) {
		t.Fatal("gifted personal theme", err)
	}
}
