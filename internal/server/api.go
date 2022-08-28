package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/minio/minio-go/v7"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

type Api struct {
	rootToken string
	db        *DB
	fs        *FileStorage
	cache     *Cache
}

func Run(host string, port string, rootToken string, db *DB, fs *FileStorage, cache *Cache) error {
	a := Api{
		rootToken: rootToken,
		db:        db,
		fs:        fs,
		cache:     cache,
	}
	r := chi.NewRouter()
	a.registerUrls(r)

	return http.ListenAndServe(fmt.Sprintf("%s:%s", host, port), r)
}

func (a *Api) writeError(w http.ResponseWriter, r *http.Request, httpStatus int, msg string) {
	log.Error(msg)

	render.Status(r, httpStatus)
	render.JSON(w, r, Response{
		Error: &ResponseError{
			Code: httpStatus,
			Text: msg,
		},
	})
}

func (a *Api) registerUrls(r *chi.Mux) {
	r.Route("/api", func(r chi.Router) {
		r.Post("/register", a.register)

		r.Route("/auth", func(r chi.Router) {
			r.Post("/", a.auth)
			r.Delete("/{token}", a.authDelete)
		})

		r.Route("/docs", func(r chi.Router) {
			r.Post("/", a.docsPost)
			r.Get("/", a.docsGetAll)
			r.Head("/", a.docsHeadAll)
			r.Get("/{id}", a.docsGetOne)
			r.Head("/{id}", a.docsHeadOne)
			r.Delete("/{id}", a.docsDelete)
		})
	})
}

func (a *Api) identity(requestToken string) (*UserToken, error) {
	if token, ok := a.cache.GetUserToken(requestToken); token.Token == requestToken && ok {
		return &token, nil
	}
	return nil, fmt.Errorf("Token %s doesn't exist. ", requestToken)
}

func (a *Api) register(w http.ResponseWriter, r *http.Request) {
	var input RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		a.writeError(w, r, http.StatusBadRequest, fmt.Sprintf("Failed to decode json body. Error: %s ", err))
		return
	}

	if input.Token != a.rootToken {
		a.writeError(w, r, http.StatusBadRequest, "Incorrect root token")
		return
	}

	if validPassword := IsPasswordValid(input.Password); !validPassword {
		a.writeError(w, r, http.StatusBadRequest, "Password must contain: minimum length 8, digits, at least 2 letters in different cases, at least 1 character (not a letter or a number)")
		return
	}

	passwordHash, err := GeneratePasswordHash(input.Password)
	if err != nil {
		a.writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	err = a.db.CreateNewUser(input.Login, passwordHash)
	if err != nil {
		a.writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, r, Response{
		Response: render.M{
			"login": input.Login,
		},
	})
}

func (a *Api) auth(w http.ResponseWriter, r *http.Request) {
	var input RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		a.writeError(w, r, http.StatusBadRequest, fmt.Sprintf("Failed to decode json body. Error: %s ", err))
		return
	}

	passwordHash, err := GeneratePasswordHash(input.Password)
	if err != nil {
		a.writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	user, err := a.db.GetUser(input.Login)
	if err != nil {
		a.writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	if user == nil {
		a.writeError(w, r, http.StatusNotFound, fmt.Sprintf("User %s not found", input.Login))
		return
	}

	if user.Password != passwordHash {
		a.writeError(w, r, http.StatusForbidden, fmt.Sprintf("Invalid password"))
		return
	}

	userToken, ok := a.cache.GetUserTokenByUserID(user.Id)
	tokenStr := userToken.Token
	if !ok {
		token, err := a.db.CreateToken(user.Id)
		if err != nil {
			a.writeError(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		tokenStr = token.Token
		a.cache.Ch <- SyncTokens
	}

	render.JSON(w, r, Response{
		Response: render.M{
			"token": tokenStr,
		},
	})
}

func (a *Api) authDelete(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	err := a.db.DeleteToken(token)
	if err != nil {
		a.writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	a.cache.Ch <- SyncTokens
	render.JSON(w, r, Response{
		Response: render.M{
			token: true,
		},
	})
}

func (a *Api) docsPost(w http.ResponseWriter, r *http.Request) {
	var input DocPostRequest

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		a.writeError(w, r, http.StatusBadRequest, fmt.Sprintf("Failed to decode json body. Error: %s ", err))
		return
	}

	usertoken, err := a.identity(input.Meta.Token)
	if err != nil {
		a.writeError(w, r, http.StatusForbidden, err.Error())
		return
	}

	// Допустим что все файлы для всех пользователей уникальны
	// Это плохо, но для исправления нужно больше времени
	_, ok := a.cache.getDoc(input.Meta.Name)
	if ok {
		a.writeError(w, r, http.StatusBadRequest, fmt.Sprintf("File %s exists", input.Meta.Name))
		return
	}

	filedata, err := base64.StdEncoding.DecodeString(input.File.Data)
	if err != nil {
		a.writeError(w, r, http.StatusInternalServerError, "Failed to decode string from base64")
		return
	}

	// minio save file
	_, err = a.fs.client.PutObject(
		context.Background(),
		MinioBucketName,
		input.Meta.Name,
		bytes.NewReader(filedata),
		int64(len(filedata)),
		minio.PutObjectOptions{},
	)
	if err != nil {
		a.writeError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to put object to minio. Error: %s ", err))
		return
	}

	// db save file
	err = a.db.CreateDoc(input.Meta.Name, input.Meta.Public, input.Meta.Mime, usertoken.UserID, input.Meta.Grant)
	if err != nil {
		a.writeError(w, r, http.StatusInternalServerError, err.Error())
	}

	// invalidate cache
	a.cache.Ch <- SyncDocs

	render.JSON(w, r, render.M{
		"data": render.M{
			"json": render.M{},
			"file": input.Meta.Name,
		},
	})

}

func (a *Api) docsGetAll(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	_, err := a.identity(token)
	if err != nil {
		a.writeError(w, r, http.StatusForbidden, err.Error())
		return
	}

	limitParam := r.URL.Query().Get("limit")
	var limit int
	if limitParam != "" {
		limit, err = strconv.Atoi(limitParam)
		if err != nil {
			a.writeError(w, r, http.StatusBadRequest, fmt.Sprintf("Limit parameter must be integer. Error: %s", err))
			return
		}
	}

	// TODO Filters
	cacheDocs := a.cache.getDocs()

	users := make(map[int64]string)
	for _, ut := range a.cache.GetUserTokens() {
		users[ut.UserID] = ut.Login
	}

	docs := make([]DocResponse, 0, len(cacheDocs))

	for _, doc := range cacheDocs {
		docResp := DocResponse{
			Id:      doc.Id,
			Name:    doc.Filename,
			Mime:    doc.Mime,
			File:    true,
			Public:  doc.Public,
			Created: doc.Created.Format("2006-01-02 15:04:05"),
			Grant:   nil,
		}

		grant := make([]string, 0, len(doc.GrantIds))
		for _, gid := range doc.GrantIds {
			grant = append(grant, users[gid])
		}

		docResp.Grant = grant
		docs = append(docs, docResp)
		if limit > 0 && len(docs) >= limit {
			break
		}
	}

	render.JSON(w, r, render.M{
		"data": render.M{
			"docs": docs,
		},
	})

}

func (a *Api) docsHeadAll(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	_, err := a.identity(token)
	if err != nil {
		a.writeError(w, r, http.StatusForbidden, err.Error())
		return
	}

	limitParam := r.URL.Query().Get("limit")
	if limitParam != "" {
		_, err = strconv.Atoi(limitParam)
		if err != nil {
			a.writeError(w, r, http.StatusBadRequest, fmt.Sprintf("Limit parameter must be integer. Error: %s", err))
			return
		}
	}
}

func (a *Api) docsGetOne(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	_, err := a.identity(token)
	if err != nil {
		a.writeError(w, r, http.StatusForbidden, err.Error())
		return
	}

	docIdParam := chi.URLParam(r, "id")
	docId, err := strconv.Atoi(docIdParam)
	if err != nil {
		a.writeError(w, r, http.StatusBadRequest, fmt.Sprintf("Doc id parameter must be integer. Error: %s", err))
		return
	}

	doc, ok := a.cache.getDocByID(int64(docId))
	if !ok {
		a.writeError(w, r, http.StatusNotFound, "File doesn't exist")
		return
	}

	object, err := a.fs.client.GetObject(context.Background(), MinioBucketName, doc.Filename, minio.GetObjectOptions{})
	if err != nil {
		a.writeError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to get file from minio. Error: %s ", err))
		return
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(object)
	if err != nil {
		a.writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, r, render.M{
		"data": render.M{
			"name": doc.Filename,
			"mime": doc.Mime,
			"file": base64.StdEncoding.EncodeToString(buf.Bytes()),
		},
	})
}

func (a *Api) docsHeadOne(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	_, err := a.identity(token)
	if err != nil {
		a.writeError(w, r, http.StatusForbidden, err.Error())
		return
	}

	docIdParam := chi.URLParam(r, "id")
	docId, err := strconv.Atoi(docIdParam)
	if err != nil {
		a.writeError(w, r, http.StatusBadRequest, fmt.Sprintf("Doc id parameter must be integer. Error: %s", err))
		return
	}

	_, ok := a.cache.getDocByID(int64(docId))
	if !ok {
		a.writeError(w, r, http.StatusNotFound, "File doesn't exist")
		return
	}
}

func (a *Api) docsDelete(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	_, err := a.identity(token)
	if err != nil {
		a.writeError(w, r, http.StatusForbidden, err.Error())
		return
	}

	docIdParam := chi.URLParam(r, "id")
	docId, err := strconv.Atoi(docIdParam)
	if err != nil {
		a.writeError(w, r, http.StatusBadRequest, fmt.Sprintf("Doc id parameter must be integer. Error: %s", err))
		return
	}

	err = a.db.DeleteDoc(int64(docId))
	if err != nil {
		a.writeError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	a.cache.Ch <- SyncDocs

	render.JSON(w, r, render.M{
		"response": render.M{
			docIdParam: true,
		},
	})
}
