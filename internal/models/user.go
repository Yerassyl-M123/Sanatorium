package models

type User struct {
	ID       int     `json:"id"`
	Phone    string  `json:"phone"`
	Email    string  `json:"email"`
	Password string  `json:"password"`
	Role     string  `json:"role"`
	Region   *string `json:"region"`
}
