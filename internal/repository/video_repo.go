package repository

import (
	"context"
	"enterprise/internal/model"
	"errors"

	"gorm.io/gorm"
)

type VideoRepository struct {
	db *gorm.DB
}

// NewVideoRepository 创建并返回一个基于 gorm DB 的 VideoRepository。
func NewVideoRepository(db *gorm.DB) *VideoRepository {
	return &VideoRepository{db: db}
}

// videoModelQuery 返回绑定上下文的 video 表基础查询对象。
func (r *VideoRepository) videoModelQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&model.Video{})
}

// outboxModelQuery 返回绑定上下文的 outbox 消息表基础查询对象。
func (r *VideoRepository) outboxModelQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&model.OutboxMsg{})
}

// CreateVideo 创建一条视频记录。
func (r *VideoRepository) CreateVideo(ctx context.Context, video *model.Video) error {
	return r.db.WithContext(ctx).Create(video).Error
}

// CreateMsg 创建一条 Outbox 消息记录。
func (r *VideoRepository) CreateMsg(ctx context.Context, msg *model.OutboxMsg) error {
	return r.db.WithContext(ctx).Create(msg).Error
}

// DeleteVideo 按视频 ID 删除视频记录。
func (r *VideoRepository) DeleteVideo(ctx context.Context, id uint) error {
	return r.videoModelQuery(ctx).Delete(&model.Video{}, id).Error
}

// ListByAuthorID 按作者 ID 查询视频列表，按创建时间倒序返回。
func (r *VideoRepository) ListByAuthorID(ctx context.Context, authorID int64) ([]model.Video, error) {
	var videos []model.Video
	if err := r.videoModelQuery(ctx).
		Where("author_id = ?", authorID).
		Order("create_time DESC").
		Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// GetByID 按视频 ID 查询单条视频详情。
func (r *VideoRepository) GetByID(ctx context.Context, id uint) (*model.Video, error) {
	var video model.Video
	if err := r.db.WithContext(ctx).First(&video, id).Error; err != nil {
		return nil, err
	}
	return &video, nil
}

// UpdateLikesCount 直接把点赞数更新为指定值。
func (r *VideoRepository) UpdateLikesCount(ctx context.Context, id uint, likesCount int64) error {
	return r.videoModelQuery(ctx).
		Where("id = ?", id).
		Update("likes_count", likesCount).Error
}

// IsExist 判断指定视频 ID 是否存在。
func (r *VideoRepository) IsExist(ctx context.Context, id uint) (bool, error) {
	var video model.Video
	if err := r.db.WithContext(ctx).First(&video, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// UpdatePopularity 按增量更新热度值，可为正数或负数。
func (r *VideoRepository) UpdatePopularity(ctx context.Context, id uint, change int64) error {
	return r.videoModelQuery(ctx).
		Where("id = ?", id).
		Update("popularity", gorm.Expr("popularity + ?", change)).Error
}

// ChangeLikesCount 按增量更新点赞数，且最小值限制为 0。
func (r *VideoRepository) ChangeLikesCount(ctx context.Context, id uint, change int64) error {
	return r.videoModelQuery(ctx).
		Where("id = ?", id).
		UpdateColumn("likes_count", gorm.Expr("GREATEST(likes_count + ?, 0)", change)).Error
}

// ChangePopularity 按增量更新热度值，且最小值限制为 0。
func (r *VideoRepository) ChangePopularity(ctx context.Context, id uint, change int64) error {
	return r.videoModelQuery(ctx).
		Where("id = ?", id).
		UpdateColumn("popularity", gorm.Expr("GREATEST(popularity + ?, 0)", change)).Error
}
