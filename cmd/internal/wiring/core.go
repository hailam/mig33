// Package wiring assembles the concrete capabilities shared by the commands.
// Each executable selects its own transports and lifecycle components.
package wiring

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sync"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/adapters/legacyweb"
	"github.com/hailam/mig33/internal/adapters/native"
	"github.com/hailam/mig33/internal/adapters/nativecodec"
	"github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/community"
	"github.com/hailam/mig33/internal/config"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/controllers"
	"github.com/hailam/mig33/internal/media"
	"github.com/hailam/mig33/internal/realtime"
	"github.com/hailam/mig33/internal/review"
	"github.com/hailam/mig33/internal/rooms"
)

// Graph exposes the concrete objects entrypoints select. It provides no
// forwarding methods and does not know whether a command serves HTTP or sockets.
type Graph struct {
	Core   *realtime.Coordinator
	Native *native.Adapter
	Pages  *legacyweb.Adapter
	Review *review.Service
	HTTP   controllers.Dependencies
}

// Open initializes one capability graph over one SQLite pool. The returned
// cleanup runs after transports have drained and the core worker has joined.
func Open(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Graph, func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if logger == nil {
		return nil, nil, fmt.Errorf("logger is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	repository, err := repositories.Open(ctx, cfg.Database.Path)
	if err != nil {
		return nil, nil, err
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
	reviews := review.New(repository, repository, cfg.Operators)
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
		return nil, nil, errors.Join(err, repository.Close())
	}

	var once sync.Once
	var closeErr error
	cleanup := func() error {
		once.Do(func() { closeErr = errors.Join(core.Close(), repository.Close()) })
		return closeErr
	}

	if err := core.Initialize(ctx); err != nil {
		return nil, nil, errors.Join(err, cleanup())
	}
	if err := core.ConfigureOperators(cfg.Operators); err != nil {
		return nil, nil, errors.Join(err, cleanup())
	}
	documents := legacyweb.New(legacyweb.Dependencies{
		PublicURL:     cfg.PublicURL,
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

	var resources fs.FS
	if cfg.Resources.Directory != "" {
		resources = os.DirFS(cfg.Resources.Directory)
	}
	codec := nativecodec.New(resources)
	protocol := native.New(core, documents, cfg.PublicURL, codec)

	graph := &Graph{Core: core, Native: protocol, Pages: documents, Review: reviews}
	graph.HTTP = controllers.Dependencies{
		Sessions:  core,
		Chat:      core,
		Contacts:  core,
		Pages:     documents,
		Media:     documents,
		Language:  codec,
		Operator:  core,
		Native:    protocol,
		PublicURL: cfg.PublicURL,
		Catalog: func(ctx context.Context) (catalog.CatalogSnapshot, error) {
			return catalog.Snapshot(ctx, cfg.PublicURL)
		},
	}

	return graph, cleanup, nil
}
