package service

import (
	"context"
	"enterprise/internal/model"
	"enterprise/internal/repository"
	"enterprise/pkg/cache"
	"enterprise/pkg/rabbitmq"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
)

type LikeService struct {
	likeRepo   *repository.LikeRepository
	videoRepo  *repository.VideoRepository
	cache      *cache.Client
	likeMQ     *rabbitmq.LikeMQ
	popularity *rabbitmq.PopularityMQ
}

func NewLikeService(likeRepo *repository.LikeRepository, videoRepo *repository.VideoRepository, cache *cache.Client, likeMQ *rabbitmq.LikeMQ, popularityMQ *rabbitmq.PopularityMQ) *LikeService {
	return &LikeService{likeRepo: likeRepo, videoRepo: videoRepo, cache: cache, likeMQ: likeMQ, popularity: popularityMQ}
}

// IsLiked reports whether the given account has liked the target video.
func (s *LikeService) IsLiked(ctx context.Context, videoID, accountID uint) (bool, error) {
	return s.likeRepo.IsLiked(ctx, videoID, accountID)
}

// ListLikedVideos returns videos liked by the account, ordered by like time.
func (s *LikeService) ListLikedVideos(ctx context.Context, accountID uint) ([]model.Video, error) {
	return s.likeRepo.ListLikedVideos(ctx, accountID)
}

// isDupKey checks whether the error is a MySQL duplicate-key (1062) error.
func isDupKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// Like handles one like action with a "MQ first, local fallback" strategy:
// 1) validate request and guard duplicate likes;
// 2) publish like + popularity events to MQ;
// 3) if MQ publish fails, fallback to direct persistence/cache update.
func (s *LikeService) Like(ctx context.Context, like *model.Like) error {
	if like == nil {
		return errors.New("like is nil")
	}
	if like.VideoID == 0 || like.AccountID == 0 {
		return errors.New("video_id and account_id are required")
	}

	if s.videoRepo != nil {
		ok, err := s.videoRepo.IsExist(ctx, like.VideoID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("video not found")
		}
	}

	isLiked, err := s.likeRepo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if isLiked {
		return errors.New("user has liked this video")
	}

	like.CreatedAt = time.Now()
	mysqlEnqueued := false
	redisEnqueued := false
	if s.likeMQ != nil {
		if err := s.likeMQ.Like(ctx, like.AccountID, like.VideoID); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularity != nil {
		if err := s.popularity.Update(ctx, like.VideoID, 1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	// Fallback: direct MySQL write when like MQ publish fails.
	if !mysqlEnqueued {
		if err := s.fallbackPersistLike(ctx, like); err != nil {
			return err
		}
	}

	// Fallback: direct Redis update when popularity MQ publish fails.
	if !redisEnqueued {
		cache.UpdatePopularityCache(ctx, s.cache, like.VideoID, 1)
	}
	return nil
}

// fallbackPersistLike writes the like record and related counters directly
// when like MQ publish fails. Duplicate likes are normalized as business errors.
func (s *LikeService) fallbackPersistLike(ctx context.Context, like *model.Like) error {
	created, err := s.likeRepo.LikeIgnoreDuplicate(ctx, like)
	if err != nil {
		if isDupKey(err) {
			return errors.New("user has liked this video")
		}
		return err
	}
	if !created {
		return errors.New("user has liked this video")
	}

	if s.videoRepo != nil {
		if err := s.videoRepo.ChangeLikesCount(ctx, like.VideoID, 1); err != nil {
			return err
		}
		if err := s.videoRepo.ChangePopularity(ctx, like.VideoID, 1); err != nil {
			return err
		}
	}
	return nil
}
