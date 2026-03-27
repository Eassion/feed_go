package service

import (
	"context"
	"encoding/json"
	"enterprise/internal/model"
	"enterprise/internal/repository"
	"enterprise/pkg/cache"
	"fmt"
	"strconv"
	"sync"
	"time"

	localCache "github.com/patrickmn/go-cache"
	redis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type FeedService struct {
	feedRepo     *repository.FeedRepository
	likeRepo     *repository.LikeRepository
	cache        *cache.Client
	localCache   *localCache.Cache
	cacheTTL     time.Duration
	requestGroup singleflight.Group
}

type CachedFeedData struct {
	PublicVideos []model.Video `json:"public_videos"`
}

func NewFeedService(repo *repository.FeedRepository, likeRepo *repository.LikeRepository, redisCache *cache.Client) *FeedService {
	return &FeedService{
		feedRepo:   repo,
		likeRepo:   likeRepo,
		cache:      redisCache,
		localCache: localCache.New(3*time.Second, 5*time.Second),
		cacheTTL:   24 * time.Hour,
	}
}


//根据ids三级缓存查询videos
func (f *FeedService) GetVideoByIDs(ctx context.Context, videoIDs []uint) ([]*model.Video, error) {
	if len(videoIDs) == 0 {
		return []*model.Video{}, nil
	}

	videoMap := make(map[uint]*model.Video, len(videoIDs))
	missed := f.fillFromLocalCache(videoIDs, videoMap)
	missed = f.fillFromRedis(ctx, missed, videoMap)
	f.fillFromMySQL(ctx, missed, videoMap)

	return buildOrderedResult(videoIDs, videoMap), nil
}

func (f *FeedService) fillFromLocalCache(videoIDs []uint, videoMap map[uint]*model.Video) []uint {
	if f.localCache == nil {
		return videoIDs
	}

	missed := make([]uint, 0, len(videoIDs))
	for _, id := range videoIDs {
		key := videoEntityCacheKey(id)
		v, found := f.localCache.Get(key)
		if !found {
			missed = append(missed, id)
			continue
		}

		cachedVideo, ok := v.(model.Video)
		if !ok {
			missed = append(missed, id)
			continue
		}

		safeCopy := cachedVideo
		videoMap[id] = &safeCopy
	}
	return missed
}

func (f *FeedService) fillFromRedis(ctx context.Context, missedIDs []uint, videoMap map[uint]*model.Video) []uint {
	if len(missedIDs) == 0 {
		return missedIDs
	}
	if f.cache == nil || f.cache.RDB == nil {
		return missedIDs
	}

	keys := make([]string, len(missedIDs))
	for i, id := range missedIDs {
		keys[i] = videoEntityCacheKey(id)
	}

	opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	results, err := f.cache.RDB.MGet(opCtx, keys...).Result()
	if err != nil {
		return missedIDs
	}

	missed := make([]uint, 0, len(missedIDs))
	for i, raw := range results {
		id := missedIDs[i]
		if raw == nil {
			missed = append(missed, id)
			continue
		}

		var b []byte
		switch v := raw.(type) {
		case string:
			b = []byte(v)
		case []byte:
			b = v
		default:
			missed = append(missed, id)
			continue
		}

		var cached model.Video
		if err := json.Unmarshal(b, &cached); err != nil {
			missed = append(missed, id)
			continue
		}

		safeCopy := cached
		videoMap[id] = &safeCopy
		if f.localCache != nil {
			f.localCache.Set(keys[i], safeCopy, 5*time.Second)
		}
	}
	return missed
}

func (f *FeedService) fillFromMySQL(ctx context.Context, missedIDs []uint, videoMap map[uint]*model.Video) {
	if len(missedIDs) == 0 {
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, id := range missedIDs {
		videoID := id
		wg.Add(1)

		go func() {
			defer wg.Done()

			sfKey := fmt.Sprintf("sf:entity:%d", videoID)
			v, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
				videos, err := f.feedRepo.GetByIDs(ctx, []uint{videoID})
				if err != nil || len(videos) == 0 {
					return nil, err
				}

				safeCopy := *videos[0]
				f.asyncWriteRedis(videoEntityCacheKey(safeCopy.ID), safeCopy)
				return &safeCopy, nil
			})
			if err != nil || v == nil {
				return
			}

			videoPtr, ok := v.(*model.Video)
			if !ok || videoPtr == nil {
				return
			}

			safeCopy := *videoPtr
			mu.Lock()
			videoMap[videoID] = &safeCopy
			mu.Unlock()

			if f.localCache != nil {
				f.localCache.Set(videoEntityCacheKey(safeCopy.ID), safeCopy, 5*time.Second)
			}
		}()
	}

	wg.Wait()
}

func (f *FeedService) asyncWriteRedis(key string, v model.Video) {
	if f.cache == nil || f.cache.RDB == nil {
		return
	}

	b, err := json.Marshal(v)
	if err != nil {
		return
	}

	go func(cacheKey string, payload []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_ = f.cache.RDB.Set(ctx, cacheKey, payload, f.cacheTTL).Err()
	}(key, b)
}

func buildOrderedResult(videoIDs []uint, videoMap map[uint]*model.Video) []*model.Video {
	result := make([]*model.Video, 0, len(videoIDs))
	for _, id := range videoIDs {
		if v, ok := videoMap[id]; ok && v != nil {
			result = append(result, v)
		}
	}
	return result
}

func videoEntityCacheKey(id uint) string {
	return fmt.Sprintf("video:entity:%d", id)
}



// ListLatest returns latest feed items with cold/hot split and cursor pagination.
func (f *FeedService) ListLatest(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (model.ListLatestResponse, error) {
	watermark, empty, err := f.globalTimelineWatermark(ctx)
	if err != nil {
		return model.ListLatestResponse{}, err
	}

	//redis  为空时，尝试重建全局时间线并获取水位线，避免后续访问大量穿透数据库。
	if empty {
		emptyDB, err := f.rebuildGlobalTimelineIfNeeded(ctx)
		if err != nil {
			return model.ListLatestResponse{}, err
		}
		if emptyDB {
			return model.ListLatestResponse{HasMore: false}, nil
		}
		watermark, _, err = f.globalTimelineWatermark(ctx)
		if err != nil {
			return model.ListLatestResponse{}, err
		}
	}

	//兼容第一页和后续分页两种状态
	reqTime := time.Now().UnixMilli()
	if !latestBefore.IsZero() {
		reqTime = latestBefore.UnixMilli()
	}

	//查询基础信息
	baseVideos, err := f.listLatestBaseVideos(ctx, limit, latestBefore, reqTime, watermark)
	if err != nil {
		return model.ListLatestResponse{}, err
	}

	//组床完成feed video item
	feedVideos, err := f.buildFeedVideos(ctx, baseVideos, viewerAccountID)
	if err != nil {
		return model.ListLatestResponse{}, err
	}

	//返回给前端下一次查询的游标
	nextTime := int64(0)
	if len(baseVideos) > 0 {
		nextTime = baseVideos[len(baseVideos)-1].CreateTime.UnixMilli()
	}

	return model.ListLatestResponse{
		VideoList: feedVideos,
		NextTime:  nextTime,
		HasMore:   len(baseVideos) == limit,
	}, nil
}

func (f *FeedService) globalTimelineWatermark(ctx context.Context) (watermark int64, empty bool, err error) {
	if f.cache == nil || f.cache.RDB == nil {
		return 0, true, nil
	}
	zsetTail, err := f.cache.RDB.ZRangeWithScores(ctx, "feed:global_timeline", 0, 0).Result()
	if err != nil {
		return 0, false, err
	}
	if len(zsetTail) == 0 {
		return 0, true, nil
	}
	return int64(zsetTail[0].Score), false, nil
}

func (f *FeedService) rebuildGlobalTimelineIfNeeded(ctx context.Context) (emptyDB bool, err error) {
	sfKey := "sf:fallback:global_timeline_rebuild"
	v, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
		dbVideos, err := f.feedRepo.ListLatest(ctx, 1000, time.Time{})
		if err != nil {
			return nil, err
		}
		if len(dbVideos) == 0 {
			return true, nil
		}
		if f.cache == nil || f.cache.RDB == nil {
			return false, nil
		}

		zElements := make([]redis.Z, 0, len(dbVideos))
		for _, vid := range dbVideos {
			zElements = append(zElements, redis.Z{
				Score:  float64(vid.CreateTime.UnixMilli()),
				Member: fmt.Sprintf("%d", vid.ID),
			})
		}
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = f.cache.RDB.ZAdd(bgCtx, "feed:global_timeline", zElements...).Err()
		return false, nil
	})
	if err != nil {
		return false, err
	}
	b, _ := v.(bool)
	return b, nil
}

// 处理冷热分离查询 包括单独在redis、redis和db结合
func (f *FeedService) listLatestBaseVideos(ctx context.Context, limit int, latestBefore time.Time, reqTime, watermark int64) ([]*model.Video, error) {
	
	// 请求时间点在水位线之前，说明请求的都是冷数据，直接从数据库查询即可，无需访问 Redis
	if reqTime <= watermark {
		sfKey := fmt.Sprintf("sf:cold:listLatest:%d:%d", limit, reqTime)
		v, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
			return f.feedRepo.ListLatest(ctx, limit, latestBefore)
		})
		if err != nil {
			return nil, err
		}
		videos, _ := v.([]*model.Video)
		return videos, nil
	}

	//从redis中尽可能地尝试拿limit个，可能不够
	videoIDs, err := f.listHotTimelineIDs(ctx, limit, latestBefore, reqTime)
	if err != nil {
		return nil, err
	}

	baseVideos := make([]*model.Video, 0, limit)
	if len(videoIDs) > 0 {
		baseVideos, err = f.GetVideoByIDs(ctx, videoIDs)
		if err != nil {
			return nil, err
		}
	}

	// 从数据库冷数据中补齐limit个视频
	if len(baseVideos) < limit {
		coldVideos, err := f.stitchColdLatest(ctx, limit-len(baseVideos), latestBefore, baseVideos)
		if err == nil {
			baseVideos = append(baseVideos, coldVideos...)
		}
	}

	return baseVideos, nil
}

// 从 Redis 中获取视频 ID 列表，查询的是所有符合条件的数据，返回一个videoIDs
func (f *FeedService) listHotTimelineIDs(ctx context.Context, limit int, latestBefore time.Time, reqTime int64) ([]uint, error) {
	if f.cache == nil || f.cache.RDB == nil {
		return nil, nil
	}

	//设置查询上限
	maxScore := "+inf"
	if !latestBefore.IsZero() {
		maxScore = fmt.Sprintf("%d", reqTime-1)
	}

	videoIDsStr, err := f.cache.RDB.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:    "feed:global_timeline",
		Start:  maxScore,
		Stop:   "-inf",
		ByScore: true,
		Rev:     true,
		Offset:  0,
		Count:   int64(limit),
	}).Result()
	if err != nil {
		return nil, err
	}

	videoIDs := make([]uint, 0, len(videoIDsStr))
	for _, idStr := range videoIDsStr {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			continue
		}
		videoIDs = append(videoIDs, uint(id))
	}
	return videoIDs, nil
}

//把redis中最早的视频当作查询数据库的游标，补齐limit个视频
func (f *FeedService) stitchColdLatest(ctx context.Context, remainLimit int, latestBefore time.Time, baseVideos []*model.Video) ([]*model.Video, error) {
	coldCursor := latestBefore
	if len(baseVideos) > 0 {
		coldCursor = baseVideos[len(baseVideos)-1].CreateTime
	}

	sfKey := fmt.Sprintf("sf:stitch:listLatest:%d:%d", remainLimit, coldCursor.UnixMilli())
	v, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
		return f.feedRepo.ListLatest(ctx, remainLimit, coldCursor)
	})
	if err != nil {
		return nil, err
	}
	coldVideos, _ := v.([]*model.Video)
	return coldVideos, nil
}

//组装video数据  包括是否点过赞
func (f *FeedService) buildFeedVideos(ctx context.Context, baseVideos []*model.Video, viewerAccountID uint) ([]model.FeedVideoItem, error) {
	if len(baseVideos) == 0 {
		return []model.FeedVideoItem{}, nil
	}

	videoIDs := make([]uint, 0, len(baseVideos))
	for _, v := range baseVideos {
		if v != nil {
			videoIDs = append(videoIDs, v.ID)
		}
	}

	likedMap := make(map[uint]bool)
	if viewerAccountID > 0 && f.likeRepo != nil {
		m, err := f.likeRepo.BatchGetLiked(ctx, videoIDs, viewerAccountID)
		if err != nil {
			return nil, err
		}
		likedMap = m
	}

	items := make([]model.FeedVideoItem, 0, len(baseVideos))
	for _, v := range baseVideos {
		if v == nil {
			continue
		}
		items = append(items, model.FeedVideoItem{
			ID: v.ID,
			Author: model.FeedAuthor{
				ID:       v.AuthorID,
				Username: v.Username,
			},
			Title:       v.Title,
			Description: v.Description,
			PlayURL:     v.PlayURL,
			CoverURL:    v.CoverURL,
			CreateTime:  v.CreateTime.UnixMilli(),
			LikesCount:  v.LikesCount,
			IsLiked:     likedMap[v.ID],
		})
	}
	return items, nil
}



// TODO:可改进：
// ListByPopularity upgrade directions:
// 1) Replace offset pagination with composite cursor pagination.
// Use (popularity, create_time, id) as the next-page cursor to reduce deep-page cost
// and avoid offset drift under concurrent updates.
//
// 2) Introduce snapshot_id with 10-minute TTL.
// Build and cache a session snapshot on first page; subsequent pages read by snapshot_id
// to keep pagination stable during a user session.
//
// 3) Read from precomputed versioned hot-rank keys.
// Generate immutable rank version keys (for example, rank:hot:v<timestamp>) in background,
// then serve reads from a fixed version key for better consistency and lower query latency.
func (f *FeedService) ListByPopularity(ctx context.Context, limit int, reqAsOf int64, offset int, viewerAccountID uint, latestPopularity int64, latestBefore time.Time, latestIDBefore uint) (model.ListByPopularityResponse, error) {
	resp, hit, err := f.listByPopularityFromRedis(ctx, limit, reqAsOf, offset, viewerAccountID)
	if err != nil {
		return model.ListByPopularityResponse{}, err
	}
	if hit {
		return resp, nil
	}

	return f.listByPopularityFromDB(ctx, limit, viewerAccountID, latestPopularity, latestBefore, latestIDBefore)
}

func (f *FeedService) listByPopularityFromRedis(ctx context.Context, limit int, reqAsOf int64, offset int, viewerAccountID uint) (model.ListByPopularityResponse, bool, error) {
	if f.cache == nil || f.cache.RDB == nil {
		return model.ListByPopularityResponse{}, false, nil
	}

	asOf := time.Now().UTC().Truncate(time.Minute)
	if reqAsOf > 0 {
		asOf = time.Unix(reqAsOf, 0).UTC().Truncate(time.Minute)
	}

	//生成前面60个分钟热榜
	const windowSize = 60
	keys := make([]string, 0, windowSize)
	for i := 0; i < windowSize; i++ {
		keys = append(keys, "hot:video:1m:"+asOf.Add(-time.Duration(i)*time.Minute).Format("200601021504"))
	}

	//合并热榜，并给这个热榜快照设置一个2h的过期时间
	dest := "hot:video:merge:1m:" + asOf.Format("200601021504")
	opCtx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()

	exists, _ := f.cache.RDB.Exists(opCtx, dest).Result()
	if exists == 0 {
		_ = f.cache.RDB.ZUnionStore(opCtx, dest, &redis.ZStore{Keys: keys, Aggregate: "SUM"}).Err()
		_ = f.cache.RDB.Expire(opCtx, dest, 2*time.Minute).Err()
	}

	//分页从快照查询
	start := int64(offset)
	stop := start + int64(limit) - 1
	members, err := f.cache.RDB.ZRangeArgs(opCtx, redis.ZRangeArgs{
		Key:   dest,
		Start: strconv.FormatInt(start, 10),
		Stop:  strconv.FormatInt(stop, 10),
		Rev:   true,
	}).Result()
	if err != nil {
		return model.ListByPopularityResponse{}, false, nil
	}

	//翻页到最后已经没有数据了  返回缓存命中，维持前端分页状态
	if len(members) == 0 && offset > 0 {
		return model.ListByPopularityResponse{
			VideoList:  []model.FeedVideoItem{},
			AsOf:       asOf.Unix(),
			NextOffset: offset,
			HasMore:    false,
		}, true, nil
	}
	if len(members) == 0 {
		return model.ListByPopularityResponse{}, false, nil
	}

	//根据ids批量获取视频信息
	ids := parsePositiveUintMembers(members)
	videos, err := f.feedRepo.GetByIDs(ctx, ids)
	if err != nil {
		return model.ListByPopularityResponse{}, false, nil
	}

	//按id排序，并加上请求者的点赞信息
	ordered := reorderVideosByID(ids, videos)
	items, err := f.buildFeedVideos(ctx, ordered, viewerAccountID)
	if err != nil {
		return model.ListByPopularityResponse{}, false, err
	}

	resp := model.ListByPopularityResponse{
		VideoList:  items,
		AsOf:       asOf.Unix(),
		NextOffset: offset + len(items),
		HasMore:    len(items) == limit,
	}
	//构建下一页游标
	populatePopularityCursor(&resp, ordered)
	return resp, true, nil
}

func (f *FeedService) listByPopularityFromDB(ctx context.Context, limit int, viewerAccountID uint, latestPopularity int64, latestBefore time.Time, latestIDBefore uint) (model.ListByPopularityResponse, error) {
	videos, err := f.feedRepo.ListByPopularity(ctx, limit, latestPopularity, latestBefore, latestIDBefore)
	if err != nil {
		return model.ListByPopularityResponse{}, err
	}

	items, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return model.ListByPopularityResponse{}, err
	}

	resp := model.ListByPopularityResponse{
		VideoList:  items,
		AsOf:       0,
		NextOffset: 0,
		HasMore:    len(items) == limit,
	}
	populatePopularityCursor(&resp, videos)
	return resp, nil
}

func parsePositiveUintMembers(members []string) []uint {
	ids := make([]uint, 0, len(members))
	for _, m := range members {
		u, err := strconv.ParseUint(m, 10, 64)
		if err == nil && u > 0 {
			ids = append(ids, uint(u))
		}
	}
	return ids
}

func reorderVideosByID(ids []uint, videos []*model.Video) []*model.Video {
	byID := make(map[uint]*model.Video, len(videos))
	for _, v := range videos {
		if v != nil {
			byID[v.ID] = v
		}
	}

	ordered := make([]*model.Video, 0, len(ids))
	for _, id := range ids {
		if v := byID[id]; v != nil {
			ordered = append(ordered, v)
		}
	}
	return ordered
}

//根据这次请求的最后一条视频信息构建下一页游标
func populatePopularityCursor(resp *model.ListByPopularityResponse, videos []*model.Video) {
	if len(videos) == 0 {
		return
	}

	last := videos[len(videos)-1]
	nextPopularity := last.Popularity
	nextBefore := last.CreateTime
	nextID := last.ID
	resp.NextLatestPopularity = &nextPopularity
	resp.NextLatestBefore = &nextBefore
	resp.NextLatestIDBefore = &nextID
}


// 按照点赞数查询视频
func (f *FeedService) ListLikesCount(ctx context.Context, limit int, cursor *model.LikesCountCursor, viewerAccountID uint) (model.ListLikesCountResponse, error) {
	videos, err := f.feedRepo.ListLikesCountWithCursor(ctx, limit, cursor)
	if err != nil {
		return model.ListLikesCountResponse{}, err
	}
	hasMore := len(videos) == limit
	feedVideos, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return model.ListLikesCountResponse{}, err
	}
	resp := model.ListLikesCountResponse{
		VideoList: feedVideos,
		HasMore:   hasMore,
	}
	if len(videos) > 0 {
		last := videos[len(videos)-1]
		nextLikesCountBefore := last.LikesCount
		nextIDBefore := last.ID
		resp.NextLikesCountBefore = &nextLikesCountBefore
		resp.NextIDBefore = &nextIDBefore
	}
	return resp, nil
}



// ListByFollowing returns followed-authors feed with cache and lock protection.
func (f *FeedService) ListByFollowing(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (model.ListByFollowingResponse, error) {
	cacheKey, canUseCache := f.listByFollowingCacheKey(limit, latestBefore, viewerAccountID)
	if canUseCache {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		cached, ok, miss := f.getListByFollowingFromCache(cacheCtx, cacheKey)
		if ok {
			return cached, nil
		}

		if miss {
			lockKey := "lock:" + cacheKey
			token, locked, _ := f.cache.Lock(cacheCtx, lockKey, 500*time.Millisecond)
			if locked {
				defer func() { _ = f.cache.Unlock(context.Background(), lockKey, token) }()

				cached, ok, _ = f.getListByFollowingFromCache(cacheCtx, cacheKey)
				if ok {
					return cached, nil
				}

				resp, err := f.listByFollowingFromDB(ctx, limit, latestBefore, viewerAccountID)
				if err != nil {
					return model.ListByFollowingResponse{}, err
				}
				f.setListByFollowingCache(cacheCtx, cacheKey, resp)
				return resp, nil
			}

			if cached, ok := f.waitListByFollowingCache(cacheCtx, cacheKey); ok {
				return cached, nil
			}
		}
	}

	resp, err := f.listByFollowingFromDB(ctx, limit, latestBefore, viewerAccountID)
	if err != nil {
		return model.ListByFollowingResponse{}, err
	}
	if canUseCache {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		f.setListByFollowingCache(cacheCtx, cacheKey, resp)
	}
	return resp, nil
}

func (f *FeedService) listByFollowingFromDB(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (model.ListByFollowingResponse, error) {
	videos, err := f.feedRepo.ListByFollowing(ctx, limit, viewerAccountID, latestBefore)
	if err != nil {
		return model.ListByFollowingResponse{}, err
	}

	nextTime := int64(0)
	if len(videos) > 0 {
		nextTime = videos[len(videos)-1].CreateTime.Unix()
	}

	feedVideos, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return model.ListByFollowingResponse{}, err
	}

	return model.ListByFollowingResponse{
		VideoList: feedVideos,
		NextTime:  nextTime,
		HasMore:   len(videos) == limit,
	}, nil
}

func (f *FeedService) listByFollowingCacheKey(limit int, latestBefore time.Time, viewerAccountID uint) (string, bool) {
	if viewerAccountID == 0 || f.cache == nil || f.cache.RDB == nil {
		return "", false
	}
	before := int64(0)
	if !latestBefore.IsZero() {
		before = latestBefore.Unix()
	}
	return fmt.Sprintf("feed:listByFollowing:limit=%d:accountID=%d:before=%d", limit, viewerAccountID, before), true
}

func (f *FeedService) getListByFollowingFromCache(ctx context.Context, cacheKey string) (model.ListByFollowingResponse, bool, bool) {
	b, err := f.cache.RDB.Get(ctx, cacheKey).Bytes()
	if err != nil {
		return model.ListByFollowingResponse{}, false, cache.IsMiss(err)
	}

	var cached model.ListByFollowingResponse
	if err := json.Unmarshal(b, &cached); err != nil {
		return model.ListByFollowingResponse{}, false, false
	}
	return cached, true, false
}

func (f *FeedService) setListByFollowingCache(ctx context.Context, cacheKey string, resp model.ListByFollowingResponse) {
	b, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_ = f.cache.RDB.Set(ctx, cacheKey, b, f.cacheTTL).Err()
}

func (f *FeedService) waitListByFollowingCache(ctx context.Context, cacheKey string) (model.ListByFollowingResponse, bool) {
	for i := 0; i < 5; i++ {
		time.Sleep(20 * time.Millisecond)
		cached, ok, _ := f.getListByFollowingFromCache(ctx, cacheKey)
		if ok {
			return cached, true
		}
	}
	return model.ListByFollowingResponse{}, false
}
