package repository

import (
	"context"
	"enterprise/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LikeRepository struct {
	db *gorm.DB
}


func NewLikeRepository(db *gorm.DB) *LikeRepository {
	return &LikeRepository{db: db}
}

// likeModelQuery 返回绑定上下文的 like 表基础查询对象。
func (r *LikeRepository) likeModelQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&model.Like{})
}

// videoModelQuery 返回绑定上下文的 video 表基础查询对象。
func (r *LikeRepository) videoModelQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&model.Video{})
}

// Like 创建一条点赞记录。
func (r *LikeRepository) Like(ctx context.Context, like *model.Like) error {
	return r.db.WithContext(ctx).Create(like).Error
}

// Unlike 按点赞对象删除对应点赞记录。
func (r *LikeRepository) Unlike(ctx context.Context, like *model.Like) error {
	if like == nil {
		return nil
	}
	return r.likeModelQuery(ctx).
		Where("video_id = ? AND account_id = ?", like.VideoID, like.AccountID).
		Delete(&model.Like{}).Error
}

// LikeIgnoreDuplicate 创建点赞记录，若重复点赞则忽略且不报错。
// 返回 created 表示本次是否真正创建了新记录。
func (r *LikeRepository) LikeIgnoreDuplicate(ctx context.Context, like *model.Like) (created bool, err error) {
	if like == nil || like.VideoID == 0 || like.AccountID == 0 {
		return false, nil
	}
	res := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(like)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// DeleteByVideoAndAccount 按 videoID 和 accountID 删除点赞记录。
// 返回 deleted 表示是否删除到数据。
func (r *LikeRepository) DeleteByVideoAndAccount(ctx context.Context, videoID, accountID uint) (deleted bool, err error) {
	if videoID == 0 || accountID == 0 {
		return false, nil
	}
	res := r.likeModelQuery(ctx).
		Where("video_id = ? AND account_id = ?", videoID, accountID).
		Delete(&model.Like{})
	return res.RowsAffected > 0, res.Error
}

// IsLiked 判断指定用户是否点赞过指定视频。
func (r *LikeRepository) IsLiked(ctx context.Context, videoID, accountID uint) (bool, error) {
	var count int64
	err := r.likeModelQuery(ctx).
		Where("video_id = ? AND account_id = ?", videoID, accountID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// BatchGetLiked 批量判断指定用户对一组视频的点赞状态。
// 返回 map 的 key 为 videoID，value 为 true 表示已点赞。
func (r *LikeRepository) BatchGetLiked(ctx context.Context, videoIDs []uint, accountID uint) (map[uint]bool, error) {
	likeMap := make(map[uint]bool)
	if len(videoIDs) == 0 || accountID == 0 {
		return likeMap, nil
	}

	var likes []model.Like
	err := r.likeModelQuery(ctx).
		Where("video_id IN ? AND account_id = ?", videoIDs, accountID).
		Find(&likes).Error
	if err != nil {
		return nil, err
	}

	for _, like := range likes {
		likeMap[like.VideoID] = true
	}
	return likeMap, nil
}

// ListLikedVideos 查询指定用户点赞过的视频列表。
// 结果按点赞时间倒序返回。
func (r *LikeRepository) ListLikedVideos(ctx context.Context, accountID uint) ([]model.Video, error) {
	var videos []model.Video
	if accountID == 0 {
		return videos, nil
	}
	err := r.videoModelQuery(ctx).
		Joins("JOIN likes ON likes.video_id = videos.id").
		Where("likes.account_id = ?", accountID).
		Order("likes.created_at DESC").
		Find(&videos).Error
	if err != nil {
		return nil, err
	}
	return videos, nil
}
