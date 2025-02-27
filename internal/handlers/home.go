package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"project/config"
	"project/internal/models"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func HomePage(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")

	var user models.User
	config.DB.QueryRow("SELECT id, phone, email FROM users WHERE id = $1", userID).
		Scan(&user.ID, &user.Phone, &user.Email)

	regionFilter := c.Query("region")
	regionsRows, err := config.DB.Query("SELECT region FROM quotas ORDER BY region ASC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки регионов"})
		return
	}
	defer regionsRows.Close()

	var regions []string
	for regionsRows.Next() {
		var reg string
		if err := regionsRows.Scan(&reg); err != nil {
			continue
		}
		regions = append(regions, reg)
	}

	var query string
	var rows *sql.Rows
	if regionFilter != "" {
		query = `
			SELECT a.id, a.full_name, a.birth_date, a.region, a.phone, a.status,
			b.name AS benefit, COALESCE(b.priority, 1) AS priority,
			queue_number, a.id_card, COALESCE(a.benefit_doc, ''), a.promoted_at
			FROM applications a
			LEFT JOIN benefits b ON a.benefit = b.name
			WHERE a.deleted_at IS NULL AND a.region = $1
			ORDER BY queue_number ASC
		`
		rows, err = config.DB.Query(query, regionFilter)
	} else {
		query := `
			SELECT a.id, a.full_name, a.birth_date, a.region, a.phone, a.status,
			b.name AS benefit, COALESCE(b.priority, 1) AS priority,
			queue_number, a.id_card, COALESCE(a.benefit_doc, ''), a.promoted_at
			FROM applications a
			LEFT JOIN benefits b ON a.benefit = b.name
			WHERE a.deleted_at IS NULL 
			ORDER BY queue_number ASC
		`
		rows, err = config.DB.Query(query)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения заявок"})
		return
	}
	defer rows.Close()

	var applications []models.Application
	for rows.Next() {
		var app models.Application
		var benefitPriority int
		var benefitDoc sql.NullString

		err := rows.Scan(&app.ID, &app.FullName, &app.BirthDate, &app.Region, &app.Phone, &app.Status, &app.Benefit, &benefitPriority, &app.QueueNumber, &app.IDCard, &benefitDoc, &app.PromotedAt)
		if err != nil {
			fmt.Println("Ошибка обработки заявки:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обработки заявок"})
			return
		}

		if benefitDoc.Valid {
			app.BenefitDoc = &benefitDoc.String
		} else {
			app.BenefitDoc = nil
		}

		if !strings.HasPrefix(app.IDCard, "uploads/") {
			app.IDCard = "uploads/" + app.IDCard
		}
		if app.BenefitDoc != nil && !strings.HasPrefix(*app.BenefitDoc, "uploads/") {
			*app.BenefitDoc = "uploads/" + *app.BenefitDoc
		}

		applications = append(applications, app)
	}

	c.HTML(http.StatusOK, "home.html", gin.H{
		"user":         user,
		"applications": applications,
		"regions":      regions,
		"selected":     regionFilter,
	})
}

// Получение данных пользователя
func GetUser(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")

	var user models.User
	err := config.DB.QueryRow("SELECT id, phone, email FROM users WHERE id = $1", userID).
		Scan(&user.ID, &user.Phone, &user.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Пользователь не найден"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// Получение списка регионов
func GetRegions(c *gin.Context) {
	rows, err := config.DB.Query("SELECT region FROM quotas ORDER BY region ASC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки регионов"})
		return
	}
	defer rows.Close()

	var regions []string
	for rows.Next() {
		var region string
		rows.Scan(&region)
		regions = append(regions, region)
	}

	c.JSON(http.StatusOK, regions)
}

// Получение заявок
func GetApplications(c *gin.Context) {
	regionFilter := c.Query("region")
	var rows *sql.Rows
	var err error

	query := `
		SELECT id, full_name, birth_date, region, phone, status, benefit, queue_number, id_card, benefit_doc, promoted_at
		FROM applications
		WHERE deleted_at IS NULL
	`
	if regionFilter != "" {
		query += " AND region = $1 ORDER BY queue_number ASC"
		rows, err = config.DB.Query(query, regionFilter)
	} else {
		query += " ORDER BY queue_number ASC"
		rows, err = config.DB.Query(query)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения заявок"})
		return
	}
	defer rows.Close()

	var applications []models.Application
	for rows.Next() {
		var app models.Application
		var benefitDoc sql.NullString
		err := rows.Scan(&app.ID, &app.FullName, &app.BirthDate, &app.Region, &app.Phone, &app.Status, &app.Benefit, &app.QueueNumber, &app.IDCard, &benefitDoc, &app.PromotedAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обработки заявок"})
			return
		}
		if benefitDoc.Valid {
			app.BenefitDoc = &benefitDoc.String
		}
		applications = append(applications, app)
	}

	c.JSON(http.StatusOK, applications)
}
