package model

import "time"

// Comment 评论实体，存储视频评论信息
type Comment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"index" json:"username"`
	VideoID   uint      `gorm:"index" json:"video_id"`
	AuthorID  uint      `gorm:"index" json:"author_id"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// PublishCommentRequest 发布评论请求
type PublishCommentRequest struct {
	VideoID uint   `json:"video_id"`
	Content string `json:"content"`
}

// DeleteCommentRequest 删除评论请求
type DeleteCommentRequest struct {
	CommentID uint `json:"comment_id"`
}

// GetAllCommentsRequest 获取视频所有评论请求
type GetAllCommentsRequest struct {
	VideoID uint `json:"video_id"`
}
