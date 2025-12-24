package session

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostgresStore implements Store using PostgreSQL
type PostgresStore struct {
	db *gorm.DB
}

// PostgresConfig holds PostgreSQL connection configuration
type PostgresConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DefaultPostgresConfig returns default PostgreSQL configuration
func DefaultPostgresConfig() PostgresConfig {
	return PostgresConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "postgres",
		Password:        "postgres",
		DBName:          "sandbox",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(config PostgresConfig) (*PostgresStore, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)

	// Auto migrate
	if err := db.AutoMigrate(&Session{}); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

// Create stores a new session
func (s *PostgresStore) Create(ctx context.Context, session *Session) error {
	return s.db.WithContext(ctx).Create(session).Error
}

// Get retrieves a session by ID
func (s *PostgresStore) Get(ctx context.Context, id string) (*Session, error) {
	var session Session
	if err := s.db.WithContext(ctx).First(&session, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, err
	}
	return &session, nil
}

// GetByUser retrieves sessions by user ID
func (s *PostgresStore) GetByUser(ctx context.Context, userID string) ([]*Session, error) {
	var sessions []*Session
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

// Update updates a session
func (s *PostgresStore) Update(ctx context.Context, session *Session) error {
	session.UpdatedAt = time.Now()
	return s.db.WithContext(ctx).Save(session).Error
}

// Delete deletes a session
func (s *PostgresStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&Session{}, "id = ?", id).Error
}

// ListExpired lists expired sessions
func (s *PostgresStore) ListExpired(ctx context.Context) ([]*Session, error) {
	var sessions []*Session
	if err := s.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

// DeleteExpired deletes expired sessions
func (s *PostgresStore) DeleteExpired(ctx context.Context) (int, error) {
	result := s.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&Session{})
	return int(result.RowsAffected), result.Error
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
