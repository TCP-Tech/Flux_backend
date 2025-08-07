package service

import (
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type UserCredentialClaims struct {
	UserId   uuid.UUID `json:"user_id"`
	UserName string    `json:"user_name"`
	jwt.RegisteredClaims
}
