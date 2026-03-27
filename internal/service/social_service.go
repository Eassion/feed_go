package service

import (
	"context"
	"enterprise/internal/model"
	"enterprise/internal/repository"
	"enterprise/pkg/rabbitmq"
	"errors"
)

type SocialService struct {
	socialRepo *repository.SocialRepository
	accountRepo *repository.AccountRepository
	socialMQ *rabbitmq.SocialMQ
}

func NewSocialService(repo *repository.SocialRepository, accountrepo *repository.AccountRepository, socialMQ *rabbitmq.SocialMQ) *SocialService {
	return &SocialService{socialRepo: repo, accountRepo: accountrepo, socialMQ: socialMQ}
}

func (s *SocialService) Follow(ctx context.Context, social *model.Social) error {
	_, err := s.accountRepo.FindByID(ctx, social.FollowerID)
	if err != nil {
		return err
	}
	_, err = s.accountRepo.FindByID(ctx, social.VloggerID)
	if err != nil {
		return err
	}
	if social.FollowerID == social.VloggerID {
		return errors.New("can not follow self")
	}
	isFollowed, err := s.socialRepo.IsFollowed(ctx, social)
	if err != nil {
		return err
	}
	if isFollowed {
		return errors.New("already followed")
	}
	if s.socialMQ != nil {
		s.socialMQ.Follow(ctx, social.FollowerID, social.VloggerID)
	}
	return s.socialRepo.Follow(ctx, social)
}

func (s *SocialService) Unfollow(ctx context.Context, social *model.Social) error {
	_, err := s.accountRepo.FindByID(ctx, social.FollowerID)
	if err != nil {
		return err
	}
	_, err = s.accountRepo.FindByID(ctx, social.VloggerID)
	if err != nil {
		return err
	}
	isFollowed, err := s.socialRepo.IsFollowed(ctx, social)
	if err != nil {
		return err
	}
	if !isFollowed {
		return errors.New("not followed")
	}
	if s.socialMQ != nil {
		s.socialMQ.UnFollow(ctx, social.FollowerID, social.VloggerID)
	}
	return s.socialRepo.Unfollow(ctx, social)
}

func (s *SocialService) GetAllFollowers(ctx context.Context, VloggerID uint) ([]*model.Account, error) {
	_, err := s.accountRepo.FindByID(ctx, VloggerID)
	if err != nil {
		return nil, err
	}
	return s.socialRepo.GetAllFollowers(ctx, VloggerID)
}

func (s *SocialService) GetAllVloggers(ctx context.Context, FollowerID uint) ([]*model.Account, error) {
	_, err := s.accountRepo.FindByID(ctx, FollowerID)
	if err != nil {
		return nil, err
	}
	return s.socialRepo.GetAllVloggers(ctx, FollowerID)
}

func (s *SocialService) IsFollowed(ctx context.Context, social *model.Social) (bool, error) {
	_, err := s.accountRepo.FindByID(ctx, social.FollowerID)
	if err != nil {
		return false, err
	}
	_, err = s.accountRepo.FindByID(ctx, social.VloggerID)
	if err != nil {
		return false, err
	}
	return s.socialRepo.IsFollowed(ctx, social)
}