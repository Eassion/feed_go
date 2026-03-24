package repository

import (
	"context"
	"enterprise/internal/model"

	"gorm.io/gorm"
)



type AccountRepository struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

func (r *AccountRepository) CreateAccount(ctx context.Context, account *model.Account) error {
	if err := r.db.WithContext(ctx).Create(account).Error; err!= nil {
		return err
	}
	return nil
}

func (r *AccountRepository) Rename(ctx context.Context, id uint, newUsername string) error {
	result := r.db.WithContext(ctx).Model(&model.Account{}).Where("id = ?", id).Update("username", newUsername)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected ==0{
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *AccountRepository) RenameWithToken(ctx context.Context, id uint, newUsername string, token string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Account{}).Where("id = ?", id).Update("username", newUsername)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(&model.Account{}).Where("id = ?", id).Update("token", token).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *AccountRepository) ChangePassword(ctx context.Context, id uint, newPassword string) error {
	result := r.db.WithContext(ctx).Model(&model.Account{}).Where("id = ?", id).Update("password", newPassword)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected ==0{
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *AccountRepository) FindByID(ctx context.Context, id uint) (*model.Account, error) {
	var account model.Account
	if err := r.db.WithContext(ctx).First(&account, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *AccountRepository) FindByUsername(ctx context.Context, username string) (*model.Account, error) {
	var account model.Account
	if err := r.db.WithContext(ctx).First(&account, "username = ?", username).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *AccountRepository) Login(ctx context.Context, id uint, token string) error {
	result := r.db.WithContext(ctx).Model(&model.Account{}).Where("id = ?", id).Update("token", token)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (r *AccountRepository) Logout(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Model(&model.Account{}).Where("id = ?", id).Update("token", "")
	if result.Error != nil {
		return result.Error
	}
	return nil
}
