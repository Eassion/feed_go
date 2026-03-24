package repository

import (
	"context"
	"enterprise/internal/model"
	"time"

	"gorm.io/gorm"
)

type FeedRepository struct {
	db *gorm.DB
}

// NewFeedRepository 创建并返回一个基于 gorm DB 的 FeedRepository。
func NewFeedRepository(db *gorm.DB) *FeedRepository {
	return &FeedRepository{db: db}
}

// videoModelQuery 返回绑定上下文的 video 表基础查询对象。
func (r *FeedRepository) videoModelQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&model.Video{})
}

// ListLatest 按 create_time 倒序返回最新视频列表。
// 当 latestBefore 非零值时，只返回该时间点之前创建的视频。
func (r *FeedRepository) ListLatest(ctx context.Context, limit int, latestBefore time.Time) ([]*model.Video, error) {
	var videos []*model.Video
	query := r.videoModelQuery(ctx).Order("create_time DESC")
	if !latestBefore.IsZero() {
		query = query.Where("create_time < ?", latestBefore)
	}
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// ListLikesCountWithCursor 按 likes_count、id 倒序返回视频列表。
// cursor 用于稳定分页，避免在 likes_count 相同场景下出现重复或漏数。
func (r *FeedRepository) ListLikesCountWithCursor(ctx context.Context, limit int, cursor *model.LikesCountCursor) ([]*model.Video, error) {
	var videos []*model.Video
	query := r.videoModelQuery(ctx).
		Order("likes_count DESC, id DESC")

	if cursor != nil {
		query = query.Where(
			"(likes_count < ?) OR (likes_count = ? AND id < ?)",
			cursor.LikesCount,
			cursor.LikesCount, cursor.ID,
		)
	}

	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// ListByFollowing 返回 viewerAccountID 关注作者发布的视频列表。
// 结果按 create_time 倒序，并可通过 latestBefore 进行时间游标分页。
func (r *FeedRepository) ListByFollowing(ctx context.Context, limit int, viewerAccountID uint, latestBefore time.Time) ([]*model.Video, error) {
	var videos []*model.Video
	query := r.videoModelQuery(ctx).
		Order("create_time DESC")
	if viewerAccountID > 0 {
		followingSubQuery := r.db.WithContext(ctx).
			Model(&model.Social{}).
			Select("vlogger_id").
			Where("follower_id = ?", viewerAccountID)
		query = query.Where("author_id IN ?", followingSubQuery)
	}
	if !latestBefore.IsZero() {
		query = query.Where("create_time < ?", latestBefore)
	}
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// ListByPopularity 按 popularity、create_time、id 倒序返回视频列表。
// popularityBefore、timeBefore、idBefore 共同组成分页游标，用于稳定翻页。
func (r *FeedRepository) ListByPopularity(ctx context.Context, limit int, popularityBefore int64, timeBefore time.Time, idBefore uint) ([]*model.Video, error) {
	var videos []*model.Video
	query := r.videoModelQuery(ctx).
		Order("popularity DESC, create_time DESC, id DESC")

	// Only apply cursor filter when time/id cursor is complete.
	if !timeBefore.IsZero() && idBefore > 0 {
		query = query.Where(
			"(popularity < ?) OR (popularity = ? AND create_time < ?) OR (popularity = ? AND create_time = ? AND id < ?)",
			popularityBefore,
			popularityBefore, timeBefore,
			popularityBefore, timeBefore, idBefore,
		)
	}

	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// GetByIDs 按给定 ID 列表批量查询视频。
// 当 ids 为空时，直接返回空切片。
func (r *FeedRepository) GetByIDs(ctx context.Context, ids []uint) ([]*model.Video, error) {
	var videos []*model.Video
	if len(ids) == 0 {
		return videos, nil
	}
	if err := r.videoModelQuery(ctx).
		Where("id IN ?", ids).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}
