package config

import (
	"os"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/log"
	"github.com/qiangxue/go-env"
	"gopkg.in/yaml.v2"
)

const (
	defaultServerPort       = 8080
	defaultJWTExpirationmin = 15
)

// Config represents an application configuration.
type Config struct {
	ServerPort    int    `yaml:"server_port" env:"SERVER_PORT"`
	DSN           string `yaml:"dsn" env:"DSN,secret"`
	JWTSigningKey string `yaml:"jwt_signing_key" env:"JWT_SIGNING_KEY,secret"`
	JWTExpiration int    `yaml:"jwt_expiration" env:"JWT_EXPIRATION"`

	Storage StorageConfig `yaml:"storage"`
}

type StorageConfig struct {
	Provider        string `yaml:"provider" env:"STORAGE_PROVIDER"` // "local" | "r2"
	Bucket          string `yaml:"bucket" env:"STORAGE_BUCKET"`
	AccountID       string `yaml:"account_id" env:"STORAGE_ACCOUNT_ID"`
	Prefix          string `yaml:"prefix" env:"STORAGE_PREFIX"`
	AccessKeyID     string `yaml:"-" env:"STORAGE_ACCESS_KEY_ID,secret"`
	SecretAccessKey string `yaml:"-" env:"STORAGE_SECRET_ACCESS_KEY,secret"`
}

// Validate validates the application configuration.
func (c Config) Validate() error {
	err := validation.ValidateStruct(&c,
		validation.Field(&c.DSN, validation.Required),
		validation.Field(&c.JWTSigningKey, validation.Required),
	)
	if err != nil {
		return err
	}

	if c.Storage.Provider == "r2" {
		return validation.ValidateStruct(&c.Storage,
			validation.Field(&c.Storage.Bucket, validation.Required),
			validation.Field(&c.Storage.AccountID, validation.Required),
			validation.Field(&c.Storage.AccessKeyID, validation.Required),
			validation.Field(&c.Storage.SecretAccessKey, validation.Required),
		)
	}

	return nil
}

// Load returns an application configuration populated from YAML and env.
func Load(file string, logger log.Logger) (*Config, error) {
	c := Config{
		ServerPort:    defaultServerPort,
		JWTExpiration: defaultJWTExpirationmin,
		Storage: StorageConfig{
			Provider: "local",
		},
	}

	bytes, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(bytes, &c); err != nil {
		return nil, err
	}

	if err = env.New("APP_", logger.Infof).Load(&c); err != nil {
		return nil, err
	}

	// go-env nested struct alanlarını beklediğimiz gibi doldurmadığı için
	// secret alanları manuel alıyoruz.
	if v := os.Getenv("APP_STORAGE_ACCESS_KEY_ID"); v != "" {
		c.Storage.AccessKeyID = v
	}
	if v := os.Getenv("APP_STORAGE_SECRET_ACCESS_KEY"); v != "" {
		c.Storage.SecretAccessKey = v
	}

	if err = c.Validate(); err != nil {
		return nil, err
	}

	return &c, nil
}
