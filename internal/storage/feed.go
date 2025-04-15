package storage

import (
	"context"
	"fastapp/internal/feed"
	"fmt"
	"sync"
	"sync/atomic"
)

type Storage struct {
	feeds     map[uint32][feed.TotalFeedSize]uint32
	offsets   sync.Map
	numExceed atomic.Uint64
}

func NewStorage() *Storage {
	return &Storage{
		feeds: make(map[uint32][feed.TotalFeedSize]uint32),
	}
}

func (s *Storage) GetNextFeed(ctx context.Context, userId uint32, size uint8) ([]uint32, error) {
	// Get current offset for user
	offsetVal, _ := s.offsets.Load(userId)
	var offset uint16
	if offsetVal != nil {
		offset = offsetVal.(uint16)
	}

	// Return empty if user has seen all items
	if int(offset) >= feed.TotalFeedSize {
		return nil, nil
	}

	// Calculate how many items to return, bounded by total feed size
	lastItem := min(int(offset)+int(size), feed.TotalFeedSize)
	if lastItem >= feed.TotalFeedSize {
		s.numExceed.Add(1)
	}

	// Get user's feed array and slice the requested portion
	feed, ok := s.feeds[userId]
	if !ok {
		return nil, fmt.Errorf("no feed found for user %d", userId)
	}
	items := feed[offset:lastItem]

	// Update user's offset
	s.offsets.Store(userId, uint16(lastItem))
	return items, nil
}

func (s *Storage) SetFeed(ctx context.Context, userId uint32, items [feed.TotalFeedSize]uint32) {
	s.feeds[userId] = items
	s.offsets.Store(userId, uint16(0))
}

func (s *Storage) GetPercentileExceed() (uint64, float64) {
	return s.numExceed.Load(), float64(s.numExceed.Load()) / float64(len(s.feeds))
}
