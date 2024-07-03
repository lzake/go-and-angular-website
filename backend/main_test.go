package test

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
	"github.com/patrickmn/go-cache"
	echoSwagger "github.com/swaggo/echo-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/lzake/gowebsite-backend/main"
)

// Import your main package
// load environment variables for test
var _ = ginkgo.BeforeSuite(func() {
	err := godotenv.Load(".env.test")
	if err != nil {
		log.Fatal("Error loading .env.test file:", err)
	}
})

// global variables for test setup
var (
	db  *gorm.DB
	e   *echo.Echo
	cfg *main.Config
)

var _ = ginkgo.BeforeSuite(func() {
	// init cache
	main.UserCache = cache.New(5*time.Minute, 10*time.Minute)

	// set up the test database connection
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		getEnvAsInt("DB_PORT", 5432),
		os.Getenv("DB_SSLMODE"),
	)
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to the test database!")
	}
	db.AutoMigrate(&main.User{})

	// read config
	cfg, err = main.ReadConfig("config.test.json") // Use a separate config file for tests
	if err != nil {
		panic("Error reading test configuration file: " + err.Error())
	}

	// init Echo
	e = echo.New()
	e.Validator = &main.CustomValidator{validator: validator.New()}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:4200"},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(cfg.App.RateLimit)))
	e.Logger.SetLevel(log.DebugLevel) // set logging level for tests
	e.GET("/swagger/*", echoSwagger.WrapHandler)
})

var _ = ginkgo.AfterSuite(func() {
	// close the test database connection
	sqlDB, _ := db.DB()
	sqlDB.Close()
})

var _ = ginkgo.BeforeEach(func() {
	// clear the test database before each test
	db.Exec("DELETE FROM users")
})

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

func TestMainApplication(t *testing.T) {
	ginkgo.RunSpecs(t, "Main Application Suite")
}

var _ = ginkgo.Describe("Main Application", func() {
	ginkgo.Context("CreateUser", func() {
		ginkgo.It("Should create a new user successfully", func() {
			// define your test user data
			testUser := main.User{Username: "testuser", Email: "testuser@example.com", Password: "password123", ProfilePictureURL: "https://example.com/profile.jpg", Bio: "Test User Bio"}
			reqBody, _ := json.Marshal(testUser)

			// create a test request
			req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(string(reqBody)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			// perform the request
			rec := httptest.NewRecorder()
			e.POST("/users", func(c echo.Context) error {
				return main.CreateUser(db, &testUser)
			}).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusCreated))

			// unmarshal the response body:
			var createdUser main.User
			json.Unmarshal(rec.Body.Bytes(), &createdUser)
			gomega.Expect(createdUser.Username).To(gomega.Equal("testuser"))
			gomega.Expect(createdUser.Email).To(gomega.Equal("testuser@example.com"))
			gomega.Expect(createdUser.ProfilePictureURL).To(gomega.Equal("https://example.com/profile.jpg"))
			gomega.Expect(createdUser.Bio).To(gomega.Equal("Test User Bio"))
			gomega.Expect(createdUser.Password).ToNot(gomega.Equal("password123")) // Password should be hashed
		})

		ginkgo.It("Should return an error for invalid user data", func() {
			// define a user with invalid data
			testUser := main.User{Username: "", Email: "invalid_email", Password: "password123"}
			reqBody, _ := json.Marshal(testUser)

			// create a test request
			req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(string(reqBody)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			// perform the request
			rec := httptest.NewRecorder()
			e.POST("/users", func(c echo.Context) error {
				return main.CreateUser(db, &testUser)
			}).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusBadRequest))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Not(gomega.BeNil()))
		})

		ginkgo.It("Should return an error for duplicate username", func() {
			// create a test user
			existingUser := main.User{Username: "duplicateuser", Email: "duplicateuser@example.com", Password: "password123"}
			db.Create(&existingUser)

			// create another user with the same username
			testUser := main.User{Username: "duplicateuser", Email: "another@example.com", Password: "password123"}
			reqBody, _ := json.Marshal(testUser)

			// create a test request
			req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(string(reqBody)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			// perform the request
			rec := httptest.NewRecorder()
			e.POST("/users", func(c echo.Context) error {
				return main.CreateUser(db, &testUser)
			}).ServeHTTP(rec, req)

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
			testUser := main.User{Username: "testuser", Email: "testuser@example.com", Password: "password123"}
			db.Create(&testUser)

			// get the user's ID from the database
			var userID int
			db.Model(&testUser).Select("id").Scan(&userID)

			// create a test request
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/users/%d", userID), nil)
			rec := httptest.NewRecorder()
			e.GET("/users/:id", func(c echo.Context) error {
				return main.GetUserByID(db, userID)
			}).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusOK))

			// unmarshal the response body
			var userResponse main.User
			json.Unmarshal(rec.Body.Bytes(), &userResponse)
			gomega.Expect(userResponse.Username).To(gomega.Equal("testuser"))
			gomega.Expect(userResponse.Email).To(gomega.Equal("testuser@example.com"))
			gomega.Expect(userResponse.Password).To(gomega.Equal("")) // Password should not be returned in response
		})

		ginkgo.It("Should return an error for an invalid user ID", func() {
			// make a request with an invalid ID
			req := httptest.NewRequest(http.MethodGet, "/users/invalid", nil)
			rec := httptest.NewRecorder()
			e.GET("/users/:id", func(c echo.Context) error {
				id, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					return err
				}
				return main.GetUserByID(db, id)
			}).ServeHTTP(rec, req)

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
			e.GET("/users/:id", func(c echo.Context) error {
				id, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					return err
				}
				return main.GetUserByID(db, id)
			}).ServeHTTP(rec, req)

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
			testUser := main.User{Username: "testuser", Email: "testuser@example.com", Password: "password123"}
			db.Create(&testUser)

			// get the user's ID from the database
			var userID int
			db.Model(&testUser).Select("id").Scan(&userID)

			// create updated user data
			updatedUser := main.User{Username: "updateduser", Email: "updateduser@example.com", Password: "newpassword123", ProfilePictureURL: "https://example.com/newprofile.jpg", Bio: "Updated User Bio"}
			reqBody, _ := json.Marshal(updatedUser)

			// create a test request
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/users/%d", userID), strings.NewReader(string(reqBody)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			// perform the request
			rec := httptest.NewRecorder()
			e.PUT("/users/:id", func(c echo.Context) error {
				id, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					return err
				}
				return main.UpdateUser(db, id, &updatedUser)
			}).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusOK))

			// unmarshal the response body
			var updatedResponse main.User
			json.Unmarshal(rec.Body.Bytes(), &updatedResponse)
			gomega.Expect(updatedResponse.Username).To(gomega.Equal("updateduser"))
			gomega.Expect(updatedResponse.Email).To(gomega.Equal("updateduser@example.com"))
			gomega.Expect(updatedResponse.ProfilePictureURL).To(gomega.Equal("https://example.com/newprofile.jpg"))
			gomega.Expect(updatedResponse.Bio).To(gomega.Equal("Updated User Bio"))
			gomega.Expect(updatedResponse.Password).ToNot(gomega.Equal("newpassword123")) // Password should be hashed
		})

		ginkgo.It("Should return an error for invalid user ID", func() {
			// create a test request with an invalid ID
			req := httptest.NewRequest(http.MethodPut, "/users/invalid", nil)
			rec := httptest.NewRecorder()
			e.PUT("/users/:id", func(c echo.Context) error {
				id, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					return err
				}
				return main.UpdateUser(db, id, &main.User{})
			}).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusBadRequest))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Not(gomega.BeNil()))
		})

		ginkgo.It("Should return an error for invalid user data", func() {
			// create a test user
			testUser := main.User{Username: "testuser", Email: "testuser@example.com", Password: "password123"}
			db.Create(&testUser)

			// get the user's ID from the database
			var userID int
			db.Model(&testUser).Select("id").Scan(&userID)

			// create invalid updated user data
			updatedUser := main.User{Username: "", Email: "invalid_email", Password: "newpassword123"}
			reqBody, _ := json.Marshal(updatedUser)

			// create a test request
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/users/%d", userID), strings.NewReader(string(reqBody)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			// perform the request
			rec := httptest.NewRecorder()
			e.PUT("/users/:id", func(c echo.Context) error {
				id, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					return err
				}
				return main.UpdateUser(db, id, &updatedUser)
			}).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusBadRequest))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Not(gomega.BeNil()))
		})

		ginkgo.It("Should return an error for duplicate username or email", func() {
			// create two test users
			testUser1 := main.User{Username: "testuser1", Email: "testuser1@example.com", Password: "password123"}
			db.Create(&testUser1)
			testUser2 := main.User{Username: "testuser2", Email: "testuser2@example.com", Password: "password123"}
			db.Create(&testUser2)

			// get the first user's ID from the database
			var userID1 int
			db.Model(&testUser1).Select("id").Scan(&userID1)

			// create updated user data with the same username as the second user
			updatedUser := main.User{Username: "testuser2", Email: "updated@example.com", Password: "newpassword123"}
			reqBody, _ := json.Marshal(updatedUser)

			// create a test request
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/users/%d", userID1), strings.NewReader(string(reqBody)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			// perform the request
			rec := httptest.NewRecorder()
			e.PUT("/users/:id", func(c echo.Context) error {
				id, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					return err
				}
				return main.UpdateUser(db, id, &updatedUser)
			}).ServeHTTP(rec, req)

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
			e.PUT("/users/:id", func(c echo.Context) error {
				id, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					return err
				}
				return main.UpdateUser(db, id, &main.User{})
			}).ServeHTTP(rec, req)

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
			testUser := main.User{Username: "testuser", Email: "testuser@example.com", Password: "password123"}
			db.Create(&testUser)

			// get the user's ID from the database
			var userID int
			db.Model(&testUser).Select("id").Scan(&userID)

			// create a test request
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/users/%d", userID), nil)
			rec := httptest.NewRecorder()
			e.DELETE("/users/:id", func(c echo.Context) error {
				id, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					return err
				}
				return main.DeleteUser(db, id)
			}).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusNoContent))

			// verify that the user is soft deleted from the database
			var deletedUser main.User
			db.First(&deletedUser, userID)
			gomega.Expect(deletedUser.DeletedAt).ToNot(gomega.BeNil())
		})

		ginkgo.It("Should return an error for an invalid user ID", func() {
			// make a request with an invalid ID
			req := httptest.NewRequest(http.MethodDelete, "/users/invalid", nil)
			rec := httptest.NewRecorder()
			e.DELETE("/users/:id", func(c echo.Context) error {
				id, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					return err
				}
				return main.DeleteUser(db, id)
			}).ServeHTTP(rec, req)

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
			e.DELETE("/users/:id", func(c echo.Context) error {
				id, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					return err
				}
				return main.DeleteUser(db, id)
			}).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusNotFound))

			// get the error message from the response body
			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			gomega.Expect(response["error"]).To(gomega.Equal("user not found"))
		})
	})

	ginkgo.Context("GetUsers", func() {
		ginkgo.It("Should return a list of all users", func() {
			// create some test users
			testUser1 := main.User{Username: "testuser1", Email: "testuser1@example.com", Password: "password123"}
			db.Create(&testUser1)
			testUser2 := main.User{Username: "testuser2", Email: "testuser2@example.com", Password: "password123"}
			db.Create(&testUser2)

			// create a test request
			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			rec := httptest.NewRecorder()
			e.GET("/users", func(c echo.Context) error {
				page, err := strconv.Atoi(c.QueryParam("page"))
				if err != nil || page < 1 {
					page = 1
				}
				pageSize, err := strconv.Atoi(c.QueryParam("pageSize"))
				if err != nil || pageSize < 1 {
					pageSize = 10
				}
				return main.GetUsers(db, page, pageSize)
			}).ServeHTTP(rec, req)

			// assertions
			gomega.Expect(rec.Code).To(gomega.Equal(http.StatusOK))

			// unmarshal the response body
			var usersResponse []main.User
			json.Unmarshal(rec.Body.Bytes(), &usersResponse)
			gomega.Expect(len(usersResponse)).To(gomega.Equal(2))
		})
	})
})

func TestMainApplication(t *testing.T) {
	ginkgo.RunSpecs(t, "Main Application Suite")
}
