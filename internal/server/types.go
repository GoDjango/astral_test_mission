package server

import (
	"github.com/go-chi/render"
)

const (
	MinioBucketName = "astral"
)

type ResponseError struct {
	Code int    `json:"code"`
	Text string `json:"text"`
}

type Response struct {
	Error    *ResponseError `json:"error,omitempty"`
	Response render.M       `json:"response,omitempty"`
	Data     render.M       `json:"data,omitempty"`
}

type RegisterRequest struct {
	Token    string `json:"token,omitempty"`
	Login    string `json:"login"`
	Password string `json:"pswd"`
}

type DocPostRequest struct {
	Meta struct {
		Name   string   `json:"name"`
		File   bool     `json:"file"`
		Public bool     `json:"public"`
		Token  string   `json:"token"`
		Mime   string   `json:"mime"`
		Grant  []string `json:"grant"`
	} `json:"meta"`
	Json interface{} `json:"json,omitempty"`
	File struct {
		Data string `json:"data"`
	} `json:"file"`
}

type DocResponse struct {
	Id      int64    `json:"id"`
	Name    string   `json:"name"`
	Mime    string   `json:"mime"`
	File    bool     `json:"file"`
	Public  bool     `json:"public"`
	Created string   `json:"created"`
	Grant   []string `json:"grant"`
}
