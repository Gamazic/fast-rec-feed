package feed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

type Service struct {
	feedStorage       feedStorage
	randomFeedStorage randomFeedStorage
	errRecorder       errRecorder
	logger            *slog.Logger
}

func NewService(feedStorage feedStorage, randomFeedStorage randomFeedStorage, errRecorder errRecorder, logger *slog.Logger) *Service {
	return &Service{
		feedStorage:       feedStorage,
		randomFeedStorage: randomFeedStorage,
		errRecorder:       errRecorder,
		logger:            logger,
	}
}

const (
	defailtNextFeedSize = 10
	TotalFeedSize       = 200
)

type FeedRequest struct {
	UserId uint32
	Size   uint8
}

func (f *Service) RetrievFeed(ctx context.Context, r FeedRequest) ([]uint32, error) {
	// Set default size if not specified
	if r.Size == 0 {
		r.Size = defailtNextFeedSize
	}

	var randomFeedSize uint8
	// Get personalized feed for user
	persFeed, err := f.feedStorage.NextFeed(ctx, r.UserId, r.Size)
	if err != nil {
		f.errRecorder.RecordFeedError(ctx, r.UserId, err)
	}
	randomFeedSize = r.Size - uint8(len(persFeed))

	// Fill remaining items with random feed
	if randomFeedSize > 0 {
		randomFeed := f.randomFeedStorage.RandomFeed(ctx, randomFeedSize, persFeed)
		persFeed = append(persFeed, randomFeed...)
	}

	// Validate final feed size
	if len(persFeed) != int(r.Size) {
		f.errRecorder.RecordFeedError(ctx, r.UserId, fmt.Errorf("feed size is not equal to requested size"))
		f.logger.ErrorContext(ctx, "critical error feed size is not equal to requested size",
			"userId", r.UserId,
			"randomFeedSize", randomFeedSize,
			"persFeedSize", len(persFeed),
			"requestedSize", r.Size)
		if len(persFeed) == 0 {
			return nil, errors.New("no feed items")
		}
	}

	return persFeed, nil
}

type feedStorage interface {
	NextFeed(ctx context.Context, userId uint32, size uint8) ([]uint32, error)
}

type randomFeedStorage interface {
	RandomFeed(ctx context.Context, size uint8, excludeItems []uint32) []uint32
}

type errRecorder interface {
	RecordFeedError(ctx context.Context, userId uint32, err error)
}
