package rabbitmq

import (
	"context"
	"errors"
	"time"
)

type PopularityMQ struct {
	MQ *RabbitMQ
}

const (
	popularityExchange   = "video.popularity.events"
	popularityQueue      = "video.popularity.events"
	popularityBindingKey = "video.popularity.*"

	popularityUpdateRK = "video.popularity.update"
)

type PopularityEvent struct {
	EventID string `json:"event_id"`
	VideoID uint   `json:"video_id"`
	Change  int64 `json:"change"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewPopularityMQ(mq *RabbitMQ) (*PopularityMQ, error) {
	if mq == nil {
		return nil, errors.New("rabbitmq is required")
	}
	if err := mq.DeclareTopic(popularityExchange, popularityQueue, popularityBindingKey); err != nil {
		return nil, err
	}
	return &PopularityMQ{MQ: mq}, nil
}

func (m *PopularityMQ) Update(ctx context.Context, videoID uint, change int64) error {
	if m == nil || m.MQ == nil {
		return errors.New("popularity mq is not initialized")
	}
	if videoID == 0 || change == 0 {
		return errors.New("videoID and change are required")
	}
	id, err := newEventID(16)
	if err != nil {
		return err
	}
	event := PopularityEvent{
		EventID:    id,
		VideoID:    videoID,
		Change:     change,
		OccurredAt: time.Now().UTC(),
	}
	return m.MQ.PublishJSON(ctx, popularityExchange, popularityUpdateRK, event)
}