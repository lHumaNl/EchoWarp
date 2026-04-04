package auth

import "github.com/google/uuid"

// NewSessionID generates a new UUID v4 session identifier.
func NewSessionID() string {
	return uuid.New().String()
}
