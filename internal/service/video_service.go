package service

import (
	"context"
	"encoding/json"
	"enterprise/internal/model"
	"enterprise/internal/repository"
	"enterprise/pkg/cache"
	"enterprise/pkg/rabbitmq"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type VideoService struct {
	videoRepo    *repository.VideoRepository
	cacheClient  *cache.Client
	cacheTTL     time.Duration
	popularityMQ *rabbitmq.PopularityMQ
}

func NewVideoService(repo *repository.VideoRepository, client *cache.Client, popularityMQ *rabbitmq.PopularityMQ) *VideoService {
	return &VideoService{videoRepo: repo, cacheClient: client, cacheTTL: 5 * time.Minute, popularityMQ: popularityMQ}
}

func (s *VideoService) Publish(ctx context.Context, video *model.Video) error {
	if video == nil {
		return errors.New("video is required")
	}

	video.Title = strings.TrimSpace(video.Title)
	video.Description = strings.TrimSpace(video.Description)
	video.CoverURL = strings.TrimSpace(video.CoverURL)
	if video.Title == "" || video.Description == "" || video.CoverURL == "" {
		return errors.New("title, description and coverURL are required")
	}
	if err := s.videoRepo.InTransaction(ctx, func(txRepo *repository.VideoRepository) error {
		if err := txRepo.CreateVideo(ctx, video); err != nil {
			return err
		}
		msg := &model.OutboxMsg{
			VideoID:    video.ID,
			EventType:  "video_published",
			Status:     "pending",
			CreateTime: video.CreateTime,
		}

		if err := txRepo.CreateMsg(ctx, msg); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *VideoService) Delete(ctx context.Context, id uint, authorID uint) error {
	video, err := s.videoRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if video == nil {
		return errors.New("video not found")
	}
	if video.AuthorID != authorID {
		return errors.New("unauthorized")
	}
	if err := s.videoRepo.DeleteVideo(ctx, id); err != nil {
		return err
	}
	if s.cacheClient != nil {
		cacheKey := fmt.Sprintf("video:detail:id=%d", id)
		_ = s.cacheClient.RDB.Del(context.Background(), cacheKey)
	}
	return nil
}

func (s *VideoService) ListByAuthorID(ctx context.Context, authorID uint) ([]model.Video, error) {
	videos, err := s.videoRepo.ListByAuthorID(ctx, int64(authorID))
	if err != nil {
		return nil, err
	}
	return videos, nil
}

func (s *VideoService) GetDetail(ctx context.Context, id uint) (*model.Video, error) {
	const (
		cacheOpTimeout = 50 * time.Millisecond
		lockTTL        = 2 * time.Second
		retryCount     = 5
		retryInterval  = 20 * time.Millisecond
	)

	cacheKey := fmt.Sprintf("video:detail:id=%d", id)
	lockKey := "lock:" + cacheKey

	getFromDB := func() (*model.Video, error) {
		return s.videoRepo.GetByID(ctx, id)
	}

	if s.cacheClient == nil || s.cacheClient.RDB == nil {
		return getFromDB()
	}

	if v, ok := s.getDetailFromCache(ctx, cacheKey, cacheOpTimeout); ok {
		return v, nil
	}

	lockCtx, lockCancel := context.WithTimeout(ctx, cacheOpTimeout)
	token, locked, lockErr := s.cacheClient.Lock(lockCtx, lockKey, lockTTL)
	lockCancel()

	if lockErr == nil && locked {
		defer func() { _ = s.cacheClient.Unlock(context.Background(), lockKey, token) }()

		// Double-check to avoid duplicate DB reads if another request has already filled cache.
		if v, ok := s.getDetailFromCache(ctx, cacheKey, cacheOpTimeout); ok {
			return v, nil
		}

		video, err := getFromDB()
		if err != nil {
			return nil, err
		}
		s.setDetailCache(ctx, cacheKey, video, cacheOpTimeout)
		return video, nil
	}

	// No lock acquired: wait briefly for the lock holder to repopulate cache.
	for i := 0; i < retryCount; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryInterval):
		}
		if v, ok := s.getDetailFromCache(ctx, cacheKey, cacheOpTimeout); ok {
			return v, nil
		}
	}

	video, err := getFromDB()
	if err != nil {
		return nil, err
	}
	s.setDetailCache(ctx, cacheKey, video, cacheOpTimeout)
	return video, nil
}

func (s *VideoService) getDetailFromCache(ctx context.Context, key string, timeout time.Duration) (*model.Video, bool) {
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	b, err := s.cacheClient.RDB.Get(opCtx, key).Bytes()
	if err != nil {
		return nil, false
	}

	var cached model.Video
	if err := json.Unmarshal(b, &cached); err != nil {
		return nil, false
	}
	return &cached, true
}

func (s *VideoService) setDetailCache(ctx context.Context, key string, video *model.Video, timeout time.Duration) {
	b, err := json.Marshal(video)
	if err != nil {
		return
	}

	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_ = s.cacheClient.RDB.Set(opCtx, key, b, s.cacheTTL).Err()
}

func (s *VideoService) UpdateLikesCount(ctx context.Context, id uint, likesCount int64) error {
	if err := s.videoRepo.UpdateLikesCount(ctx, id, likesCount); err != nil {
		return err
	}
	return nil
}

// TODO:这个方法暂时还没有弄懂
func (s *VideoService) UpdatePopularity(ctx context.Context, id uint, change int64) error {
	if err := s.videoRepo.UpdatePopularity(ctx, id, change); err != nil {
		return err
	}

	if s.tryUpdatePopularityViaMQ(ctx, id, change) {
		return nil
	}
	s.fallbackUpdatePopularityCache(ctx, id, change)
	return nil
}

func (s *VideoService) tryUpdatePopularityViaMQ(ctx context.Context, id uint, change int64) bool {
	if s.popularityMQ == nil {
		return false
	}
	return s.popularityMQ.Update(ctx, id, change) == nil
}


func (s *VideoService) fallbackUpdatePopularityCache(ctx context.Context, id uint, change int64) {
	const (
		cacheOpTimeout = 50 * time.Millisecond
		windowTTL      = 2 * time.Hour
	)

	if s.cacheClient == nil || s.cacheClient.RDB == nil {
		return
	}

	// Invalidate detail cache to avoid stale popularity in detail API.
	_ = s.cacheClient.RDB.Del(context.Background(), fmt.Sprintf("video:detail:id=%d", id))

	// Update current 1-minute hotness window by video id.
	now := time.Now().UTC().Truncate(time.Minute)
	windowKey := "hot:video:1m:" + now.Format("200601021504")
	member := strconv.FormatUint(uint64(id), 10)

	opCtx, cancel := context.WithTimeout(ctx, cacheOpTimeout)
	defer cancel()

	_ = s.cacheClient.RDB.ZIncrBy(opCtx, windowKey, float64(change), member)
	_ = s.cacheClient.RDB.Expire(opCtx, windowKey, windowTTL)
}
