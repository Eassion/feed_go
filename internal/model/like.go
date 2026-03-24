package model

import "time"

// Like 点赞实体，存储视频点赞信息
type Like struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	VideoID   uint      `gorm:"uniqueIndex:idx_like_video_account;not null" json:"video_id"`
	AccountID uint      `gorm:"uniqueIndex:idx_like_video_account;not null" json:"account_id"`
	CreatedAt time.Time `json:"created_at"`
}

// LikeRequest 点赞请求
// 用于创建点赞记录
type LikeRequest struct {
	VideoID uint `json:"video_id"`
}