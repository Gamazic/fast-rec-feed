package api

import (
	"context"
	"fastapp/internal/feed"
	"fastapp/internal/metrics"
	"fastapp/internal/storage"
	"fastapp/internal/updater"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type App struct {
	feedService *feed.Service
	fiberApp    *fiber.App
}

func NewApp() *App {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	feedStorage := storage.NewStorage()
	fixedFeedStorage := storage.NewGoldenFixedStorage()
	metrics := metrics.NewDummyMetrics()
	feedService := feed.NewService(feedStorage, fixedFeedStorage, metrics, logger)

	app := &App{
		feedService: feedService,
		fiberApp:    fiber.New(fiber.Config{}),
	}
	app.fiberApp.Get("/feed/:userId", app.feedHandler)
	updater.UpdateFeed(context.Background(), feedStorage, 5_000_000, 10_000_000)
	go func() {
		for {
			time.Sleep(5 * time.Second)
			n, p := feedStorage.GetPercentileExceed()
			logger.Debug("num exceed", "num", n, "p", p)
		}
	}()
	return app
}

func (a *App) feedHandler(ctx *fiber.Ctx) error {
	// Get userId from path params
	userId, err := ctx.ParamsInt("userId")
	if err != nil {
		return ctx.Status(fiber.StatusUnprocessableEntity).SendString("userId is required")
	}
	// Get optional size from query params
	size := ctx.QueryInt("size", 0)

	// Call feed service to get items
	feed, err := a.feedService.RetrievFeed(ctx.Context(), feed.FeedRequest{
		UserId: uint32(userId),
		Size:   uint8(size),
	})
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Format feed items as array string
	var sb strings.Builder
	sb.Grow(len(feed) * 3)
	sb.WriteString("[")
	for i, id := range feed {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.FormatUint(uint64(id), 10))
	}
	sb.WriteString("]")

	// Return response
	return ctx.Status(fiber.StatusOK).SendString(sb.String())
}

func (a *App) Run() error {
	return a.fiberApp.Listen(":8080")
}
