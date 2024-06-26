// package backend
package main

import (
	"math"
	"testing"
)

//test per handler
// func testuser_handler_getUserByID
// it

func TestBackend(t *testing.T) {
	got := math.Abs(-1)
	if got != 1 {
		t.Errorf("Abs(-1) = %f; want 1", got)
	}
}

// func (h *UserHandler) UpdateUser(c echo.Context) error {
// 	id, err := strconv.Atoi(c.Param("id"))
// 	if err != nil {
// 		log.Printf("Error converting user ID to integer: %v", err)
// 		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "invalid_user_id"})
// 	}

// 	var user User
// 	if err := c.Bind(&user); err != nil {
// 		log.Printf("Error binding user data: %v", err)
// 		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "invalid_request_payload"})
// 	}

// 	if err := c.Validate(user); err != nil {
// 		log.Printf("Validation error for user: %v, error: %v", user, err)
// 		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "validation_failed", "details": err.Error()})
// 	}

// 	err = updateUser(h.DB, id, &user)
// 	if err != nil {
// 		if errors.Is(err, ErrNoRowsAffected) {
// 			log.Printf("No user found with ID %d to update", id)
// 			return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "user_not_found"})
// 		}
// 		if err.Error() == "username_or_email_exists" {
// 			return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "username_or_email_exists"})
// 		}
// 		log.Printf("Error updating user with ID %d: %v", id, err)
// 		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "failed_to_update_user"})
// 	}
// 	return c.JSON(http.StatusOK, user)
// }

// type MockDB struct {
// 	mock.Mock
// }

// func (mdb *MockDB) updateUser(id int, user *User){
// 	args := mdb.Called(id, user)
// 	return args.Error(0)
// }

// var _ = Describe("UpdateUser", func(){
// 	var (
// 		e			*echo.Echo
// 		req			*http.Request
// 		rec			*httptest.ResponseRecorder
// 		c			*echo.Context
// 		h			*UserHandler
// 		mockDB		*MockDB
// 		user

// 	)
// })
