package model

// Social 用户关注关系表，记录粉丝与博主之间的关注关系
type Social struct {
	ID         uint `gorm:"primaryKey"`
	FollowerID uint `gorm:"not null;index:idx_social_follower;uniqueIndex:idx_social_follower_vlogger"`
	VloggerID  uint `gorm:"not null;index:idx_social_vlogger;uniqueIndex:idx_social_follower_vlogger"`
}

// FollowRequest 关注请求
type FollowRequest struct {
	VloggerID uint `json:"vlogger_id"`
}

// UnfollowRequest 取消关注请求
type UnfollowRequest struct {
	VloggerID uint `json:"vlogger_id"`
}

// GetAllFollowersRequest 获取博主所有粉丝请求
type GetAllFollowersRequest struct {
	VloggerID uint `json:"vlogger_id"`
}

// GetAllFollowersResponse 获取博主所有粉丝响应
type GetAllFollowersResponse struct {
	Followers []*Account `json:"followers"`
}

// GetAllVloggersRequest 获取用户关注的所有博主请求
type GetAllVloggersRequest struct {
	FollowerID uint `json:"follower_id"`
}

// GetAllVloggersResponse 获取用户关注的所有博主响应
type GetAllVloggersResponse struct {
	Vloggers []*Account `json:"vloggers"`
}
