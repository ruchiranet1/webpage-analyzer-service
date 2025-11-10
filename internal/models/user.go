package models

import "time"

// User represents a user in the system.
// For this application, it's mostly a placeholder for the auth context.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}
