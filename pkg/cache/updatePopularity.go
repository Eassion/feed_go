package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

func UpdatePopularityCache(ctx context.Context, cache *Client, id uint, change int64) {
	if cache == nil || cache.RDB == nil || id == 0 || change == 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Minute)
	windowKey := "hot:video:1m:" + now.Format("200601021504")
	member := strconv.FormatUint(uint64(id), 10)

	opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	_ = cache.RDB.ZIncrBy(opCtx, windowKey, float64(change), member).Err()
	_ = cache.RDB.Expire(opCtx, windowKey, 2*time.Hour).Err()
	_ = cache.RDB.Del(context.Background(), fmt.Sprintf("video:detail:id=%d", id)).Err()
}
