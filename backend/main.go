package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/lzake/gowebsite-backend/docs"

	"github.com/Masterminds/squirrel"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
	echoSwagger "github.com/swaggo/echo-swagger"
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

var statementBuilder = squirrel.StatementBuilder

func init() {
	statementBuilder = statementBuilder.PlaceholderFormat(squirrel.Dollar)
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
		TimeZone string `json:"timezone"`
	} `json:"app"`
}

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func readConfig(filename string) (*Config, error) {
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

// @Summary Get all users
// @Description Get all users
// @Tags users
// @Accept  json
// @Produce  json
// @Success 200 {array} User
// @Failure 500 {object} map[string]interface{}
// @Router /users [get]
func getUsers(db *sql.DB) ([]User, error) {
	queryBuilder := squirrel.Select("id", "username", "email", "created_at", "updated_at").From("users")
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
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// @Summary Get user by ID
// @Description Get user by ID
// @Tags users
// @Accept  json
// @Produce  json
// @Param id path int true "User ID"
// @Success 200 {object} User
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users/{id} [get]
func getUserByID(db *sql.DB, id int) (User, error) {
	var user User
	queryBuilder := squirrel.Select("id", "username", "email", "created_at", "updated_at").From("users").Where(squirrel.Eq{"id": id})
	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		return user, err
	}

	err = db.QueryRow(sql, args...).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

// @Summary Create a new user
// @Description Create a new user
// @Tags users
// @Accept  json
// @Produce  json
// @Param user body User true "User"
// @Success 201 {object} User
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users [post]
func createUser(db *sql.DB, user *User) error {
	queryBuilder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Insert("users").
		Columns("username", "email").
		Values(user.Username, user.Email).
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
	return nil
}

// @Summary Update user
// @Description Update user
// @Tags users
// @Accept  json
// @Produce  json
// @Param id path int true "User ID"
// @Param user body User true "User"
// @Success 200 {object} User
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users/{id} [put]
func updateUser(db *sql.DB, id int, user *User) error {
	queryBuilder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("users").
		Set("username", user.Username).
		Set("email", user.Email).
		Where(squirrel.Eq{"id": id}).
		Suffix("RETURNING updated_at")

	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		log.Printf("Error building SQL for updateUser: %s, error: %v", sql, err)
		return err
	}

	log.Printf("Executing updateUser: %s, args: %v", sql, args)

	err = db.QueryRow(sql, args...).Scan(&user.UpdatedAt)
	if err != nil {
		log.Printf("Error executing updateUser: %s, args: %v, error: %v", sql, args, err)
		return err
	}

	log.Printf("User updated successfully with ID: %d, updated_at: %v", id, user.UpdatedAt)

	return nil
}

func deleteUser(db *sql.DB, id int) error {
	queryBuilder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Delete("users").
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
		return errors.New("user not found") // Return a custom error
	}

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

// @Summary List all users
// @Description Get a list of all users
// @Tags users
// @Produce json
// @Success 200 {array} User
// @Failure 500 {object} map[string]interface{}
// @Router /users [get]
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

	e.Validator = &CustomValidator{validator: validator.New()}

	e.GET("/swagger/*", echoSwagger.WrapHandler)

	e.GET("/users", func(c echo.Context) error {
		users, err := getUsers(db)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to retrieve users"})
		}
		return c.JSON(http.StatusOK, users)
	})

	// @Summary Get a user by ID
	// @Description Get a user by their ID
	// @Tags users
	// @Produce json
	// @Param id path int true "User ID"
	// @Success 200 {object} User
	// @Failure 400 {object} map[string]interface{}
	// @Failure 404 {object} map[string]interface{}
	// @Failure 500 {object} map[string]interface{}
	// @Router /users/{id} [get]
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
			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request payload"})
		}
		if err := c.Validate(user); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		}
		err := createUser(db, &user)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to create user"})
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
			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid user ID"})
		}
		var user User
		if err := c.Bind(&user); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request payload"})
		}
		if err := c.Validate(user); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		}
		err = updateUser(db, id, &user)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "User not found"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to update user"})
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
