package handlers

import (
	"database/sql"
	"net/http"
	"project/config"
	"project/internal/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func ProfilePage(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.Redirect(http.StatusFound, "/signInPage")
		return
	}

	var user models.User
	err := config.DB.QueryRow("SELECT id, phone, password FROM users WHERE id = $1", userID).
		Scan(&user.ID, &user.Phone, &user.Password)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки профиля"})
		return
	}

	rows, err := config.DB.Query(`
        SELECT id, status, queue_number, region, full_name
        FROM applications
        WHERE user_id = $1
        ORDER BY id DESC
    `, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения данных"})
		return
	}
	defer rows.Close()

	var applications []models.Application
	for rows.Next() {
		var app models.Application
		if err := rows.Scan(&app.ID, &app.Status, &app.QueueNumber, &app.Region, &app.FullName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обработки заявок"})
			return
		}
		applications = append(applications, app)
	}

	c.HTML(http.StatusOK, "profile.html", gin.H{
		"user":         user,
		"applications": applications,
	})

	// c.JSON(http.StatusOK, gin.H{
	// 	"user":         user,
	// 	"applications": applications,
	// })
}

func UpdateProfile(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Вы не авторизованы"})
		return
	}

	userPhone := c.PostForm("phone")

	if userPhone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Все поля обязательны"})
		return
	}

	_, err := config.DB.Exec(`
        UPDATE users 
        SET phone = $1 
        WHERE id = $2
    `, userPhone, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления профиля"})
		return
	}

	session.Save()

	c.Redirect(http.StatusFound, "/profile")
}

func UpdatePassword(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Вы не авторизованы"})
		return
	}

	oldPassword := c.PostForm("old_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	if oldPassword == "" || newPassword == "" || confirmPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Все поля обязательны"})
		return
	}

	if newPassword != confirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Пароли не совпадают"})
		return
	}

	if len(newPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Пароль должен быть длиной не менее 8 символов"})
		return
	}

	var hashedPassword string
	err := config.DB.QueryRow("SELECT password FROM users WHERE id = $1", userID).Scan(&hashedPassword)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Пользователь не найден"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения пароля"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(oldPassword))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный старый пароль"})
		return
	}

	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка хеширования пароля"})
		return
	}

	_, err = config.DB.Exec("UPDATE users SET password = $1 WHERE id = $2", newHashedPassword, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления пароля"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Пароль успешно изменён"})
}
