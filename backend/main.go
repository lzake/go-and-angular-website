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

	"github.com/Masterminds/squirrel"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
)

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

	e.GET("/users", func(c echo.Context) error {
		users, err := getUsers(db)
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

	e.Logger.Fatal(e.Start(":8080"))
}
