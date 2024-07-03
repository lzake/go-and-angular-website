package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/patrickmn/go-cache"
	echoSwagger "github.com/swaggo/echo-swagger"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

var (
	statementBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	userCache        = cache.New(5*time.Minute, 10*time.Minute) // Initializing cache
)

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
	Password          string     `json:"password,omitempty"`
	ProfilePictureURL string     `json:"profile_picture_url"`
	Bio               string     `json:"bio"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"`
}

func readConfig(filename string) (*Config, error) {
	err := godotenv.Load() // Load environment variables from .env file
	if err != nil {
		fmt.Println("Error loading .env file, using config.json")
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

func getUsers(db *sql.DB, page int, pageSize int) ([]User, error) {
	offset := (page - 1) * pageSize

	queryBuilder := squirrel.Select("id", "username", "email", "profile_picture_url", "bio", "created_at", "updated_at").
		From("users").
		Where(squirrel.Eq{"deleted_at": nil}).
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

func getUserByID(db *sql.DB, id int) (User, error) {
	if cachedUser, found := userCache.Get(strconv.Itoa(id)); found {
		return cachedUser.(User), nil
	}

	var user User
	queryBuilder := squirrel.Select("id", "username", "email", "profile_picture_url", "bio", "created_at", "updated_at").From("users").Where(squirrel.Eq{"id": id, "deleted_at": nil})
	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		return user, err
	}

	err = db.QueryRow(sql, args...).Scan(&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return user, err
	}

	userCache.Set(strconv.Itoa(id), user, cache.DefaultExpiration)

	return user, nil
}

func createUser(db *sql.DB, user *User) error {
	var existingUser User
	err := db.QueryRow("SELECT id FROM users WHERE username = $1 OR email = $2", user.Username, user.Email).Scan(&existingUser.ID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if existingUser.ID != 0 {
		return errors.New("username_or_email_exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hashedPassword)

	verificationToken := "dummy_verification_token"

	queryBuilder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Insert("users").
		Columns("username", "email", "password", "profile_picture_url", "bio", "verification_token").
		Values(user.Username, user.Email, user.Password, user.ProfilePictureURL, user.Bio, verificationToken).
		Suffix("RETURNING id, created_at, updated_at")

	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		fmt.Printf("Error building SQL for createUser: %s, error: %v", sql, err)
		return err
	}

	err = db.QueryRow(sql, args...).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		fmt.Printf("Error executing createUser: %s, args: %v, error: %v", sql, args, err)
		return err
	}

	fmt.Printf("Sending verification email to %s with token %s", user.Email, verificationToken)
	fmt.Printf("User created: %s", user.Username)

	return nil
}

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
		fmt.Printf("Error building SQL for updateUser: %s, error: %v", sql, err)
		return err
	}

	err = db.QueryRow(sql, args...).Scan(&user.UpdatedAt)
	if err != nil {
		fmt.Printf("Error executing updateUser: %s, args: %v, error: %v", sql, args, err)
		return err
	}

	fmt.Printf("User updated: %s", user.Username)

	return nil
}

func deleteUser(db *sql.DB, id int) error {
	deletedAt := time.Now()
	queryBuilder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("users").
		Set("deleted_at", deletedAt).
		Where(squirrel.Eq{"id": id})
	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		fmt.Printf("Error building SQL for deleteUser: %s, error: %v", sql, err)
		return err
	}

	result, err := db.Exec(sql, args...)
	if err != nil {
		fmt.Printf("Error executing deleteUser: %s, args: %v, error: %v", sql, args, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		fmt.Printf("Error fetching rows affected after deleteUser: %v", err)
		return err
	}

	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	fmt.Printf("User soft deleted: %d", id)

	return nil
}

type CustomValidator struct {
	validator *validator.Validate
}

func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

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

	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(config.App.RateLimit))))

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
