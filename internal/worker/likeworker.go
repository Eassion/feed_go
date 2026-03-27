package worker

import (
	"context"
	"encoding/json"
	"enterprise/internal/model"
	"enterprise/internal/repository"
	"enterprise/pkg/rabbitmq"
	"errors"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type LikeWorker struct {
	ch       *amqp.Channel
	likeRepo *repository.LikeRepository
	videoRepo  *repository.VideoRepository
	queue    string
}

func NewLikeWorker(ch *amqp.Channel, likes *repository.LikeRepository, videos *repository.VideoRepository, queue string) *LikeWorker {
	return &LikeWorker{ch: ch, likeRepo: likes, videoRepo: videos, queue: queue}
}

func (w *LikeWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.likeRepo == nil || w.videoRepo == nil {
		return errors.New("like worker is not initialized")
	}
	if w.queue == "" {
		return errors.New("queue is required")
	}

	deliveries, err := w.ch.Consume(
		w.queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("deliveries channel closed")
			}
			w.handleDelivery(ctx, d)
		}
	}
}

func (w *LikeWorker) handleDelivery(ctx context.Context, d amqp.Delivery) {
	if err := w.process(ctx, d.Body); err != nil {
		log.Printf("like worker: failed to process message: %v", err)
		_ = d.Nack(false, true)
		return
	}
	_ = d.Ack(false)
}

func (w *LikeWorker) process(ctx context.Context, body []byte) error {
	var evt rabbitmq.LikeEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		// 解析事件失败，直接丢弃
		return nil
	}
	if evt.UserID == 0 || evt.VideoID == 0 {
		return nil
	}

	switch evt.Action {
	case "like":
		return w.applyLike(ctx, evt.UserID, evt.VideoID)
	case "unlike":
		return w.applyUnlike(ctx, evt.UserID, evt.VideoID)
	default:
		return nil
	}
}

func (w *LikeWorker) applyLike(ctx context.Context, userID, videoID uint) error {
	ok, err := w.videoRepo.IsExist(ctx, videoID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	created, err := w.likeRepo.LikeIgnoreDuplicate(ctx, &model.Like{
		VideoID:   videoID,
		AccountID: userID,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return err
	}
	if !created {
		return nil
	}

	if err := w.videoRepo.ChangeLikesCount(ctx, videoID, 1); err != nil {
		return err
	}
	return w.videoRepo.ChangePopularity(ctx, videoID, 1)
}

func (w *LikeWorker) applyUnlike(ctx context.Context, userID, videoID uint) error {
	ok, err := w.videoRepo.IsExist(ctx, videoID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	deleted, err := w.likeRepo.DeleteByVideoAndAccount(ctx, videoID, userID)
	if err != nil {
		return err
	}
	if !deleted {
		return nil
	}

	if err := w.videoRepo.ChangeLikesCount(ctx, videoID, -1); err != nil {
		return err
	}
	return w.videoRepo.ChangePopularity(ctx, videoID, -1)
}