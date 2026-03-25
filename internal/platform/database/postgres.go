package database

import (
	"fmt"

	"skilljudge/backend/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewPostgres(cfg config.DBConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("postgres connection failed (%s:%s/%s): %w", cfg.Host, cfg.Port, cfg.Name, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("postgres db handle failed (%s:%s/%s): %w", cfg.Host, cfg.Port, cfg.Name, err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("postgres ping failed (%s:%s/%s): %w", cfg.Host, cfg.Port, cfg.Name, err)
	}

	return db, nil
}
