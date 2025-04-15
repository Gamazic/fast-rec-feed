package updater

import (
	"context"
	"fastapp/internal/feed"
	"fastapp/internal/storage"
	"math/rand"
)

// formula of data size: numOfUsers * feedSize * 4 bytes (uint32) / 1024 / 1024 (MB)
// for 1kk users and feed of size 200 around 768mb data will be stored
func UpdateFeed(ctx context.Context, feedStorage *storage.Storage, maxUserId uint32, maxVideoId uint32) {
	numUsers := rand.Intn(int(maxUserId)) + 1
	for i := range numUsers {
		var newFeed [feed.TotalFeedSize]uint32

		for j := 0; j < feed.TotalFeedSize; j++ {
			newFeed[j] = uint32(rand.Intn(int(maxVideoId))) + 1
		}
		feedStorage.SetFeed(ctx, uint32(i), newFeed)
	}
}
