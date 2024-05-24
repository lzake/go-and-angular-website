package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username string `json:"username" gorm:"unique;not null" validate:"required"`
	Email    string `json:"email" gorm:"unique;not null" validate:"required,email"`
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
