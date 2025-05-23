package storage

import (
	"context"
	"math/rand"
)

type FixedStorage []uint32

func NewGoldenFixedStorage() FixedStorage {
	// Here can be stored golden feed for user
	return FixedStorage([]uint32{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		21, 22, 23, 24, 25, 26, 27, 28, 29, 30,
		31, 32, 33, 34, 35, 36, 37, 38, 39, 40,
		41, 42, 43, 44, 45, 46, 47, 48, 49, 50,
	})
}

func (s FixedStorage) RandomFeed(ctx context.Context, size uint8, excludeItems []uint32) []uint32 {
	if len(s) == 0 {
		return nil
	}

	excluded := make(map[uint32]struct{}, len(excludeItems)+int(size))
	for _, item := range excludeItems {
		excluded[item] = struct{}{}
	}

	result := make([]uint32, size)
	i := 0

	for i != int(size) && len(excluded) != len(s) {
		idx := rand.Intn(len(s))
		item := s[idx]

		if _, isExcluded := excluded[item]; !isExcluded {
			result[i] = item
			excluded[item] = struct{}{}
			i++
		}
	}

	return result
}
