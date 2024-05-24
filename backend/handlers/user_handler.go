package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
)

var ErrNoRowsAffected = errors.New("no rows affected")

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserHandler struct {
	DB *sql.DB
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
	if err != nil {
		return user, err
	}
	return user, nil
}
func createUser(db *sql.DB, user *User) error {
	queryBuilder := squirrel.Insert("users").Columns("username", "email").Values(user.Username, user.Email).Suffix("RETURNING id, created_at, updated_at")
	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		return err
	}

	err = db.QueryRow(sql, args...).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return err
	}
	return nil
}
func updateUser(db *sql.DB, id int, user *User) error {
	queryBuilder := squirrel.Update("users").Set("username", user.Username).Set("email", user.Email).Where(squirrel.Eq{"id": id}).Suffix("RETURNING updated_at")
	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		return err
	}

	err = db.QueryRow(sql, args...).Scan(&user.UpdatedAt)
	if err != nil {
		return err
	}
	return nil
}
func deleteUser(db *sql.DB, id int) error {
	queryBuilder := squirrel.Delete("users").Where(squirrel.Eq{"id": id})
	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		return err
	}

	_, err = db.Exec(sql, args...)
	if err != nil {
		return err
	}
	return nil
}

func NewUserHandler(db *sql.DB) *UserHandler {
	return &UserHandler{DB: db}
}

func (h *UserHandler) GetUsers(c echo.Context) error {
	users, err := getUsers(h.DB)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to retrieve users"})
	}
	return c.JSON(http.StatusOK, users)
}

func (h *UserHandler) GetUserByID(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid user ID"})
	}

	user, err := getUserByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "User not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to retrieve user"})
	}
	return c.JSON(http.StatusOK, user)
}

func (h *UserHandler) CreateUser(c echo.Context) error {
	var user User
	if err := c.Bind(&user); err != nil {
		log.Printf("Error binding user data: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request payload"})
	}

	if err := c.Validate(user); err != nil {
		log.Printf("Validation error for user: %v, error: %v", user, err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
	}

	err := createUser(h.DB, &user)
	if err != nil {
		log.Printf("Error creating user in database: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to create user"})
	}
	return c.JSON(http.StatusCreated, user)
}

func (h *UserHandler) UpdateUser(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("Error converting user ID to integer: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid user ID"})
	}

	var user User
	if err := c.Bind(&user); err != nil {
		log.Printf("Error binding user data: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid request payload"})
	}

	if err := c.Validate(user); err != nil {
		log.Printf("Validation error for user: %v, error: %v", user, err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
	}

	err = updateUser(h.DB, id, &user)
	if err != nil {
		if errors.Is(err, ErrNoRowsAffected) {
			log.Printf("No user found with ID %d to update", id)
			return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "User not found"})
		}
		log.Printf("Error updating user with ID %d: %v", id, err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to update user"})
	}
	return c.JSON(http.StatusOK, user)
}

func (h *UserHandler) DeleteUser(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("Error converting user ID to integer: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Invalid user ID"})
	}

	err = deleteUser(h.DB, id)
	if err != nil {
		if err.Error() == "user not found" {
			log.Printf("No user found with ID %d to delete", id)
			return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "User not found"})
		}
		log.Printf("Error deleting user with ID %d: %v", id, err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to delete user"})
	}
	return c.NoContent(http.StatusNoContent)
}
