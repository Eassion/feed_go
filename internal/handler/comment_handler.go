package handler

import (
	"enterprise/internal/model"
	"enterprise/internal/service"

	"github.com/gin-gonic/gin"
)

type CommentHandler struct {
	service        *service.CommentService
	accountService *service.AccountService
}

func NewCommentHandler(commentService *service.CommentService, accountService *service.AccountService) *CommentHandler {
	return &CommentHandler{service: commentService, accountService: accountService}
}
func (h *CommentHandler) PublishComment(c *gin.Context) {
	var req model.PublishCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Content == "" {
		c.JSON(400, gin.H{"error": "content is required"})
		return
	}
	if req.VideoID <= 0 {
		c.JSON(400, gin.H{"error": "video_id is required"})
		return
	}
	authorId, err := getAccountID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	user, err := h.accountService.FindByID(c.Request.Context(), authorId)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	comment := &model.Comment{
		Username: user.Username,
		VideoID:  req.VideoID,
		AuthorID: authorId,
		Content:  req.Content,
	}
	if err := h.service.Publish(c.Request.Context(), comment); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "comment published successfully"})
}

func (h *CommentHandler) DeleteComment(c *gin.Context) {
	var req model.DeleteCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	accountID, err := getAccountID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.CommentID <= 0 {
		c.JSON(400, gin.H{"error": "comment_id is required"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), req.CommentID, accountID); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "comment deleted successfully"})
}

func (h *CommentHandler) GetAllComments(c *gin.Context) {
	var req model.GetAllCommentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.VideoID == 0 {
		c.JSON(400, gin.H{"error": "video_id is required"})
		return
	}
	comments, err := h.service.GetAll(c.Request.Context(), req.VideoID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, comments)
}
