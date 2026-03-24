package handler

import (
	"enterprise/internal/model"
	"enterprise/internal/service"
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AccountHandler struct {
	accountService *service.AccountService
}

func NewAccountHandler(accountService *service.AccountService) *AccountHandler {
	return &AccountHandler{accountService: accountService}
}

func (h *AccountHandler) CreateAccount(c *gin.Context) {
	var req model.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.accountService.CreateAccount(c.Request.Context(), &model.Account{
		Username: req.Username,
		Password: req.Password,
	}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "account created"})
}

func (h *AccountHandler) Rename(c *gin.Context) {
	var req model.RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	accountID, err := getAccountID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	token, err := h.accountService.Rename(c.Request.Context(), accountID, req.NewUsername)
	if err != nil {
		if errors.Is(err, errors.New("用户名不能为空！")) {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, errors.New("用户名已被存在！")) {
			c.JSON(409, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"error": "account not found"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"token": token})
}

func getAccountID(c *gin.Context) (uint, error) {
	value, exists := c.Get("accountID")
	if !exists {
		return 0, errors.New("accountID not found")
	}
	id, ok := value.(uint)
	if !ok {
		return 0, errors.New("accountID has invalid type")
	}
	return id, nil
}

func (h *AccountHandler) ChangePassword(c *gin.Context) {
	var req model.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.accountService.ChangePassword(c.Request.Context(), req.Username, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(400, gin.H{"error": "unsuccessfully password changed"})
		return
	}
	c.JSON(200, gin.H{"message": "successfully password changed"})
}

func (h *AccountHandler) FindByID(c *gin.Context) {
	var req model.FindByIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if account, err := h.accountService.FindByID(c.Request.Context(), req.ID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	} else {
		c.JSON(200, account)
	}
}

func (h *AccountHandler) FindByUsername(c *gin.Context) {
	var req model.FindByUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if account, err := h.accountService.FindByUsername(c.Request.Context(), req.Username); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	} else {
		c.JSON(200, account)
	}
}

func (h *AccountHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if token, err := h.accountService.Login(c.Request.Context(), req.Username, req.Password); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	} else {
		c.JSON(200, gin.H{"token": token})
	}
}

func (h *AccountHandler) Logout(c *gin.Context) {
	accountID, err := getAccountID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.accountService.Logout(c.Request.Context(), accountID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "account logged out"})
}