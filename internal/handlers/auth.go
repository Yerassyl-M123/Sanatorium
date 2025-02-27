package handlers

import (
	"fmt"
	"log"
	"net/http"
	"project/config"
	"project/internal/models"
	"project/internal/services"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/exp/rand"
)

func SignInPage(c *gin.Context) {
	c.HTML(http.StatusOK, "signIn.html", nil)
}

func SignIn(c *gin.Context) {
	phone := c.PostForm("user-phone")
	password := c.PostForm("user-password")

	var storedUser models.User
	err := config.DB.QueryRow("SELECT id, phone, password, role FROM users WHERE phone = $1", phone).Scan(&storedUser.ID, &storedUser.Phone, &storedUser.Password, &storedUser.Role)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный номер или пароль"})
		return
	}

	if !services.CheckPasswordHash(password, storedUser.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный номер или пароль"})
		return
	}

	session := sessions.Default(c)
	session.Set("user_id", storedUser.ID)
	session.Set("user_phone", storedUser.Phone)
	session.Set("user_role", storedUser.Role)
	session.Save()

	c.Redirect(http.StatusFound, "/")
}

func SignUpPage(c *gin.Context) {
	c.HTML(http.StatusOK, "signUp.html", nil)
}

func SignUp(c *gin.Context) {
	userPhone := c.PostForm("user-phone")
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
	err := config.DB.QueryRow("SELECT id FROM users WHERE phone = $1", userPhone).Scan(&existingUser.ID)
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
	err = config.DB.QueryRow("INSERT INTO users (phone, password) VALUES ($1, $2) RETURNING id", userPhone, hashedPassword).Scan(&newUser.ID)
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

var resetCodes = make(map[string]string)

func generateCode() string {
	rand.Seed(uint64(time.Now().UnixNano()))
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

func SendResetCode(c *gin.Context) {
	var request struct {
		Phone string `json:"phone"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}

	var userID int
	err := config.DB.QueryRow("SELECT id FROM users WHERE phone = $1", request.Phone).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Пользователь с таким номером не найден"})
		return
	}

	code := generateCode()
	resetCodes[request.Phone] = code

	message := "Ваш код для сброса пароля: " + code
	services.SendSMS(request.Phone, message)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func ResetPassword(c *gin.Context) {
	var request struct {
		Phone       string `json:"phone"`
		Code        string `json:"code"`
		NewPassword string `json:"newPassword"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}

	storedCode, exists := resetCodes[request.Phone]
	if !exists || storedCode != request.Code {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный код"})
		return
	}

	hashedPassword, err := services.HashPassword(request.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка хеширования пароля"})
		return
	}

	if len(request.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Пароль должен быть длиной не менее 8 символов"})
		return
	}

	_, err = config.DB.Exec("UPDATE users SET password = $1 WHERE phone = $2", hashedPassword, request.Phone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления пароля"})
		return
	}

	delete(resetCodes, request.Phone)

	c.JSON(http.StatusOK, gin.H{"success": true})
}
