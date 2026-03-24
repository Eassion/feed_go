package service

import (
	"context"
	auth "enterprise/internal/middleware"
	"enterprise/internal/model"
	"enterprise/internal/repository"
	"enterprise/pkg/cache"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"

	"gorm.io/gorm"
)

type AccountService struct {
	accountRepo *repository.AccountRepository
	cacheClient *cache.Client
}

func NewAccountService(accountRepo *repository.AccountRepository, cacheClient *cache.Client) *AccountService {
	return &AccountService{accountRepo: accountRepo, cacheClient: cacheClient}
}

func (s *AccountService) CreateAccount(ctx context.Context, account *model.Account) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(account.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	account.Password = string(passwordHash)
	return s.accountRepo.CreateAccount(ctx, account)
}

func (s *AccountService) Rename(ctx context.Context, accountID uint, newUsername string) (string, error) {
	if newUsername == "" {
		return "", errors.New("用户名不能为空！")
	}

	token, err := auth.GenerateToken(accountID, newUsername)
	if err != nil {
		return "", err
	}

	if err := s.accountRepo.RenameWithToken(ctx, accountID, newUsername, token); err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return "", errors.New("用户名已被存在！")
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
		return "", err
	}
	if s.cacheClient != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		s.cacheClient.RDB.Set(cacheCtx, fmt.Sprintf("account:%d", accountID), token, 24*time.Hour)
	}
	return token, nil
}

func (s *AccountService) ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error {
	account, err := s.accountRepo.FindByUsername(ctx, username)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(oldPassword)); err != nil {
		return err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := s.accountRepo.ChangePassword(ctx, account.ID, string(passwordHash)); err != nil {
		return err
	}
	// TODO: 退出登录
	if err := s.accountRepo.Logout(ctx, account.ID); err != nil {
		log.Printf("failed to logout: %v", err)
	}
	return nil
}

func (s *AccountService) Logout(ctx context.Context, accountID uint) error {
	account, err := s.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return err
	}
	if account.Token == "" {
		return nil
	}
	if s.cacheClient != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		if err := s.cacheClient.RDB.Del(cacheCtx, fmt.Sprintf("account:%d", account.ID)).Err(); err != nil {
			log.Printf("failed to del cache: %v", err)
		}
	}
	return s.accountRepo.Logout(ctx, account.ID)
}

func (s *AccountService) FindByID(ctx context.Context, id uint) (*model.Account, error) {
	account, err := s.accountRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	} else {
		return account, nil
	}
}

func (s *AccountService) FindByUsername(ctx context.Context, username string) (*model.Account, error) {
	account, err := s.accountRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	} else {
		return account, nil
	}
}

func (s *AccountService) Login(ctx context.Context, username, password string) (string, error) {
	account, err := s.FindByUsername(ctx, username)
	if err != nil {
		return "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(password)); err != nil {
		return "", err
	}
	// generate token
	token, err := auth.GenerateToken(account.ID, account.Username)
	if err != nil {
		return "", err
	}
	if err := s.accountRepo.Login(ctx, account.ID, token); err != nil {
		return "", err
	}
	if s.cacheClient != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		if err := s.cacheClient.RDB.Set(cacheCtx, fmt.Sprintf("account:%d", account.ID), []byte(token), 24*time.Hour); err != nil {
			log.Printf("failed to set cache: %v", err)
		}
	}
	return token, nil
}
