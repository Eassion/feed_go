package service

import (
	"enterprise/internal/repository"
	"enterprise/pkg/cache"
	"time"
)

type VideoService struct {
	videoRepo *repository.VideoRepository
	cacheClient *cache.Client
	cacheTTL     time.Duration
	
}