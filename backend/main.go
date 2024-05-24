package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
		TimeZone string `json:"timezone"`
	} `json:"app"`
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

type User struct {
	gorm.Model
	Username string `json:"username" gorm:"unique;not null" validate:"required"`
	Email    string `json:"email" gorm:"unique;not null" validate:"required,email"`
}

type CustomValidator struct {
	validator *validator.Validate
}

func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

type UserHandler struct {
	DB *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{DB: db}
}

func (h *UserHandler) GetUsers(c echo.Context) error {
	var users []User
	result := h.DB.Find(&users)
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to retrieve users from the database"})
	}
	return c.JSON(http.StatusOK, users)
}

func (h *UserHandler) GetUserByID(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid user ID"})
	}

	var user User
	result := h.DB.First(&user, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "User not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to retrieve user from the database"})
	}

	return c.JSON(http.StatusOK, user)
}

func (h *UserHandler) CreateUser(c echo.Context) error {
	var user User
	if err := c.Bind(&user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request payload"})
	}

	if err := c.Validate(user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
	}

	result := h.DB.Create(&user)
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to create user in the database"})
	}

	return c.JSON(http.StatusCreated, user)
}

func (h *UserHandler) UpdateUser(c echo.Context) error {
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

	result := h.DB.Model(&User{}).Where("id = ?", id).Updates(user)
	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "User not found"})
	}
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to update user in the database"})
	}

	return c.JSON(http.StatusOK, user)
}

func (h *UserHandler) DeleteUser(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid user ID"})
	}

	result := h.DB.Delete(&User{}, id)
	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "User not found"})
	}
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to delete user from the database"})
	}

	return c.NoContent(http.StatusNoContent)
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

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		config.Database.Host,
		config.Database.User,
		config.Database.Password,
		config.Database.DBName,
		config.Database.Port,
		config.Database.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}

	db.AutoMigrate(&User{})

	e := echo.New()

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:4200"}, // Your Angular frontend URL
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	e.Validator = &CustomValidator{validator: validator.New()}

	userHandler := NewUserHandler(db)
	e.GET("/users", userHandler.GetUsers)
	e.GET("/users/:id", userHandler.GetUserByID)
	e.POST("/users", userHandler.CreateUser)
	e.PUT("/users/:id", userHandler.UpdateUser)
	e.DELETE("/users/:id", userHandler.DeleteUser)

	e.Logger.Fatal(e.Start(":8080"))
}
