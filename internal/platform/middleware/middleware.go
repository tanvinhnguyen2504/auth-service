package middleware

import "fmt"

type Middleware interface{}

type middleware struct{}

func New() Middleware {
	return middleware{}
}

func (m *middleware) ValidateToken() {
	fmt.Println("ValidateToken...")
}
