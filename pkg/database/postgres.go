package database

import (
	"fmt"
	"log"
	"time"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	LogLevel string
}

func InitDB(cfg Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	var level logger.LogLevel
	switch cfg.LogLevel {
	case "info":
		level = logger.Info
	case "silent":
		level = logger.Silent
	default:
		level = logger.Warn
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(level),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(1 * time.Hour)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("database connected successfully")
	return db, nil
}

func RunMigrations(db *gorm.DB) error {
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`).Error; err != nil {
		return fmt.Errorf("failed to create uuid-ossp extension: %w", err)
	}

	if err := db.AutoMigrate(&domain.User{}, &domain.Wallet{}, &domain.WalletBalance{}, &domain.Transaction{}, &domain.CryptoAddress{}, &domain.Notification{}); err != nil {
		return fmt.Errorf("failed to auto-migrate: %w", err)
	}

	if err := db.Exec(`
		ALTER TABLE wallets
		DROP CONSTRAINT IF EXISTS fk_wallets_user_id,
		ADD CONSTRAINT fk_wallets_user_id
		FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
	`).Error; err != nil {
		return fmt.Errorf("failed to add wallets user_id FK: %w", err)
	}

	if err := db.Exec(`
		ALTER TABLE wallet_balances
		DROP CONSTRAINT IF EXISTS fk_wallet_balances_wallet_id,
		ADD CONSTRAINT fk_wallet_balances_wallet_id
		FOREIGN KEY (wallet_id) REFERENCES wallets(wallet_id) ON DELETE CASCADE
	`).Error; err != nil {
		return fmt.Errorf("failed to add wallet_balances wallet_id FK: %w", err)
	}

	if err := db.Exec(`
		ALTER TABLE crypto_addresses
		DROP CONSTRAINT IF EXISTS fk_crypto_addresses_wallet_id,
		ADD CONSTRAINT fk_crypto_addresses_wallet_id
		FOREIGN KEY (wallet_id) REFERENCES wallets(wallet_id) ON DELETE CASCADE
	`).Error; err != nil {
		return fmt.Errorf("failed to add crypto_addresses wallet_id FK: %w", err)
	}

	if err := db.Exec(`
		ALTER TABLE transactions
		DROP CONSTRAINT IF EXISTS fk_transactions_source_wallet_id,
		ADD CONSTRAINT fk_transactions_source_wallet_id
		FOREIGN KEY (source_wallet_id) REFERENCES wallets(wallet_id) ON DELETE SET NULL
	`).Error; err != nil {
		return fmt.Errorf("failed to add transactions source_wallet_id FK: %w", err)
	}

	if err := db.Exec(`
		ALTER TABLE transactions
		DROP CONSTRAINT IF EXISTS fk_transactions_destination_wallet_id,
		ADD CONSTRAINT fk_transactions_destination_wallet_id
		FOREIGN KEY (destination_wallet_id) REFERENCES wallets(wallet_id) ON DELETE SET NULL
	`).Error; err != nil {
		return fmt.Errorf("failed to add transactions destination_wallet_id FK: %w", err)
	}

	_ = db.Exec(`UPDATE users SET pin_enabled = false WHERE transaction_pin IS NULL OR transaction_pin = '';`).Error

	log.Println("database migrations completed")
	return nil
}
