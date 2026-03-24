package auth

import (
	"context"
	"fmt"
	"time"

	dbx "github.com/go-ozzo/ozzo-dbx"
	"github.com/google/uuid"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/entity"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/dbcontext"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/log"
)

type AuthRepository interface {
	GetUserByDeviceKey(ctx context.Context, deviceKey string) (*entity.User, error)
	CreateAnnonymusUser(ctx context.Context, deviceKey string) (*entity.User, error)
	CreateNewRefreshToken(ctx context.Context, deviceKey, userID, hashedValue string) error
}

type repository struct {
	db    *dbcontext.DB
	loger log.Logger
}

func NewsRepoAuth(db *dbcontext.DB, logger log.Logger) AuthRepository {
	return &repository{
		db:    db,
		loger: logger,
	}
}

func (r *repository) GetUserByDeviceKey(ctx context.Context, deviceKey string) (*entity.User, error) {

	var user entity.User

	err := r.db.DB().WithContext(ctx).Select("id", "name").From("public.users").Where(dbx.HashExp{
		"auth_method": entity.AuthMethodAnonymous,
		"auth_id":     deviceKey,
	}).One(&user)

	return &user, err
}

func (r *repository) CreateAnnonymusUser(ctx context.Context, deviceKey string) (*entity.User, error) {
	currentTime := time.Now()
	userID := entity.GenerateID()     // ← önceden üret
	customerID := uuid.New().String() // ← önceden üret
	userName := fmt.Sprintf("user%d", currentTime.Unix())

	result, err := r.db.DB().WithContext(ctx).NewQuery(
		`
		INSERT INTO public.users(
				id,
				name,
				customer_id,
				auth_method,
				auth_id,
				credits,
				is_new_user,
				created_at, 
				updated_at
   )VALUES (
			 {:id},
			 {:name},
			 {:customer_id},
			 {:auth_method},
			 {:auth_id},
			 {:credits},
			 {:is_new_user},
			 {:created_at},
			 {:updated_at}
		 );
	`).Bind(dbx.Params{
		"id":          userID,
		"name":        userName,
		"customer_id": customerID,
		"auth_method": entity.AuthMethodAnonymous,
		"auth_id":     deviceKey,
		"is_new_user": true,
		"credits":     3,
		"created_at":  currentTime,
		"updated_at":  currentTime,
	}).Prepare().Execute()

	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()

	if err != nil {
		return nil, err
	} else if rowsAffected == 0 {
		return nil, fmt.Errorf("kullanıcı oluşturulamadı") // ✅
	}

	return &entity.User{
		ID:         userID,
		Name:       userName,
		CustomerID: customerID,
		AuthMethod: entity.AuthMethodAnonymous,
		AuthID:     deviceKey,
		Credits:    3,
		IsNewUser:  true,
		CreatedAt:  currentTime,
		UpdatedAt:  currentTime,
	}, nil
}

func (r *repository) CreateNewRefreshToken(ctx context.Context, deviceKey, userID, hashedValue string) error {

	tx, err := r.db.DB().WithContext(ctx).Begin()

	if err != nil {
		return err
	}

	_, err1 := tx.Update("refresh_token",
		dbx.Params{"revoked_at": time.Now()},
		dbx.HashExp{"device_key": deviceKey, "user_id": userID},
	).Execute()

	token := entity.RefreshToken{
		ID:          entity.GenerateID(),
		DeviceKey:   deviceKey,
		UserID:      userID,
		HashedValue: hashedValue,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(time.Hour * 24 * 7),
	}

	_, err2 := tx.Insert("refresh_token", dbx.Params{
		"id":           token.ID,
		"device_key":   token.DeviceKey,
		"user_id":      token.UserID,
		"hashed_value": token.HashedValue,
		"created_at":   token.CreatedAt,
		"expires_at":   token.ExpiresAt,
	}).Execute()

	if err1 != nil || err2 != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			r.loger.Errorf("rollback hatası: %v", rbErr)
		}
		if err1 != nil {
			return err1
		}
		return err2
	}

	return tx.Commit()

}
