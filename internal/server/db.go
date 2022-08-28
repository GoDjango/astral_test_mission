package server

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"time"
)

type User struct {
	Id       int64  `db:"id"`
	Login    string `db:"login"`
	Password string `db:"password"`
}

type Doc struct {
	Id       int64     `db:"id"`
	Filename string    `db:"filename"`
	Public   bool      `db:"public"`
	Mime     string    `db:"mime"`
	OwnerId  int64     `db:"owner_id"`
	Created  time.Time `db:"created"`
	GrantIds []int64
	Grant    []string
}

type UsersDocsGrant struct {
	UserId int64 `db:"user_id"`
	DocId  int64 `db:"doc_id"`
}

type Token struct {
	UserId int64  `db:"user_id"`
	Token  string `db:"token"`
}

type UserToken struct {
	UserID   int64  `db:"id"`
	Login    string `db:"login"`
	Password string `db:"password"`
	Token    string `db:"token"`
}

type DB struct {
	db *sqlx.DB
}

func NewDB(url string) *DB {
	db, err := sqlx.Open("postgres", url)
	if err != nil {
		log.Fatalf("Failed to connect db. Eror: %s", err)
	}
	return &DB{
		db: db,
	}
}

func (d *DB) Close() {
	if err := d.db.Close(); err != nil {
		log.Fatalf("Failed to close db connection. Error: %s", err)
	}
}

func (d *DB) CreateNewUser(login string, passwordHash string) error {
	tx, err := d.db.Beginx()
	if err != nil {
		return fmt.Errorf("Failed to create user transaction. Error: %s ", err)
	}

	_, err = tx.Exec("INSERT INTO public.users (login, password) VALUES ($1, $2)", login, passwordHash)
	if err != nil {
		return fmt.Errorf("Failed to create new user. Error: %s ", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Failed to commit create user. Error: %s ", err)
	}

	return nil
}

func (d *DB) GetUser(login string) (*User, error) {
	var users []User
	err := d.db.Select(&users, "SELECT id, login, password FROM public.users WHERE login = $1 LIMIT 1", login)
	if err != nil {
		return nil, fmt.Errorf("Failed to get user from db. Error: %s ", err.Error())
	}
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

func (d *DB) GetUserByIds(ids []int64) ([]User, error) {
	var users []User
	err := d.db.Select(&users, "SELECT id, login, password FROM public.users WHERE id = ANY($1)", pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("Failed to get users from db. Error: %s ", err.Error())
	}

	return users, nil
}

func (d *DB) GetTokens() (map[string]UserToken, error) {
	var userTokens []UserToken
	err := d.db.Select(&userTokens, "SELECT u.id, u.login, u.password, t.token FROM public.users u JOIN public.tokens t ON (u.id = t.user_id)")
	if err != nil {
		return nil, fmt.Errorf("Failed to get tokens. Error: %s ", err)
	}

	userTokensMap := make(map[string]UserToken)
	for _, ut := range userTokens {
		userTokensMap[ut.Token] = ut
	}

	return userTokensMap, nil
}

func (d *DB) CreateToken(userId int64) (*Token, error) {
	token, err := GenerateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("Failed to generate token. Error: %s", err)
	}

	tx, err := d.db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("Failed to create token transaction. Error: %s ", err)
	}

	_, err = tx.Exec("INSERT INTO public.tokens (user_id, token) VALUES ($1, $2)", userId, token)
	if err != nil {
		return nil, fmt.Errorf("Failed to create new token. Error: %s ", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("Failed to commit created token. Error: %s ", err)
	}

	return &Token{
		UserId: userId,
		Token:  token,
	}, nil
}

func (d *DB) GetToken(userId int64) (*Token, error) {
	var tokens []Token

	err := d.db.Select(tokens, "SELECT user_id, token FROM public.tokens WHERE user_id = $1 LIMIT 1", userId)
	if err != nil {
		return nil, fmt.Errorf("Failed to get token by user id. Error: %s ", err)
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	return &tokens[0], nil
}

func (d *DB) DeleteToken(token string) error {
	tx, err := d.db.Beginx()
	if err != nil {
		return fmt.Errorf("Failed to delete token transaction. Error: %s ", err)
	}

	_, err = tx.Exec("DELETE FROM public.tokens WHERE token = $1", token)
	if err != nil {
		return fmt.Errorf("Failed to delete token. Error: %s ", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Failed to commit token delete. Error: %s ", err)
	}

	return nil

}

func (d *DB) GetDocs() (map[string]Doc, error) {
	var docs []Doc

	err := d.db.Select(&docs, "SELECT id, filename, public, mime, owner_id, created FROM public.docs")
	if err != nil {
		return nil, fmt.Errorf("Failed to get docs from db. Error: %s ", err)
	}

	var userDocGrants []UsersDocsGrant
	err = d.db.Select(&userDocGrants, "SELECT user_id, doc_id FROM public.users_docs_grant")
	if err != nil {
		return nil, fmt.Errorf("Failed to get user doc grants from db. Error: %s ", err)
	}

	docsMap := make(map[string]Doc)
	for _, doc := range docs {
		grantUserIds := make([]int64, 0)
		for _, udg := range userDocGrants {
			if udg.DocId == doc.Id {
				grantUserIds = append(grantUserIds, udg.UserId)
			}
		}
		doc.GrantIds = grantUserIds
		docsMap[doc.Filename] = doc
	}
	return docsMap, nil
}

func (d *DB) CreateDoc(filename string, public bool, mime string, owner int64, grant []string) error {
	tx, err := d.db.Beginx()
	if err != nil {
		return fmt.Errorf("Failed to create doc transaction. Error: %s ", err)
	}

	row := tx.QueryRowx("INSERT INTO public.docs (filename, public, mime, owner_id, created) VALUES ($1, $2, $3, $4, $5) RETURNING id", filename, public, mime, owner, time.Now().UTC())
	if row.Err() != nil {
		return fmt.Errorf("Failed to create new doc. Error: %s ", err)
	}
	var docId int64
	err = row.Scan(&docId)
	if err != nil {
		return fmt.Errorf("Failed to scan doc id from row. Error: %s ", err)
	}

	if !public && len(grant) != 0 {
		var userIds []int64
		err := tx.Select(&userIds, "SELECT id FROM public.users WHERE login = ANY($1) AND id != $2", pq.Array(grant), owner)
		if err != nil {
			return fmt.Errorf("Failed to get users by grant string. Error: %s ", err)
		}

		for _, uid := range userIds {
			_, err := tx.Exec("INSERT INTO public.users_docs_grant (doc_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", docId, uid)
			if err != nil {
				return fmt.Errorf("Failed to insert user doc grant. Error: %s ", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Failed to commit created doc. Error: %s ", err)
	}
	return nil
}

func (d *DB) DeleteDoc(id int64) error {
	tx, err := d.db.Beginx()
	if err != nil {
		return fmt.Errorf("Failed to doc token transaction. Error: %s ", err)
	}

	_, err = tx.Exec("DELETE FROM public.docs WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("Failed to delete doc. Error: %s ", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Failed to commit doc delete. Error: %s ", err)
	}

	return nil
}
