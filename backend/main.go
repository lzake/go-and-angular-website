package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/lzake/gowebsite-backend/docs"

	"github.com/Masterminds/squirrel"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
	"github.com/patrickmn/go-cache"
	echoSwagger "github.com/swaggo/echo-swagger"
	"golang.org/x/crypto/bcrypt"
)

// @title Swagger for zach lowe go and angular website
// @version 1.0
// @description Swagger for zach lowe go and angular website.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /

var (
	statementBuilder = squirrel.StatementBuilder
	userCache        *cache.Cache
)

func init() {
	statementBuilder = statementBuilder.PlaceholderFormat(squirrel.Dollar)
	userCache = cache.New(5*time.Minute, 10*time.Minute) // Initializing cache
}

type Config struct {
	Database struct {
		Host     string `json:"host"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbname"`
		Port     int    `json:"port"`
		SSLMode  string `json:"sslmode"`
	} `json:"database"`
	App struct {
		TimeZone  string `json:"timezone"`
		LogLevel  string `json:"log_level"`
		RateLimit int    `json:"rate_limit"`
	} `json:"app"`
}

type User struct {
	ID                int        `json:"id"`
	Username          string     `json:"username"`
	Email             string     `json:"email"`
	Password          string     `json:"password,omitempty"`  // New field for password
	ProfilePictureURL string     `json:"profile_picture_url"` // New field for profile picture URL
	Bio               string     `json:"bio"`                 // New field for bio
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"` // New field for soft delete
}

func readConfig(filename string) (*Config, error) {
	err := godotenv.Load() // Load environment variables from .env file
	if err != nil {
		log.Println("Error loading .env file, using config.json")
		file, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration file: %w", err)
		}

		var config Config
		err = json.Unmarshal(file, &config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse configuration file: %w", err)
		}

		return &config, nil
	}

	config := &Config{
		Database: struct {
			Host     string `json:"host"`
			User     string `json:"user"`
			Password string `json:"password"`
			DBName   string `json:"dbname"`
			Port     int    `json:"port"`
			SSLMode  string `json:"sslmode"`
		}{
			Host:     os.Getenv("DB_HOST"),
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			DBName:   os.Getenv("DB_NAME"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			SSLMode:  os.Getenv("DB_SSLMODE"),
		},
		App: struct {
			TimeZone  string `json:"timezone"`
			LogLevel  string `json:"log_level"`
			RateLimit int    `json:"rate_limit"`
		}{
			TimeZone:  os.Getenv("APP_TIMEZONE"),
			LogLevel:  os.Getenv("APP_LOG_LEVEL"),
			RateLimit: getEnvAsInt("APP_RATE_LIMIT", 100),
		},
	}
	return config, nil
}

func getEnvAsInt(name string, defaultVal int) int {
	valueStr := os.Getenv(name)
	if valueStr == "" {
		return defaultVal
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultVal
	}
	return value
}

// FUTURE! // Optimize the getUsers function by adding pagination to handle large datasets efficiently.
func getUsers(db *sql.DB, page int, pageSize int) ([]User, error) {
	offset := (page - 1) * pageSize

	queryBuilder := squirrel.Select("id", "username", "email", "profile_picture_url", "bio", "created_at", "updated_at").
		From("users").
		Where(squirrel.Eq{"deleted_at": nil}). // Exclude soft deleted records
		Limit(uint64(pageSize)).
		Offset(uint64(offset))
	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.ProfilePictureURL, &u.Bio, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// FUTURE! // Add caching for getUserByID to improve performance for frequently accessed user data.
func getUserByID(db *sql.DB, id int) (User, error) {
	// Check cache first
	if cachedUser, found := userCache.Get(strconv.Itoa(id)); found {
		return cachedUser.(User), nil
	}

	var user User
	queryBuilder := squirrel.Select("id", "username", "email", "profile_picture_url", "bio", "created_at", "updated_at").From("users").Where(squirrel.Eq{"id": id, "deleted_at": nil}) // Exclude soft deleted records
	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		return user, err
	}

	err = db.QueryRow(sql, args...).Scan(&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return user, err
	}

	// Store in cache
	userCache.Set(strconv.Itoa(id), user, cache.DefaultExpiration)

	return user, nil
}

// FUTURE! // Enhance the createUser function with email verification and password hashing.
func createUser(db *sql.DB, user *User) error {
	var existingUser User
	err := db.QueryRow("SELECT id FROM users WHERE username = $1 OR email = $2", user.Username, user.Email).Scan(&existingUser.ID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if existingUser.ID != 0 {
		return errors.New("username_or_email_exists")
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hashedPassword)

	// Create a verification token (dummy implementation)
	verificationToken := "dummy_verification_token"

	queryBuilder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Insert("users").
		Columns("username", "email", "password", "profile_picture_url", "bio", "verification_token").
		Values(user.Username, user.Email, user.Password, user.ProfilePictureURL, user.Bio, verificationToken).
		Suffix("RETURNING id, created_at, updated_at")

	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		log.Printf("Error building SQL for createUser: %s, error: %v", sql, err)
		return err
	}

	err = db.QueryRow(sql, args...).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		log.Printf("Error executing createUser: %s, args: %v, error: %v", sql, args, err)
		return err
	}

	// Send verification email (dummy implementation)
	log.Printf("Sending verification email to %s with token %s", user.Email, verificationToken)

	// Audit log
	log.Printf("User created: %s", user.Username)

	return nil
}

// FUTURE! // Include audit logging for changes made to user data for better traceability.
func updateUser(db *sql.DB, id int, user *User) error {
	var existingUser User
	err := db.QueryRow("SELECT id FROM users WHERE (username = $1 OR email = $2) AND id != $3", user.Username, user.Email, id).Scan(&existingUser.ID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if existingUser.ID != 0 {
		return errors.New("username_or_email_exists")
	}

	queryBuilder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("users").
		Set("username", user.Username).
		Set("email", user.Email).
		Set("profile_picture_url", user.ProfilePictureURL).
		Set("bio", user.Bio).
		Where(squirrel.Eq{"id": id}).
		Suffix("RETURNING updated_at")

	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		log.Printf("Error building SQL for updateUser: %s, error: %v", sql, err)
		return err
	}

	err = db.QueryRow(sql, args...).Scan(&user.UpdatedAt)
	if err != nil {
		log.Printf("Error executing updateUser: %s, args: %v, error: %v", sql, args, err)
		return err
	}

	// Audit log
	log.Printf("User updated: %s", user.Username)

	return nil
}

// FUTURE! // Implement a soft delete mechanism instead of permanently deleting user records.
func deleteUser(db *sql.DB, id int) error {
	deletedAt := time.Now()
	queryBuilder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("users").
		Set("deleted_at", deletedAt). // Soft delete by setting deleted_at
		Where(squirrel.Eq{"id": id})
	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		log.Printf("Error building SQL for deleteUser: %s, error: %v", sql, err)
		return err
	}

	result, err := db.Exec(sql, args...)
	if err != nil {
		log.Printf("Error executing deleteUser: %s, args: %v, error: %v", sql, args, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error fetching rows affected after deleteUser: %v", err)
		return err
	}

	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	// Audit log
	log.Printf("User soft deleted: %d", id)

	return nil
}

type CustomValidator struct {
	validator *validator.Validate
}

// FUTURE! // Extend the custom validator to include custom validation rules specific to your application's requirements.
func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

// FUTURE! // Add connection pooling and better error handling for database connection failures.
func dbConnect(cfg *Config) (*sql.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		cfg.Database.Host,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.Port,
		cfg.Database.SSLMode,
	)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}

// FUTURE! // Consider breaking down the main function into smaller, more modular functions for better readability and maintainability.
func main() {
	config, err := readConfig("config.json")
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	location, err := time.LoadLocation(config.App.TimeZone)
	if err != nil {
		log.Fatalf("Error loading timezone: %v", err)
	}
	time.Local = location

	db, err := dbConnect(config)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:4200"},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	// Implement rate limiting
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(config.App.RateLimit)))

	// Set log level based on configuration
	switch config.App.LogLevel {
	case "DEBUG":
		e.Logger.SetLevel(log.DEBUG)
	case "INFO":
		e.Logger.SetLevel(log.INFO)
	case "WARN":
		e.Logger.SetLevel(log.WARN)
	case "ERROR":
		e.Logger.SetLevel(log.ERROR)
	default:
		e.Logger.SetLevel(log.INFO)
	}

	e.Validator = &CustomValidator{validator: validator.New()}

	e.GET("/swagger/*", echoSwagger.WrapHandler)

	e.GET("/users", func(c echo.Context) error {
		page, err := strconv.Atoi(c.QueryParam("page"))
		if err != nil || page < 1 {
			page = 1
		}
		pageSize, err := strconv.Atoi(c.QueryParam("pageSize"))
		if err != nil || pageSize < 1 {
			pageSize = 10
		}

		users, err := getUsers(db, page, pageSize)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to retrieve users"})
		}
		return c.JSON(http.StatusOK, users)
	})

	// FUTURE! // Add comprehensive error handling and logging across all endpoints to capture and log more detailed information.
	e.GET("/users/:id", func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid user ID"})
		}
		user, err := getUserByID(db, id)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "User not found"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to retrieve user"})
		}
		return c.JSON(http.StatusOK, user)
	})

	// @Summary Create a new user
	// @Description Create a new user with the provided details
	// @Tags users
	// @Accept json
	// @Produce json
	// @Param user body User true "User"
	// @Success 201 {object} User
	// @Failure 400 {object} map[string]interface{}
	// @Failure 500 {object} map[string]interface{}
	// @Router /users [post]
	e.POST("/users", func(c echo.Context) error {
		var user User
		if err := c.Bind(&user); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "invalid_request_payload"})
		}
		if err := c.Validate(user); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "validation_failed", "details": err.Error()})
		}
		err := createUser(db, &user)
		if err != nil {
			if err.Error() == "username_or_email_exists" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "username_or_email_exists"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "failed_to_create_user"})
		}
		return c.JSON(http.StatusCreated, user)
	})

	// @Summary Update an existing user
	// @Description Update an existing user by their ID
	// @Tags users
	// @Accept json
	// @Produce json
	// @Param id path int true "User ID"
	// @Param user body User true "User"
	// @Success 200 {object} User
	// @Failure 400 {object} map[string]interface{}
	// @Failure 404 {object} map[string]interface{}
	// @Failure 500 {object} map[string]interface{}
	// @Router /users/{id} [put]
	e.PUT("/users/:id", func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "invalid_user_id"})
		}
		var user User
		if err := c.Bind(&user); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "invalid_request_payload"})
		}
		if err := c.Validate(user); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "validation_failed", "details": err.Error()})
		}
		err = updateUser(db, id, &user)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "user_not_found"})
			}
			if err.Error() == "username_or_email_exists" {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "username_or_email_exists"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "failed_to_update_user"})
		}
		return c.JSON(http.StatusOK, user)
	})

	// @Summary Delete a user
	// @Description Delete a user by their ID
	// @Tags users
	// @Param id path int true "User ID"
	// @Success 204 {object} nil
	// @Failure 400 {object} map[string]interface{}
	// @Failure 404 {object} map[string]interface{}
	// @Failure 500 {object} map[string]interface{}
	// @Router /users/{id} [delete]
	e.DELETE("/users/:id", func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid user ID"})
		}
		err = deleteUser(db, id)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "User not found"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to delete user"})
		}
		return c.NoContent(http.StatusNoContent)
	})

	e.GET("/swagger/*", echoSwagger.WrapHandler)
	e.Logger.Fatal(e.Start(":8080"))
}

// FUTURE! // Add comprehensive error handling and logging across all endpoints to capture and log more detailed information.
