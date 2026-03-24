package repository

import (
	"context"
	"enterprise/internal/model"
	"errors"

	"gorm.io/gorm"
)

type CommentRepository struct {
	db *gorm.DB
}

// NewCommentRepository 创建并返回一个基于 gorm DB 的 CommentRepository。
func NewCommentRepository(db *gorm.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

// commentModelQuery 返回绑定上下文的 comment 表基础查询对象。
func (r *CommentRepository) commentModelQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&model.Comment{})
}

// CreateComment 创建一条评论记录。
func (r *CommentRepository) CreateComment(ctx context.Context, comment *model.Comment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

// DeleteComment 删除一条评论记录。
func (r *CommentRepository) DeleteComment(ctx context.Context, comment *model.Comment) error {
	return r.db.WithContext(ctx).Delete(comment).Error
}

// GetAllComments 查询指定视频下的全部评论。
func (r *CommentRepository) GetAllComments(ctx context.Context, videoID uint) ([]model.Comment, error) {
	var comments []model.Comment
	err := r.commentModelQuery(ctx).
		Where("video_id = ?", videoID).
		Find(&comments).Error
	return comments, err
}

// IsExist 判断指定评论 ID 是否存在。
func (r *CommentRepository) IsExist(ctx context.Context, id uint) (bool, error) {
	var comment model.Comment
	if err := r.db.WithContext(ctx).First(&comment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetByID 按评论 ID 查询单条评论详情。
// 当记录不存在时，返回 nil, nil。
func (r *CommentRepository) GetByID(ctx context.Context, id uint) (*model.Comment, error) {
	var comment model.Comment
	if err := r.db.WithContext(ctx).First(&comment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &comment, nil
}
