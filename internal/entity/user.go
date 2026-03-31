package entity

import (
	"time"
)

type User struct {
	ID               string        `db:"id" json:"id"`
	Name             string        `db:"name" json:"name"`
	CustomerID       string        `db:"customer_id" json:"customer_id"`
	AuthMethod       AuthMetod     `db:"auth_method" json:"auth_method"`
	AuthID           string        `db:"auth_id" json:"auth_id"`
	FcmToken         *string       `db:"fcm_token" json:"-"`
	Credits          int64         `db:"credits" json:"credits"`
	CreditsExpiresAt *time.Time    `db:"credits_expires_at" json:"credits_expires_at"`
	Subscription     *Subscription `db:"-"                json:"subscription"`
	IsNewUser        bool          `db:"is_new_user" json:"is_new_user"`
	CreatedAt        time.Time     `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time     `db:"updated_at" json:"updated_at"`
	DeletedAt        *time.Time    `db:"deleted_at" json:"deleted_at"`
}

type Subscription struct {
	Plan      SubscriptionPlan   `db:"subscription_plan"       json:"subscription_plan"`
	Period    SubscriptionPeriod `db:"subscription_period"     json:"subscription_period"`
	Type      SubscriptionType   `db:"subscription_type"       json:"subscription_type"`
	Status    SubscriptionStatus `db:"subscription_status"     json:"subscription_status"`
	ExpiresAt *time.Time         `db:"subscription_expires_at" json:"subscription_expires_at"`
}
type SubscriptionType string

const (
	subscriptionTypeTrial   SubscriptionType = "trial"
	subscriptionTypeIntro   SubscriptionType = "intro"
	subscriptionTypeNormal  SubscriptionType = "normal"
	subscriptionTypePrepaid SubscriptionType = "prepaid"
	subscriptionTypePromo   SubscriptionType = "promo"
)

type SubscriptionStatus string

const (
	SubscriptionStatusActive       SubscriptionStatus = "active"
	SubscriptionStatusExpired      SubscriptionStatus = "expired"
	SubscriptionStatusBillingIssue SubscriptionStatus = "billingissue"
)

type SubscriptionPlan string

const (
	subscriptionPlanPro SubscriptionPlan = "pro"
)

type SubscriptionPeriod string

const (
	subscriptionPlanPeriod1w SubscriptionPeriod = "1w"
	subscriptionPlanPeriod1m SubscriptionPeriod = "1m"
	subscriptionPlanPerio6m  SubscriptionPeriod = "6m"
	subscriptionPlanPeriod1y SubscriptionPeriod = "1y"
)

func (s SubscriptionPeriod) getDays() int {
	switch s {
	case subscriptionPlanPeriod1w:
		return 7
	case subscriptionPlanPeriod1m:
		return 30
	case subscriptionPlanPerio6m:
		return 30 * 6
	case subscriptionPlanPeriod1y:
		return 30 * 12
	default:
		return 0
	}
}

func (u User) GetID() string {
	return u.ID
}

func (u User) GetName() string {
	return u.Name
}
