package service

import "github.com/golang-jwt/jwt/v4"

type UserCredentialClaims struct {
	UserName string `json:"user_name"`
	RollNo   string `json:"roll_no"`
	jwt.RegisteredClaims
}
