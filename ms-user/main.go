package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var (
	dbHost      = os.Getenv("PHANTOM_DB_HOST")
	dbPort      = os.Getenv("PHANTON_DB_PORT")
	dbUser      = os.Getenv("PHANTOM_DB_USER")
	dbPasssword = os.Getenv("PHANTOM_DB_PASSWORD")
	dbName      = os.Getenv("PHANTOM_DB_NAME")
	db          *sql.DB
)

type (
	Response struct {
		Code int         `json:"message"`
		Info string      `json:"info"`
		Data interface{} `json:"data"`
	}

	UserRequest struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}

	User struct {
		Id       string `json:"id"`
		Name     string `json:"name"`
		Username string `json:"username"`
		Password string `json:"-"`
	}

	response func(w http.ResponseWriter, code int, info string, data interface{}) map[string]interface{}
)

func (r response) singleResult(w http.ResponseWriter, code int, info string, data interface{}) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": code,
		"info": info,
		"data": map[string]interface{}{
			"doc": data,
		},
	})
}

func (r response) multipleResult(w http.ResponseWriter, code int, info string, data interface{}) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": code,
		"info": info,
		"data": map[string]interface{}{
			"doc": data,
		},
	})
}

func (r response) errResult(w http.ResponseWriter, code int, info string, data interface{}) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": code,
		"info": info,
		"data": map[string]interface{}{
			"doc": data,
		},
	})
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	var (
		req  = UserRequest{}
		resp response
	)

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		resp.errResult(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	password, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		resp.errResult(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	id := uuid.NewString()

	_, err = db.ExecContext(r.Context(), `
		INSERT INTO users(id, name, username, password) VALUES ($1, $2, $3, $4)
	`, id, req.Name, req.Username, password)
	if err != nil {
		resp.errResult(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	resp.singleResult(w, http.StatusOK, "success", User{
		Id:       id,
		Name:     req.Name,
		Username: req.Username,
	})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var (
		req  = UserRequest{}
		resp response
	)

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		resp.errResult(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	user := User{}

	err = db.QueryRowContext(
		r.Context(),
		`select id, name, username, password from users where username = $1`,
		req.Username,
	).Scan(
		&user.Id, &user.Name, &user.Username, &user.Password,
	)
	if errors.Is(err, sql.ErrNoRows) {
		resp.errResult(w, http.StatusNotFound, "user not found", nil)
		return
	}

	if err != nil {
		resp.errResult(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		resp.errResult(w, http.StatusForbidden, "wrong username/password", nil)
		return
	}

	if err != nil {
		resp.errResult(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	jwt, err := generateJwt(user.Id)
	if err != nil {
		resp.errResult(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	opaque, err := generateOpaque()
	if err != nil {
		resp.errResult(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	_, err = db.ExecContext(
		r.Context(),
		`insert into opaque_jwt_token (opaque,jwt) values($1, $2)`,
		opaque, jwt,
	)
	if err != nil {
		resp.errResult(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	resp.singleResult(
		w,
		http.StatusOK,
		"success",
		map[string]interface{}{
			"opaqueToken": opaque,
		},
	)
}

func getJWTHandler(w http.ResponseWriter, r *http.Request) {
	var (
		resp        response
		opaqueToken = r.Header.Get("x-opaque-token")
	)

	if opaqueToken == "" {
		resp.errResult(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	var tokenPair struct {
		JWT, Opaque string
	}

	err := db.QueryRowContext(
		r.Context(),
		`select jwt, opaque from opaque_jwt_token where opaque = $1`,
		opaqueToken,
	).Scan(&tokenPair.JWT, &tokenPair.Opaque)
	if errors.Is(err, sql.ErrNoRows) {
		resp.errResult(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	if err != nil {
		resp.errResult(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	w.Header().Set("access_token", tokenPair.JWT)
	resp.singleResult(w, http.StatusOK, "success", nil)
}

func main() {
	dbConn, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPasssword, dbHost, dbPort, dbName))
	if err != nil {
		log.Fatalf("failed open db connection: %v", err)
	}

	db = dbConn

	migrateUp(db)

	r := chi.NewRouter()

	r.Use(middleware.Logger)

	r.Post("/users/action/register", registerHandler)
	r.Post("/users/action/login", loginHandler)
	r.Get("/users/auth", getJWTHandler)

	http.ListenAndServe(":8000", r)
}
