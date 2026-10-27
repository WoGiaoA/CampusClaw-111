package campus

import (
	"database/sql"
	"errors"
	"github.com/go-sql-driver/mysql"
	"os"
	"strings"
	"time"
)

type Config struct {
	DemoMode      bool
	SessionKey    string
	UploadDir     string
	SecureCookie  bool
	SeedPasswords map[string]string
}

func ConfigFromEnv() (Config, error) {
	c := Config{SessionKey: os.Getenv("SESSION_KEY"), UploadDir: os.Getenv("UPLOAD_DIR"), SecureCookie: os.Getenv("SESSION_COOKIE_SECURE") == "true", SeedPasswords: map[string]string{}}
	if os.Getenv("DEMO_MODE") == "true" {
		if os.Getenv("APP_ENV") != "local" {
			return c, errors.New("DEMO_MODE requires APP_ENV=local")
		}
		c.DemoMode = true
	}
	if len(c.SessionKey) < 32 {
		return c, errors.New("SESSION_KEY requires at least 32 bytes")
	}
	if c.UploadDir == "" {
		c.UploadDir = "uploads"
	}
	for _, name := range []string{"teacher_a", "teacher_b", "student_a1", "student_b1"} {
		c.SeedPasswords[name] = os.Getenv("SEED_" + strings.ToUpper(name) + "_PASSWORD")
	}
	return c, nil
}

func OpenDB() (*sql.DB, error) {
	if os.Getenv("MYSQL_PASSWORD") == "" {
		return nil, errors.New("MYSQL_PASSWORD is required")
	}
	cfg := mysql.NewConfig()
	cfg.User = os.Getenv("MYSQL_USER")
	cfg.Passwd = os.Getenv("MYSQL_PASSWORD")
	cfg.DBName = os.Getenv("MYSQL_DATABASE")
	if cfg.User == "" {
		cfg.User = "campusclaw"
	}
	if cfg.DBName == "" {
		cfg.DBName = "campusclaw"
	}
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("MYSQL_ADDR")
	if cfg.Addr == "" {
		cfg.Addr = "db:3306"
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 10 * time.Second
	cfg.WriteTimeout = 10 * time.Second
	cfg.Params = map[string]string{"charset": "utf8mb4", "time_zone": "'+00:00'"}
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}
