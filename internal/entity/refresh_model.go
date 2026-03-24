package entity

import "time"

type RefreshToken struct {
	ID          string     `db:"id"`
	UserID      string     `db:"user_id"`
	HashedValue string     `db:"hashed_value"`
	DeviceKey   string     `db:"device_key"`
	CreatedAt   time.Time  `db:"created_at"`
	ExpiresAt   time.Time  `db:"expires_at"`
	RevokedAt   *time.Time `db:"revoked_at"`
}
