package entity

import (
	"time"
)

type User struct {
	ID                    string     `db:"id"`
	Name                  string     `db:"name"`
	CustomerID            string     `db:"customer_id"`
	AuthMethod            AuthMetod  `db:"auth_method"`
	AuthID                string     `db:"auth_id"`
	FcmToken              *string    `db:"fcm_token"`
	Credits               int64      `db:"credits"`
	CreditsExpiresAt      *time.Time `db:"credits_expires_at"`
	SubscriptionPlan      *string    `db:"subscription_plan"`
	SubscriptionPeriod    *string    `db:"subscription_period"`
	SubscriptionType      *string    `db:"subscription_type"`
	SubscriptionStatus    *string    `db:"subscription_status"`
	SubscriptionExpiresAt *time.Time `db:"subscription_expires_at"`
	IsNewUser             bool       `db:"is_new_user"`
	CreatedAt             time.Time  `db:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at"`
	DeletedAt             *time.Time `db:"deleted_at"`
}

func (u User) GetID() string {
	return u.ID
}

func (u User) GetName() string {
	return u.Name
}
