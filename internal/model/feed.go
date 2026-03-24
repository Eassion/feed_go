package model

import "time"

// FeedAuthor 视频作者信息
type FeedAuthor struct {
	ID       uint   `json:"id"`       // 作者ID
	Username string `json:"username"` // 作者用户名
}

// FeedVideoItem 视频条目，Feed流中的单个视频信息
type FeedVideoItem struct {
	ID          uint       `json:"id"`                    // 视频ID
	Author      FeedAuthor `json:"author"`                // 视频作者
	Title       string     `json:"title"`                 // 视频标题
	Description string     `json:"description,omitempty"` // 视频描述，可选字段
	PlayURL     string     `json:"play_url"`              // 视频播放地址
	CoverURL    string     `json:"cover_url"`             // 视频封面地址
	CreateTime  int64      `json:"create_time"`           // 创建时间戳
	LikesCount  int64      `json:"likes_count"`           // 点赞数
	IsLiked     bool       `json:"is_liked"`              // 当前用户是否已点赞
}

// ListLatestRequest 获取最新视频列表请求
type ListLatestRequest struct {
	Limit      int   `json:"limit"`       // 每页数量限制
	LatestTime int64 `json:"latest_time"` // 最新时间戳，用于分页拉取更早的视频
}

// ListLatestResponse 获取最新视频列表响应
type ListLatestResponse struct {
	VideoList []FeedVideoItem `json:"video_list"` // 视频列表
	NextTime  int64           `json:"next_time"`  // 下一页的时间戳游标
	HasMore   bool            `json:"has_more"`   // 是否还有更多数据
}

// ListLikesCountRequest 按点赞数排序获取视频列表请求
type ListLikesCountRequest struct {
	Limit            int    `json:"limit"`                        // 每页数量限制
	LikesCountBefore *int64 `json:"likes_count_before,omitempty"` // 上一页最后一条的点赞数，用于游标分页
	IDBefore         *uint  `json:"id_before,omitempty"`          // 上一页最后一条的ID，用于同点赞数时的二级排序
}

// LikesCountCursor 点赞数游标，用于分页定位
type LikesCountCursor struct {
	LikesCount int64 // 点赞数值
	ID         uint  // 视频ID，用于同点赞数时的二级排序
}

// ListLikesCountResponse 按点赞数排序获取视频列表响应
type ListLikesCountResponse struct {
	VideoList            []FeedVideoItem `json:"video_list"`                        // 视频列表
	NextLikesCountBefore *int64          `json:"next_likes_count_before,omitempty"` // 下一页的点赞数游标
	NextIDBefore         *uint           `json:"next_id_before,omitempty"`          // 下一页的ID游标
	HasMore              bool            `json:"has_more"`                          // 是否还有更多数据
}

// ListByFollowingRequest 获取关注用户的视频列表请求
type ListByFollowingRequest struct {
	Limit      int   `json:"limit"`       // 每页数量限制
	LatestTime int64 `json:"latest_time"` // 最新时间戳，用于分页
}

// ListByFollowingResponse 获取关注用户的视频列表响应
type ListByFollowingResponse struct {
	VideoList []FeedVideoItem `json:"video_list"` // 视频列表
	NextTime  int64           `json:"next_time"`  // 下一页的时间戳游标
	HasMore   bool            `json:"has_more"`   // 是否还有更多数据
}

// ListByPopularityRequest 按热度排序获取视频列表请求
type ListByPopularityRequest struct {
	Limit          int   `json:"limit"`                      // 每页数量限制
	AsOf           int64 `json:"as_of"`                      // 服务器返回的分钟时间戳；第一页传0
	Offset         int   `json:"offset"`                     // 下一页从这里开始；第一页传0
	LatestIDBefore *uint `json:"latest_id_before,omitempty"` // 最新视频ID，用于去重

	// DB fallback 用（可选）
	LatestPopularity int64     `json:"latest_popularity"` // 热度值
	LatestBefore     time.Time `json:"latest_before"`     // 时间点
}

// ListByPopularityResponse 按热度排序获取视频列表响应
type ListByPopularityResponse struct {
	VideoList  []FeedVideoItem `json:"video_list"`  // 视频列表
	AsOf       int64           `json:"as_of"`       // 当前查询的时间基准点
	NextOffset int             `json:"next_offset"` // 下一页的偏移量
	HasMore    bool            `json:"has_more"`    // 是否还有更多数据

	NextLatestPopularity *int64     `json:"next_latest_popularity,omitempty"` // 下一页的热度游标
	NextLatestBefore     *time.Time `json:"next_latest_before,omitempty"`     // 下一页的时间游标
	NextLatestIDBefore   *uint      `json:"next_latest_id_before,omitempty"`  // 下一页的ID游标
}
