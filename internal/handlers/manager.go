package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"project/config"
	"project/internal/models"
	"project/internal/services"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func ManagerPage(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	userRole := session.Get("user_role")

	if userID == nil || userRole != "manager" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещен"})
		return
	}

	var region string
	err := config.DB.QueryRow("SELECT region FROM users WHERE id = $1", userID).Scan(&region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения региона менеджера"})
		return
	}

	query := `
		SELECT a.id, a.full_name, a.birth_date, a.region, a.phone, a.status,
		b.name AS benefit, COALESCE(b.priority, 1) AS priority,
		queue_number, a.id_card, COALESCE(a.benefit_doc, ''), a.promoted_at
		FROM applications a
		LEFT JOIN benefits b ON a.benefit = b.name
		WHERE a.deleted_at IS NULL AND a.region = $1
		ORDER BY queue_number ASC
	`

	rows, err := config.DB.Query(query, region)
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
		var promotedAt sql.NullTime

		err := rows.Scan(&app.ID, &app.FullName, &app.BirthDate, &app.Region, &app.Phone, &app.Status, &app.Benefit, &benefitPriority, &app.QueueNumber, &app.IDCard, &benefitDoc, &promotedAt)
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

		if promotedAt.Valid {
			app.PromotedAt = promotedAt
		} else {
			app.PromotedAt = sql.NullTime{}
		}

		applications = append(applications, app)
	}

	var maxPlaces, currentCount int
	err = config.DB.QueryRow("SELECT max_places FROM quotas WHERE region = $1", region).Scan(&maxPlaces)
	if err == sql.ErrNoRows {
		maxPlaces = 0
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки квоты"})
		return
	}

	err = config.DB.QueryRow("SELECT COUNT(*) FROM applications WHERE region = $1 AND deleted_at IS NULL", region).Scan(&currentCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка подсчета заявок"})
		return
	}

	c.HTML(http.StatusOK, "manager.html", gin.H{
		"applications": applications,
		"region":       region,
		"MaxPlaces":    maxPlaces,
		"CurrentCount": currentCount,
	})
}

func ApproveApplication(c *gin.Context) {
	appID := c.PostForm("application_id")

	var phone string
	err := config.DB.QueryRow("SELECT phone FROM applications WHERE id = $1", appID).Scan(&phone)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Заявка не найдена"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения данных"})
		return
	}

	_, err = config.DB.Exec("UPDATE applications SET status = 'approved' WHERE id = $1", appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка подтверждения заявки"})
		return
	}

	message := "Ваша заявка на путевку одобрена! Ожидайте."
	services.SendSMS(phone, message)

	c.Redirect(http.StatusFound, "/manager")
}

func RejectApplication(c *gin.Context) {
	applicationID := c.PostForm("application_id")

	_, err := config.DB.Exec("UPDATE applications SET deleted_at = NOW(), status = 'rejected' WHERE id = $1", applicationID)
	if err != nil {
		fmt.Println("Ошибка при отклонении заявки:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при отклонении заявки"})
		return
	}

	c.Redirect(http.StatusFound, "/manager")
}

func PromoteApplication(c *gin.Context) {
	applicationID := c.PostForm("application_id")

	_, err := config.DB.Exec(`
        UPDATE applications 
        SET status = 'promoted', promoted_at = NOW() 
        WHERE id = $1 AND status = 'approved'
    `, applicationID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка продвижения заявки"})
		return
	}

	c.Redirect(http.StatusFound, "/manager")
}
