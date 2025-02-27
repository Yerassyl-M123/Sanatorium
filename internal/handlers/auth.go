package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"project/config"
	"project/internal/models"
	"project/internal/services"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func SignInPage(c *gin.Context) {
	c.HTML(http.StatusOK, "signIn.html", nil)
}

func SignIn(c *gin.Context) {
	email := c.PostForm("user-email")
	password := c.PostForm("user-password")

	var storedUser models.User
	err := config.DB.QueryRow("SELECT id, phone, email, password, role FROM users WHERE email = $1", email).Scan(&storedUser.ID, &storedUser.Phone, &storedUser.Email, &storedUser.Password, &storedUser.Role)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный email или пароль"})
		return
	}

	if !services.CheckPasswordHash(password, storedUser.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный email или пароль"})
		return
	}

	session := sessions.Default(c)
	session.Set("user_id", storedUser.ID)
	session.Set("user_email", storedUser.Email)
	session.Set("user_role", storedUser.Role)
	session.Save()

	c.Redirect(http.StatusFound, "/")
}

func SignUpPage(c *gin.Context) {
	c.HTML(http.StatusOK, "signUp.html", nil)
}

func SignUp(c *gin.Context) {
	userPhone := c.PostForm("user-phone")
	userEmail := c.PostForm("user-email")
	userPass := c.PostForm("user-password")
	confirmpass := c.PostForm("confirm-password")

	if userPhone == "" || len(userPass) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Пароль должен быть длиной не менее 8 символов"})
		return
	}

	if userPass != confirmpass {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Пароли не совпадают"})
		return
	}

	var existingUser models.User
	err := config.DB.QueryRow("SELECT id FROM users WHERE email = $1", userEmail).Scan(&existingUser.ID)
	if err != sql.ErrNoRows {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Эта почта уже зарегистрирована"})
		return
	}

	err = config.DB.QueryRow("SELECT id FROM users WHERE phone = $1", userPhone).Scan(&existingUser.ID)
	if err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Этот номер телефона уже зарегистрирован"})
		return
	}

	hashedPassword, err := services.HashPassword(userPass)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	var newUser models.User
	err = config.DB.QueryRow("INSERT INTO users (phone, email, password) VALUES ($1, $2, $3) RETURNING id", userPhone, userEmail, hashedPassword).Scan(&newUser.ID)
	if err != nil {
		log.Fatal(err)
	}

	c.Redirect(http.StatusFound, "/")
}

func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()

	c.Redirect(http.StatusFound, "/signInPage")
}
