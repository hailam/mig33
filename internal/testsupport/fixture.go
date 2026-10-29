// Package testsupport constructs isolated fixtures for adapter and acceptance
// tests. Production entrypoints retain their own explicit composition root.
package testsupport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/adapters/httptransport"
	"github.com/hailam/mig33/internal/adapters/legacyweb"
	"github.com/hailam/mig33/internal/adapters/native"
	"github.com/hailam/mig33/internal/adapters/nativecodec"
	"github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/community"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/controllers"
	"github.com/hailam/mig33/internal/media"
	"github.com/hailam/mig33/internal/realtime"
	"github.com/hailam/mig33/internal/review"
	"github.com/hailam/mig33/internal/rooms"
	"github.com/hailam/mig33/internal/testassets"
)

type Fixture struct {
	close      func() error
	Core       *realtime.Coordinator
	Native     *native.Adapter
	Pages      *legacyweb.Adapter
	Review     *review.Service
	Repository *repositories.Store
	Handler    http.Handler
}

// Open uses only the requested fixture database. No listeners are opened.
func Open(t *testing.T, publicURL, databasePath string, operators ...string) *Fixture {
	t.Helper()
	ctx := t.Context()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repository, err := repositories.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	identities := accounts.NewService(repository)
	addressbook := contacts.New(repository)
	conversations := chat.New(repository)
	roomPolicy := rooms.New(repository)
	privacy := accounts.NewPrivacy(repository, repository)
	products := catalog.New(repository)
	credits := commerce.New(repository, privacy, products)
	groups := community.New(repository, privacy, roomPolicy)
	photos := media.New(repository, privacy)
	updates := activity.New(repository, privacy, repository)
	reviews := review.New(repository, repository, operators)
	customization := accounts.NewCustomization(repository, photos)
	core, err := realtime.New(realtime.Dependencies{
		Accounts:  identities,
		Contacts:  addressbook,
		Chat:      conversations,
		Rooms:     roomPolicy,
		Catalog:   products,
		Commerce:  credits,
		Community: groups,
		Media:     photos,
		Activity:  updates,
		Review:    reviews,
		Privacy:   privacy,
		Logger:    logger,
	})
	if err != nil {
		t.Fatal(errors.Join(err, repository.Close()))
	}

	if err := core.Initialize(ctx); err != nil {
		_ = core.Close()
		_ = repository.Close()
		t.Fatal(err)
	}
	if err := core.ConfigureOperators(operators); err != nil {
		_ = core.Close()
		_ = repository.Close()
		t.Fatal(err)
	}
	documents := legacyweb.New(legacyweb.Dependencies{
		PublicURL:     publicURL,
		Logger:        logger,
		Accounts:      identities,
		Customization: customization,
		Privacy:       privacy,
		Catalog:       products,
		Commerce:      credits,
		Community:     groups,
		Media:         photos,
		Activity:      updates,
		Review:        reviews,
		Rooms:         roomPolicy,
		Contacts:      addressbook,
		Chat:          conversations,
		Workflow:      core,
	})
	codec := nativecodec.New(testassets.FS())
	protocol := native.New(core, documents, publicURL, codec)
	fixture := &Fixture{Core: core, Native: protocol, Pages: documents, Review: reviews, Repository: repository}
	dependencies := controllers.Dependencies{
		Sessions:  core,
		Chat:      core,
		Contacts:  core,
		Pages:     documents,
		Media:     documents,
		Operator:  core,
		Native:    protocol,
		Language:  codec,
		PublicURL: publicURL,
		Catalog: func(ctx context.Context) (catalog.CatalogSnapshot, error) {
			return catalog.Snapshot(ctx, publicURL)
		},
	}

	fixture.Handler = httptransport.New(controllers.New(dependencies))
	runContext, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- core.Run(runContext) }()
	var once sync.Once
	var closeErr error
	fixture.close = func() error {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				if err != nil && !errors.Is(err, context.Canceled) {
					closeErr = err
				}
			case <-time.After(5 * time.Second):
				closeErr = errors.New("fixture maintenance worker did not stop")
			}
			closeErr = errors.Join(closeErr, core.Close(), repository.Close())
		})
		return closeErr
	}
	t.Cleanup(func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	})

	return fixture
}

func (f *Fixture) Close() error { return f.close() }
