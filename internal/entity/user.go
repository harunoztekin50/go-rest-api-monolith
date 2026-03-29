package entity

import (
	"time"
)

type User struct {
	ID                    string     `db:"id" json:"id"`
	Name                  string     `db:"name" json:"name"`
	CustomerID            string     `db:"customer_id" json:"customer_id"`
	AuthMethod            AuthMetod  `db:"auth_method" json:"auth_method"`
	AuthID                string     `db:"auth_id" json:"auth_id"`
	FcmToken              *string    `db:"fcm_token" json:"fcm_token"`
	Credits               int64      `db:"credits" json:"credits"`
	CreditsExpiresAt      *time.Time `db:"credits_expires_at" json:"credits_expires_at"`
	SubscriptionPlan      *string    `db:"subscription_plan" json:"subscription_plan"`
	SubscriptionPeriod    *string    `db:"subscription_period" json:"subscription_period"`
	SubscriptionType      *string    `db:"subscription_type" json:"subscription_type"`
	SubscriptionStatus    *string    `db:"subscription_status" json:"subscription_status"`
	SubscriptionExpiresAt *time.Time `db:"subscription_expires_at" json:"subscription_expires_at"`
	IsNewUser             bool       `db:"is_new_user" json:"is_new_user"`
	CreatedAt             time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt             *time.Time `db:"deleted_at" json:"deleted_at"`
}

func (u User) GetID() string {
	return u.ID
}

func (u User) GetName() string {
	return u.Name
}
