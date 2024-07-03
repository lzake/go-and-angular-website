package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/lzake/gowebsite-backend/handlers" 
	"github.com/lzake/gowebsite-backend/models" 
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB
var e *echo.Echo
var userHandler *handlers.UserHandler

var _ = ginkgo.BeforeSuite(func() {
	// set up your test database connection here:
	dsn := "host=localhost user=your_test_user password=your_test_password dbname=your_test_db port=5432 sslmode=disable TimeZone=Asia/Shanghai"
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to the test database!")
	}

	// migrate your User model:
	db.AutoMigrate(&models.User{})

	// init Echo:
	e = echo.New()
	e.Validator = &handlers.CustomValidator{validator: validator.New()}
	userHandler = handlers.NewUserHandler(db)
})

var _ = ginkgo.AfterSuite(func() {
	// close the test database connection
	sqlDB, _ := db.DB()
	sqlDB.Close()
})

var _ = ginkgo.BeforeEach(func() {
	// clear the test database before each test. 
	db.Exec("DELETE FROM users")
})

ginkgo.Describe("User Handler", func() {
	ginkgo.Context("CreateUser", func() {
		ginkgo.It("Should create a new user successfully", func() {
			// define your test user data
			testUser := models.User{Username: "testuser", Email: "testuser@example.com"}
			reqBody, _ := json.Marshal(testUser)

			// create a test request
			req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(string(reqBody)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			// perform the request
			rec := httptest.NewRecorder()
			e.POST("/users", userHandler.CreateUser).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusCreated))

			// unmarshal the response body:
			var createdUser models.User
			json.Unmarshal(rec.Body.Bytes(), &createdUser)
			gomega.Expect(createdUser.Username).To(gomega.Equal("testuser"))
			gomega.Expect(createdUser.Email).To(gomega.Equal("testuser@example.com"))
		})

		ginkgo.It("Should return an error for invalid user data", func() {
			// define a user with invalid data
			testUser := models.User{Username: "", Email: "invalid_email"}
			reqBody, _ := json.Marshal(testUser)

			// create a test request
			req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(string(reqBody)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			// perform the request
			rec := httptest.NewRecorder()
			e.POST("/users", userHandler.CreateUser).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusBadRequest))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Not(gomega.BeNil()))
		})

		ginkgo.It("Should return an error for duplicate username", func() {
			// create a test user
			existingUser := models.User{Username: "duplicateuser", Email: "duplicateuser@example.com"}
			db.Create(&existingUser)

			// create another user with the same username
			testUser := models.User{Username: "duplicateuser", Email: "another@example.com"}
			reqBody, _ := json.Marshal(testUser)

			// create a test request
			req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(string(reqBody)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			// perform the request
			rec := httptest.NewRecorder()
			e.POST("/users", userHandler.CreateUser).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusBadRequest))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Equal("username_or_email_exists"))
		})
	})

	ginkgo.Context("GetUserByID", func() {
		ginkgo.It("Should return a user by ID successfully", func() {
			// create a test user
			testUser := models.User{Username: "testuser", Email: "testuser@example.com"}
			db.Create(&testUser)

			// get the user's ID from the database
			var userID int
			db.Model(&testUser).Select("id").Scan(&userID)

			// create a test request
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/users/%d", userID), nil)
			rec := httptest.NewRecorder()
			e.GET("/users/:id", userHandler.GetUserByID).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusOK))

			// unmarshal the response body
			var userResponse models.User
			json.Unmarshal(rec.Body.Bytes(), &userResponse)
			gomega.Expect(userResponse.Username).To(gomega.Equal("testuser"))
			gomega.Expect(userResponse.Email).To(gomega.Equal("testuser@example.com"))
		})

		ginkgo.It("Should return an error for an invalid user ID", func() {
			// make a request with an invalid ID
			req := httptest.NewRequest(http.MethodGet, "/users/invalid", nil)
			rec := httptest.NewRecorder()
			e.GET("/users/:id", userHandler.GetUserByID).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusBadRequest))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Not(gomega.BeNil()))
		})

		ginkgo.It("Should return a 404 error for a non-existent user ID", func() {
			// create a test request with a non-existent user ID
			req := httptest.NewRequest(http.MethodGet, "/users/999", nil)
			rec := httptest.NewRecorder()
			e.GET("/users/:id", userHandler.GetUserByID).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusNotFound))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Not(gomega.BeNil()))
		})
	})

	ginkgo.Context("UpdateUser", func() {
		ginkgo.It("Should update a user by ID successfully", func() {
			// create a test user
			testUser := models.User{Username: "testuser", Email: "testuser@example.com"}
			db.Create(&testUser)

			// get the user's ID from the database
			var userID int
			db.Model(&testUser).Select("id").Scan(&userID)

			// create updated user data
			updatedUser := models.User{Username: "updateduser", Email: "updateduser@example.com"}
			reqBody, _ := json.Marshal(updatedUser)

			// create a test request
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/users/%d", userID), strings.NewReader(string(reqBody)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			// perform the request
			rec := httptest.NewRecorder()
			e.PUT("/users/:id", userHandler.UpdateUser).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusOK))

			// unmarshal the response body
			var updatedResponse models.User
			json.Unmarshal(rec.Body.Bytes(), &updatedResponse)
			gomega.Expect(updatedResponse.Username).To(gomega.Equal("updateduser"))
			gomega.Expect(updatedResponse.Email).To(gomega.Equal("updateduser@example.com"))
		})

		ginkgo.It("Should return an error for invalid user ID", func() {
			// create a test request with an invalid ID
			req := httptest.NewRequest(http.MethodPut, "/users/invalid", nil)
			rec := httptest.NewRecorder()
			e.PUT("/users/:id", userHandler.UpdateUser).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusBadRequest))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Not(gomega.BeNil()))
		})

		ginkgo.It("Should return an error for invalid user data", func() {
			// create a test user
			testUser := models.User{Username: "testuser", Email: "testuser@example.com"}
			db.Create(&testUser)

			// get the user's ID from the database
			var userID int
			db.Model(&testUser).Select("id").Scan(&userID)

			// create invalid updated user data
			updatedUser := models.User{Username: "", Email: "invalid_email"}
			reqBody, _ := json.Marshal(updatedUser)

			// create a test request
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/users/%d", userID), strings.NewReader(string(reqBody)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			// perform the request
			rec := httptest.NewRecorder()
			e.PUT("/users/:id", userHandler.UpdateUser).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusBadRequest))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Not(gomega.BeNil()))
		})

		ginkgo.It("Should return an error for duplicate username or email", func() {
			// create two test users
			testUser1 := models.User{Username: "testuser1", Email: "testuser1@example.com"}
			db.Create(&testUser1)
			testUser2 := models.User{Username: "testuser2", Email: "testuser2@example.com"}
			db.Create(&testUser2)

			// get the first user's ID from the database
			var userID1 int
			db.Model(&testUser1).Select("id").Scan(&userID1)

			// create updated user data with the same username as the second user
			updatedUser := models.User{Username: "testuser2", Email: "updated@example.com"}
			reqBody, _ := json.Marshal(updatedUser)

			// create a test request
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/users/%d", userID1), strings.NewReader(string(reqBody)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			// perform the request
			rec := httptest.NewRecorder()
			e.PUT("/users/:id", userHandler.UpdateUser).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusBadRequest))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Equal("username_or_email_exists"))
		})

		ginkgo.It("Should return a 404 error for a non-existent user ID", func() {
			// create a test request with a non-existent user ID
			req := httptest.NewRequest(http.MethodPut, "/users/999", nil)
			rec := httptest.NewRecorder()
			e.PUT("/users/:id", userHandler.UpdateUser).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusNotFound))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Not(gomega.BeNil()))
		})
	})

	ginkgo.Context("DeleteUser", func() {
		ginkgo.It("Should delete a user by ID successfully", func() {
			// create a test user
			testUser := models.User{Username: "testuser", Email: "testuser@example.com"}
			db.Create(&testUser)

			// get the user's ID from the database
			var userID int
			db.Model(&testUser).Select("id").Scan(&userID)

			// create a test request
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/users/%d", userID), nil)
			rec := httptest.NewRecorder()
			e.DELETE("/users/:id", userHandler.DeleteUser).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusNoContent))

			// Verify that the user is deleted from the database
			var deletedUser models.User
			db.First(&deletedUser, userID)
			gomega.Expect(deletedUser.ID).To(gomega.BeZero())
		})

		ginkgo.It("Should return an error for an invalid user ID", func() {
			// make a request with an invalid ID
			req := httptest.NewRequest(http.MethodDelete, "/users/invalid", nil)
			rec := httptest.NewRecorder()
			e.DELETE("/users/:id", userHandler.DeleteUser).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusBadRequest))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Not(gomega.BeNil()))
		})

		ginkgo.It("Should return a 404 error for a non-existent user ID", func() {
			// create a test request with a non-existent user ID
			req := httptest.NewRequest(http.MethodDelete, "/users/999", nil)
			rec := httptest.NewRecorder()
			e.DELETE("/users/:id", userHandler.DeleteUser).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusNotFound))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Not(gomega.BeNil()))
		})
	})

	ginkgo.Context("GetUsers", func() {
		ginkgo.It("Should return a list of all users", func() {
			// create some test users
			testUser1 := models.User{Username: "testuser1", Email: "testuser1@example.com"}
			db.Create(&testUser1)
			testUser2 := models.User{Username: "testuser2", Email: "testuser2@example.com"}
			db.Create(&testUser2)

			// create a test request
			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			rec := httptest.NewRecorder()
			e.GET("/users", userHandler.GetUsers).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusOK))

			// unmarshal the response body
			var usersResponse []models.User
			json.Unmarshal(rec.Body.Bytes(), &usersResponse)
			gomega.Expect(len(usersResponse)).To(gomega.Equal(2))
		})
	})
})

func TestUserHandler(t *testing.T) {
	ginkgo.RunSpecs(t, "User Handler Suite")
}