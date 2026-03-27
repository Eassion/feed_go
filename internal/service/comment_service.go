package service

import (
	"context"
	"enterprise/internal/model"
	"enterprise/internal/repository"
	"enterprise/pkg/cache"
	"enterprise/pkg/rabbitmq"
	"errors"
	"strings"
)

type CommentService struct {
	commentRepo  *repository.CommentRepository
	videoRepo    *repository.VideoRepository
	cache        *cache.Client
	commentMQ    *rabbitmq.CommentMQ
	popularityMQ *rabbitmq.PopularityMQ
}

func NewCommentService(repo *repository.CommentRepository, videoRepo *repository.VideoRepository, cache *cache.Client, commentMQ *rabbitmq.CommentMQ, popularityMQ *rabbitmq.PopularityMQ) *CommentService {
	return &CommentService{commentRepo: repo, videoRepo: videoRepo, cache: cache, commentMQ: commentMQ, popularityMQ: popularityMQ}
}

func (s *CommentService) Publish(ctx context.Context, comment *model.Comment) error {
	if comment == nil {
		return errors.New("comment is nil")
	}
	comment.Username = strings.TrimSpace(comment.Username)
	comment.Content = strings.TrimSpace(comment.Content)
	if comment.VideoID == 0 || comment.AuthorID == 0 {
		return errors.New("video_id and author_id are required")
	}
	if comment.Content == "" {
		return errors.New("content is required")
	}

	exists, err := s.videoRepo.IsExist(ctx, comment.VideoID)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("video not found")
	}

	mysqlEnqueued := false
	redisEnqueued := false
	if s.commentMQ != nil {
		if err := s.commentMQ.Publish(ctx, comment.Username, comment.VideoID, comment.AuthorID, comment.Content); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, comment.VideoID, 1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	// Fallback: direct MySQL write when comment MQ publish fails.
	if !mysqlEnqueued {
		if err := s.fallbackPersistComment(ctx, comment); err != nil {
			return err
		}
	}

	// Fallback: direct Redis update when popularity MQ publish fails.
	if !redisEnqueued {
		cache.UpdatePopularityCache(ctx, s.cache, comment.VideoID, 1)
	}
	return nil
}

func (s *CommentService) fallbackPersistComment(ctx context.Context, comment *model.Comment) error {
	if err := s.commentRepo.CreateComment(ctx, comment); err != nil {
		return err
	}
	if s.videoRepo != nil {
		if err := s.videoRepo.ChangePopularity(ctx, comment.VideoID, 1); err != nil {
			return err
		}
	}
	return nil
}

func (s *CommentService) Delete(ctx context.Context, commentID uint, accountID uint) error {
	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment == nil {
		return errors.New("comment not found")
	}
	if comment.AuthorID != accountID {
		return errors.New("permission denied")
	}
	if s.commentMQ != nil {
		if err := s.commentMQ.Delete(ctx, commentID); err == nil {
			return nil
		}
	}
	return s.commentRepo.DeleteComment(ctx, comment)
}

func (s *CommentService) GetAll(ctx context.Context, videoID uint) ([]model.Comment, error) {
	exists, err := s.videoRepo.IsExist(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("video not found")
	}
	return s.commentRepo.GetAllComments(ctx, videoID)
}
