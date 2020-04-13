package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/ahussein/session-based-signin-golang/internal/platform/database"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %q", err)
	}
	db, err := database.DB(database.ConnectionParams{
		Driver:               "postgres",
		Username:             "auth_user",
		Password:             "auth_pass",
		Host:                 "postgres",
		Port:                 5432,
		Database:             "auth",
		MaxDBConnections:     5,
		MaxDBIdleConnections: 5,
	})
	if err != nil {
		logger.Fatal("failed to connect to db", zap.Error(err))
	}
	cache, err := initCache(logger)
	if err != nil {
		logger.Fatal("failed to initialize cache", zap.Error(err))
	}
	http.HandleFunc("/signup", SingupHanlder(db, cache, logger))
	http.HandleFunc("/signin", SigninHandler(db, cache, logger))
	http.HandleFunc("/welcome", WelcomeHandler(db, cache, logger))
	http.HandleFunc("/refresh", RefreshTokenHandler(db, cache, logger))
	log.Fatal(http.ListenAndServe(":80", nil))
}

func initCache(logger *zap.Logger) (redis.Conn, error) {
	conn, err := redis.DialURL("redis://redis")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to radis cache")
	}
	return conn, nil
}

// Credentials ...
type Credentials struct {
	Password string `json:"password"`
	Username string `json:"username"`
}

// SingupHanlder handles singup requests
func SingupHanlder(db *sql.DB, cache redis.Conn, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		creds := &Credentials{}
		err := json.NewDecoder(r.Body).Decode(creds)
		if err != nil {
			logger.Error("failed to parse request body", zap.Error(err))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(creds.Password), 8)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "failed to generate hashed password")
			return
		}
		if _, err := db.Query("insert into users values ($1, $2)", creds.Username, string(hashedPassword)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "failed to add user to db")
			return
		}
	}
}

// SigninHandler ...
func SigninHandler(db *sql.DB, cache redis.Conn, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		creds := &Credentials{}
		err := json.NewDecoder(r.Body).Decode(creds)
		if err != nil {
			logger.Error("failed to parse request body", zap.Error(err))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		result := db.QueryRow("select password from users where username=$1", creds.Username)
		storedCreds := &Credentials{}
		err = result.Scan(&storedCreds.Password)
		if err != nil {
			if err == sql.ErrNoRows {
				logger.Info("no result found for user", zap.String("username", creds.Username))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			logger.Error("failed to retrieve user data", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err = bcrypt.CompareHashAndPassword([]byte(storedCreds.Password), []byte(creds.Password)); err != nil {
			logger.Error("failed to compare passwords", zap.Error(err))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Create a random session cookie
		sessionUUID, err := uuid.NewUUID()
		if err != nil {
			logger.Error("failed to create session UUID", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		sessionCookie := sessionUUID.String()
		_, err = cache.Do("SETEX", sessionCookie, "120", creds.Username)
		if err != nil {
			logger.Error("failed to set save session cookie", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Set the session cookie in the client
		http.SetCookie(w, &http.Cookie{
			Name:    "session_token",
			Value:   sessionCookie,
			Expires: time.Now().Add(120 * time.Second),
		})
	}
}

// WelcomeHandler ...
func WelcomeHandler(db *sql.DB, cache redis.Conn, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session_token")
		if err != nil {
			if err == http.ErrNoCookie {
				logger.Error("no session token found in requst cookie", zap.Error(err))
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			logger.Error("failed to fetch session token from request cookie", zap.Error(err))
			return
		}
		sessionToken := c.Value
		res, err := cache.Do("GET", sessionToken)
		if err != nil {
			logger.Error("failed to fetch session token from cache", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if res == nil {
			logger.Error("session token not found in cache", zap.Error(err))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(fmt.Sprintf("Weolcome %s!", res)))
	}
}

// RefreshTokenHandler ...
func RefreshTokenHandler(db *sql.DB, cache redis.Conn, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session_token")
		if err != nil {
			if err == http.ErrNoCookie {
				logger.Error("no session token found in requst cookie", zap.Error(err))
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			logger.Error("failed to fetch session token from request cookie", zap.Error(err))
			return
		}
		sessionToken := c.Value
		res, err := cache.Do("GET", sessionToken)
		if err != nil {
			logger.Error("failed to fetch session token from cache", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if res == nil {
			logger.Error("session token not found in cache", zap.Error(err))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		newSessionUUID, err := uuid.NewUUID()
		if err != nil {
			logger.Error("failed to create session UUID", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		newSessionCookie := newSessionUUID.String()
		_, err = cache.Do("SETEX", newSessionCookie, "120", res)
		if err != nil {
			logger.Error("failed to set save session cookie", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Delete the older session token
		_, err = cache.Do("DEL", sessionToken)
		if err != nil {
			logger.Error("failed to delete existing session token from cache", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Set the session cookie in the client
		http.SetCookie(w, &http.Cookie{
			Name:    "session_token",
			Value:   newSessionCookie,
			Expires: time.Now().Add(120 * time.Second),
		})
	}
}
