package worker

import (
	"context"
	"encoding/json"
	"enterprise/internal/repository"
	"enterprise/internal/model"
	"enterprise/pkg/rabbitmq"
	"errors"
	"log"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
)

type CommentWorker struct {
	ch  *amqp.Channel
	commentRepo *repository.CommentRepository
	videoRepo *repository.VideoRepository
	queue string
}

func NewCommentWorker(ch *amqp.Channel, comments *repository.CommentRepository, videos *repository.VideoRepository, queue string) *CommentWorker {
	return &CommentWorker{ch: ch, commentRepo: comments, videoRepo: videos, queue: queue}
}

func (w *CommentWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.commentRepo == nil || w.videoRepo == nil {
		return errors.New("comment worker is not initialized")
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

func (w *CommentWorker) handleDelivery(ctx context.Context, d amqp.Delivery) {
	if err := w.process(ctx, d.Body); err != nil {
		log.Printf("comment worker: failed to process message: %v", err)
		_ = d.Nack(false, true)
		return
	}
	_ = d.Ack(false)
}

func (w *CommentWorker) process(ctx context.Context, body []byte) error {
	var evt rabbitmq.CommentEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return nil
	}
	switch evt.Action {
	case "publish":
		return w.applyPublish(ctx, &evt)
	case "delete":
		return w.applyDelete(ctx, &evt)
	default:
		return nil
	}
}

func (w *CommentWorker) applyPublish(ctx context.Context, evt *rabbitmq.CommentEvent) error {
	if evt == nil || evt.VideoID == 0 || evt.AuthorID == 0 || strings.TrimSpace(evt.Content) == "" {
		return nil
	}

	ok, err := w.videoRepo.IsExist(ctx, evt.VideoID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	c := &model.Comment{
		Username: strings.TrimSpace(evt.Username),
		VideoID:  evt.VideoID,
		AuthorID: evt.AuthorID,
		Content:  strings.TrimSpace(evt.Content),
	}
	if err := w.commentRepo.CreateComment(ctx, c); err != nil {
		return err
	}
	return w.videoRepo.ChangePopularity(ctx, evt.VideoID, 1)
}

func (w *CommentWorker) applyDelete(ctx context.Context, evt *rabbitmq.CommentEvent) error {
	if evt == nil || evt.CommentID == 0 {
		return nil
	}
	c, err := w.commentRepo.GetByID(ctx, evt.CommentID)
	if err != nil {
		return err
	}
	if c == nil {
		return nil
	}
	return w.commentRepo.DeleteComment(ctx, c)
}
