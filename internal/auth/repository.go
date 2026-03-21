package auth

import (
	"context"

	dbx "github.com/go-ozzo/ozzo-dbx"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/entity"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/dbcontext"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/log"
)

type AuthRepository interface {
	GetUserByDeviceKey(ctx context.Context, deviceKey string) (entity.User, error)
}

type repository struct {
	db    *dbcontext.DB
	loger log.Logger
}

func NewsRepo(db *dbcontext.DB, logger log.Logger) AuthRepository {
	return &repository{
		db:    db,
		loger: logger,
	}
}

func (r *repository) GetUserByDeviceKey(ctx context.Context, deviceKey string) (entity.User, error) {

	var user entity.User

	err := r.db.DB().Select("id", "name").From("public.users").Where(dbx.HashExp{
		"auth_method": entity.AuthMethodAnonymous,
		"auth_id":     deviceKey,
	}).One(&user)

	return user, err
}
