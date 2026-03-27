package repository

import (
	"context"
	"enterprise/internal/model"

	"gorm.io/gorm"
)

type SocialRepository struct {
	db *gorm.DB
}

func NewSocialRepository(db *gorm.DB) *SocialRepository {
	return &SocialRepository{db: db}
}

func (r *SocialRepository) Follow(ctx context.Context, social *model.Social) error {
	return r.db.WithContext(ctx).Create(social).Error
}

func (r *SocialRepository) Unfollow(ctx context.Context, social *model.Social) error {
	return r.db.WithContext(ctx).
		Where("follower_id = ? AND vlogger_id = ?", social.FollowerID, social.VloggerID).
		Delete(&model.Social{}).Error
}

func (r *SocialRepository) GetAllFollowers(ctx context.Context, VloggerID uint) ([]*model.Account, error) {
	var relations []model.Social
	if err := r.db.WithContext(ctx).
		Model(&model.Social{}).
		Where("vlogger_id = ?", VloggerID).
		Find(&relations).Error; err != nil {
		return nil, err
	}

	followerIDs := make([]uint, 0, len(relations))
	for _, rel := range relations {
		followerIDs = append(followerIDs, rel.FollowerID)
	}
	if len(followerIDs) == 0 {
		return []*model.Account{}, nil
	}

	var followers []*model.Account
	if err := r.db.WithContext(ctx).
		Model(&model.Account{}).
		Where("id IN ?", followerIDs).
		Find(&followers).Error; err != nil {
		return nil, err
	}
	return followers, nil
}

func (r *SocialRepository) GetAllVloggers(ctx context.Context, FollowerID uint) ([]*model.Account, error) {
	var relations []model.Social
	if err := r.db.WithContext(ctx).
		Model(&model.Social{}).
		Where("follower_id = ?", FollowerID).
		Find(&relations).Error; err != nil {
		return nil, err
	}

	vloggerIDs := make([]uint, 0, len(relations))
	for _, rel := range relations {
		vloggerIDs = append(vloggerIDs, rel.VloggerID)
	}
	if len(vloggerIDs) == 0 {
		return []*model.Account{}, nil
	}

	var vloggers []*model.Account
	if err := r.db.WithContext(ctx).
		Model(&model.Account{}).
		Where("id IN ?", vloggerIDs).
		Find(&vloggers).Error; err != nil {
		return nil, err
	}
	return vloggers, nil
}

func (r *SocialRepository) IsFollowed(ctx context.Context, social *model.Social) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Social{}).
		Where("follower_id = ? AND vlogger_id = ?", social.FollowerID, social.VloggerID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}