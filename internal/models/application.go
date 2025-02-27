package models

import "database/sql"

type Application struct {
	ID          int          `json:"id"`
	UserID      int          `json:"user_id"`
	FullName    string       `json:"full_name"`
	BirthDate   string       `json:"birth_date"`
	Region      string       `json:"region"`
	Phone       string       `json:"phone"`
	IIN         string       `json:"iin"`
	IDCard      string       `json:"id_card"`
	Benefit     string       `json:"benefit"`
	PromotedAt  sql.NullTime `json:"promoted_at"`
	QueueNumber int          `json:"queue_number"`
	BenefitDoc  *string      `json:"benefit_doc"`
	Status      string       `json:"status"`
}
