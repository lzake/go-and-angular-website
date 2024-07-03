package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	echoSwagger "github.com/swaggo/echo-swagger"
	"golang.org/x/time/rate"
)

var (
	db  *sql.DB
	e   *echo.Echo
	cfg *Config
)

var _ = ginkgo.BeforeSuite(func() {
	err := godotenv.Load(".env.test")
	if err != nil {
		log.Fatal("Error loading .env.test file:", err)
	}

	userCache = cache.New(5*time.Minute, 10*time.Minute)

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		getEnvAsInt("DB_PORT", 5432),
		os.Getenv("DB_SSLMODE"),
	)
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		panic("Failed to connect to the test database!")
	}
	err = db.Ping()
	if err != nil {
		panic("Failed to ping the test database!")
	}

	cfg, err = readConfig("config.test.json")
	if err != nil {
		panic("Error reading test configuration file: " + err.Error())
	}

	e = echo.New()
	e.Validator = &CustomValidator{validator: validator.New()}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:4200"},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(cfg.App.RateLimit))))
	e.Logger.SetLevel(0)
	e.GET("/swagger/*", echoSwagger.WrapHandler)
})

var _ = ginkgo.AfterSuite(func() {
	db.Close()
})

var _ = ginkgo.BeforeEach(func() {
	db.Exec("DELETE FROM users")
})

func TestMainApplication(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Main Application Suite")
}

var _ = ginkgo.Describe("Main Application", func() {
	ginkgo.Context("CreateUser", func() {
		ginkgo.It("Should create a new user successfully", func() {
			testUser := User{Username: "testuser", Email: "testuser@example.com", Password: "password123", ProfilePictureURL: "https://example.com/profile.jpg", Bio: "Test User Bio"}
			reqBody, _ := json.Marshal(testUser)

			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(reqBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users")

			err := createUser(db, &testUser)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusCreated))
		})

		ginkgo.It("Should return an error for invalid user data", func() {
			testUser := User{Username: "", Email: "invalid_email", Password: "password123"}
			reqBody, _ := json.Marshal(testUser)

			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(reqBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users")

			err := createUser(db, &testUser)
			gomega.Expect(err).Should(gomega.Not(gomega.BeNil()))
			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusBadRequest))
		})

		ginkgo.It("Should return an error for duplicate username", func() {
			existingUser := User{Username: "duplicateuser", Email: "duplicateuser@example.com", Password: "password123"}
			err := createUser(db, &existingUser)
			gomega.Expect(err).Should(gomega.BeNil())

			testUser := User{Username: "duplicateuser", Email: "another@example.com", Password: "password123"}
			reqBody, _ := json.Marshal(testUser)

			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(reqBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users")

			err = createUser(db, &testUser)
			gomega.Expect(err).Should(gomega.Not(gomega.BeNil()))
			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusBadRequest))
		})
	})

	ginkgo.Context("GetUserByID", func() {
		ginkgo.It("Should return a user by ID successfully", func() {
			testUser := User{Username: "testuser", Email: "testuser@example.com", Password: "password123"}
			err := db.QueryRow("INSERT INTO users (username, email, password) VALUES ($1, $2, $3) RETURNING id", testUser.Username, testUser.Email, testUser.Password).Scan(&testUser.ID)
			gomega.Expect(err).Should(gomega.BeNil())

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/users/%d", testUser.ID), nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users/:id")
			c.SetParamNames("id")
			c.SetParamValues(strconv.Itoa(testUser.ID))

			user, err := getUserByID(db, testUser.ID)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusOK))
			gomega.Expect(user.Username).Should(gomega.Equal(testUser.Username))
			gomega.Expect(user.Email).Should(gomega.Equal(testUser.Email))
		})

		ginkgo.It("Should return an error for an invalid user ID", func() {
			req := httptest.NewRequest(http.MethodGet, "/users/invalid", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users/:id")
			c.SetParamNames("id")
			c.SetParamValues("invalid")

			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusBadRequest))
		})

		ginkgo.It("Should return a 404 error for a non-existent user ID", func() {
			req := httptest.NewRequest(http.MethodGet, "/users/999", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users/:id")
			c.SetParamNames("id")
			c.SetParamValues("999")

			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusNotFound))
		})
	})

	ginkgo.Context("UpdateUser", func() {
		ginkgo.It("Should update a user by ID successfully", func() {
			testUser := User{Username: "testuser", Email: "testuser@example.com", Password: "password123"}
			err := db.QueryRow("INSERT INTO users (username, email, password) VALUES ($1, $2, $3) RETURNING id", testUser.Username, testUser.Email, testUser.Password).Scan(&testUser.ID)
			gomega.Expect(err).Should(gomega.BeNil())

			updatedUser := User{Username: "updateduser", Email: "updateduser@example.com", Password: "newpassword123", ProfilePictureURL: "https://example.com/newprofile.jpg", Bio: "Updated User Bio"}
			reqBody, _ := json.Marshal(updatedUser)

			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/users/%d", testUser.ID), bytes.NewReader(reqBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users/:id")
			c.SetParamNames("id")
			c.SetParamValues(strconv.Itoa(testUser.ID))

			err = updateUser(db, testUser.ID, &updatedUser)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusOK))
		})

		ginkgo.It("Should return an error for invalid user ID", func() {
			req := httptest.NewRequest(http.MethodPut, "/users/invalid", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users/:id")
			c.SetParamNames("id")
			c.SetParamValues("invalid")

			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusBadRequest))
		})

		ginkgo.It("Should return an error for invalid user data", func() {
			testUser := User{Username: "testuser", Email: "testuser@example.com", Password: "password123"}
			err := db.QueryRow("INSERT INTO users (username, email, password) VALUES ($1, $2, $3) RETURNING id", testUser.Username, testUser.Email, testUser.Password).Scan(&testUser.ID)
			gomega.Expect(err).Should(gomega.BeNil())

			updatedUser := User{Username: "", Email: "invalid_email", Password: "newpassword123"}
			reqBody, _ := json.Marshal(updatedUser)

			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/users/%d", testUser.ID), bytes.NewReader(reqBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users/:id")
			c.SetParamNames("id")
			c.SetParamValues(strconv.Itoa(testUser.ID))

			err = updateUser(db, testUser.ID, &updatedUser)
			gomega.Expect(err).Should(gomega.Not(gomega.BeNil()))
			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusBadRequest))
		})

		ginkgo.It("Should return an error for duplicate username or email", func() {
			testUser1 := User{Username: "testuser1", Email: "testuser1@example.com", Password: "password123"}
			err := db.QueryRow("INSERT INTO users (username, email, password) VALUES ($1, $2, $3) RETURNING id", testUser1.Username, testUser1.Email, testUser1.Password).Scan(&testUser1.ID)
			gomega.Expect(err).Should(gomega.BeNil())

			testUser2 := User{Username: "testuser2", Email: "testuser2@example.com", Password: "password123"}
			err = db.QueryRow("INSERT INTO users (username, email, password) VALUES ($1, $2, $3) RETURNING id", testUser2.Username, testUser2.Email, testUser2.Password).Scan(&testUser2.ID)
			gomega.Expect(err).Should(gomega.BeNil())

			updatedUser := User{Username: "testuser2", Email: "updated@example.com", Password: "newpassword123"}
			reqBody, _ := json.Marshal(updatedUser)

			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/users/%d", testUser1.ID), bytes.NewReader(reqBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users/:id")
			c.SetParamNames("id")
			c.SetParamValues(strconv.Itoa(testUser1.ID))

			err = updateUser(db, testUser1.ID, &updatedUser)
			gomega.Expect(err).Should(gomega.Not(gomega.BeNil()))
			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusBadRequest))
		})

		ginkgo.It("Should return a 404 error for a non-existent user ID", func() {
			req := httptest.NewRequest(http.MethodPut, "/users/999", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users/:id")
			c.SetParamNames("id")
			c.SetParamValues("999")

			err := updateUser(db, 999, &User{})
			gomega.Expect(err).Should(gomega.Not(gomega.BeNil()))
			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusNotFound))
		})
	})

	ginkgo.Context("DeleteUser", func() {
		ginkgo.It("Should delete a user by ID successfully", func() {
			testUser := User{Username: "testuser", Email: "testuser@example.com", Password: "password123"}
			err := db.QueryRow("INSERT INTO users (username, email, password) VALUES ($1, $2, $3) RETURNING id", testUser.Username, testUser.Email, testUser.Password).Scan(&testUser.ID)
			gomega.Expect(err).Should(gomega.BeNil())

			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/users/%d", testUser.ID), nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users/:id")
			c.SetParamNames("id")
			c.SetParamValues(strconv.Itoa(testUser.ID))

			err = deleteUser(db, testUser.ID)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusNoContent))
		})

		ginkgo.It("Should return an error for an invalid user ID", func() {
			req := httptest.NewRequest(http.MethodDelete, "/users/invalid", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users/:id")
			c.SetParamNames("id")
			c.SetParamValues("invalid")

			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusBadRequest))
		})

		ginkgo.It("Should return a 404 error for a non-existent user ID", func() {
			req := httptest.NewRequest(http.MethodDelete, "/users/999", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users/:id")
			c.SetParamNames("id")
			c.SetParamValues("999")

			err := deleteUser(db, 999)
			gomega.Expect(err).Should(gomega.Not(gomega.BeNil()))
			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusNotFound))
		})
	})

	ginkgo.Context("GetUsers", func() {
		ginkgo.It("Should return a list of all users", func() {
			testUser1 := User{Username: "testuser1", Email: "testuser1@example.com", Password: "password123"}
			_, err := db.Exec("INSERT INTO users (username, email, password) VALUES ($1, $2, $3)", testUser1.Username, testUser1.Email, testUser1.Password)
			gomega.Expect(err).Should(gomega.BeNil())

			testUser2 := User{Username: "testuser2", Email: "testuser2@example.com", Password: "password123"}
			_, err = db.Exec("INSERT INTO users (username, email, password) VALUES ($1, $2, $3)", testUser2.Username, testUser2.Email, testUser2.Password)
			gomega.Expect(err).Should(gomega.BeNil())

			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/users")

			page := 1
			pageSize := 10

			users, err := getUsers(db, page, pageSize)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(rec.Code).Should(gomega.Equal(http.StatusOK))
			gomega.Expect(len(users)).Should(gomega.Equal(2))
		})
	})
})
