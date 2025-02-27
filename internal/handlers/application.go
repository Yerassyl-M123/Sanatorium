package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"
	"project/config"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func isValidFileType(filename string) bool {
	allowedExtensions := map[string]bool{
		".pdf":  true,
		".jpg":  true,
		".jpeg": true,
		".png":  true,
	}

	ext := strings.ToLower(filepath.Ext(filename))
	return allowedExtensions[ext]
}

func SubmitForm(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	fullName := c.PostForm("full_name")
	birthDate := c.PostForm("birth_date")
	region := c.PostForm("region")
	phone := c.PostForm("phone")
	iin := c.PostForm("iin")
	benefit := c.PostForm("benefit")

	if fullName == "" || birthDate == "" || region == "" || phone == "" || iin == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Все поля обязательны"})
		return
	}

	var status string
	var promotedAt sql.NullTime

	err := config.DB.QueryRow(`
    SELECT status, promoted_at
    FROM applications
    WHERE iin = $1
    ORDER BY id DESC
    LIMIT 1
`, iin).Scan(&status, &promotedAt)

	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка проверки ИИН"})
		return
	}

	if err == nil && (status == "pending" || status == "approved" || status == "promoted") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Вы уже подали заявку с этим ИИН, дождитесь её обработки."})
		return
	}

	if err == nil && (status == "archive" || status == "rejected") {
		if promotedAt.Valid {
			almatyLocation, _ := time.LoadLocation("Asia/Almaty")
			promotedTime := promotedAt.Time.In(almatyLocation)
			now := time.Now().In(almatyLocation)

			oneMinuteLater := promotedTime.Add(-5 * time.Hour)
			oneMinuteLater = oneMinuteLater.Add(5 * time.Minute)

			if now.Before(oneMinuteLater) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("Вы можете подать заявку с этим ИИН после %s", oneMinuteLater.Format("02.01.2006 15:04:05")),
				})
				return
			}
		}
	}

	var maxPlaces, currentCount int
	err = config.DB.QueryRow("SELECT max_places FROM quotas WHERE region = $1", region).Scan(&maxPlaces)
	if err == sql.ErrNoRows {
		fmt.Println("Ошибка: нет квоты для района", region)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Регион не зарегистрирован"})
		return
	} else if err != nil {
		fmt.Println("Ошибка при получении квоты:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения квоты"})
		return
	}

	err = config.DB.QueryRow("SELECT COUNT(*) FROM applications WHERE region = $1 AND deleted_at IS NULL", region).Scan(&currentCount)
	if err != nil {
		fmt.Println("Ошибка проверки текущего количества заявок", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка проверки текущего количества заявок"})
		return
	}

	if currentCount >= maxPlaces {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Квота на этот регион уже заполнена"})
		return
	}

	idCard, err := c.FormFile("id_card")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Удостоверение личности обязательно"})
		return
	}
	if !isValidFileType(idCard.Filename) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Допустимые форматы: PDF, JPEG, PNG"})
		return
	}
	idCardPath := "uploads/" + idCard.Filename
	c.SaveUploadedFile(idCard, idCardPath)

	var benefitDocPath *string
	benefitDoc, err := c.FormFile("benefit_doc")
	if err == nil {
		if !isValidFileType(benefitDoc.Filename) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Допустимые форматы: PDF, JPEG, PNG"})
			return
		}
		path := "uploads/" + benefitDoc.Filename
		c.SaveUploadedFile(benefitDoc, path)
		benefitDocPath = &path
	}

	if benefit != "Пенсионер" && benefitDocPath == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо загрузить документ, подтверждающий льготу"})
		return
	}

	_, err = config.DB.Exec(`
		INSERT INTO applications (user_id, full_name, birth_date, region, phone, iin, id_card, benefit, benefit_doc, status) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending')`,
		userID, fullName, birthDate, region, phone, iin, idCardPath, benefit, benefitDocPath)
	if err != nil {
		fmt.Println("Ошибка сохранения заявки:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения заявки"})
		return
	}

	_, err = config.DB.Exec(`
		WITH Ranked AS (
			SELECT a.id AS application_id,
			ROW_NUMBER() OVER (
				ORDER BY COALESCE(b.priority, 1) DESC, a.id ASC
			) AS new_queue_number
			FROM applications a
			LEFT JOIN benefits b ON a.benefit = b.name
			WHERE a.status NOT IN ('archive', 'rejected') AND a.deleted_at IS NULL
		)
		UPDATE applications
		SET queue_number = Ranked.new_queue_number
		FROM Ranked
		WHERE applications.id = Ranked.application_id;
	`)
	if err != nil {
		fmt.Println("Ошибка пересчета queue_number:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления queue_number"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Заявка успешно отправлена"})
}

func RejectApplicationUser(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Вы не авторизованы"})
		return
	}

	applicationID := c.PostForm("application_id")
	if applicationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный идентификатор заявки"})
		return
	}

	result, err := config.DB.Exec(`
        UPDATE applications 
        SET status = 'rejected', deleted_at = NOW()
        WHERE id = $1 AND user_id = $2 AND status IN ('pending', 'approved')
    `, applicationID, userID)

	rowsAffected, _ := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Заявка не найдена или её нельзя отклонить"})
		return
	}

	c.Redirect(http.StatusFound, "/profile")
}
