package model

import "time"

// Video 视频实体，存储视频的基本信息
type Video struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AuthorID    uint      `gorm:"index;not null" json:"author_id"`
	Username    string    `gorm:"type:varchar(255);not null" json:"username"`
	Title       string    `gorm:"type:varchar(255);not null" json:"title"`
	Description string    `gorm:"type:varchar(255);" json:"description,omitempty"`
	PlayURL     string    `gorm:"type:varchar(255);not null" json:"play_url"`
	CoverURL    string    `gorm:"type:varchar(255);not null" json:"cover_url"`
	CreateTime  time.Time `gorm:"autoCreateTime" json:"create_time"`
	LikesCount  int64     `gorm:"column:likes_count;not null;default:0" json:"likes_count"`
	Popularity  int64     `gorm:"column:popularity;not null;default:0" json:"popularity"`
}

// PublishVideoRequest 发布视频请求
type PublishVideoRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	PlayURL     string `json:"play_url"`
	CoverURL    string `json:"cover_url"`
}

// DeleteVideoRequest 删除视频请求
type DeleteVideoRequest struct {
	ID uint `json:"id"`
}

// ListByAuthorIDRequest 按作者ID获取视频列表请求
type ListByAuthorIDRequest struct {
	AuthorID uint `json:"author_id"`
}

// GetDetailRequest 获取视频详情请求
type GetDetailRequest struct {
	ID uint `json:"id"`
}

// UpdateLikesCountRequest 更新视频点赞数请求
type UpdateLikesCountRequest struct {
	ID         uint  `json:"id"`
	LikesCount int64 `json:"likes_count"`
}

// OutboxMsg Outbox模式消息表，用于保证消息可靠投递
type OutboxMsg struct {
	ID         uint      `gorm:"primaryKey"`
	VideoID    uint      `gorm:"index"`
	EventType  string    `gorm:"type:varchar(50)"`
	CreateTime time.Time `gorm:"autoCreateTime"`
	Status     string    `gorm:"type:varchar(50);index"`
}
