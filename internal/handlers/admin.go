package handlers

import (
	"fmt"
	"net/http"
	"project/config"
	"project/internal/models"
	"project/internal/services"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func AdminPage(c *gin.Context) {
	qRows, err := config.DB.Query("SELECT region, max_places FROM quotas")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения квот"})
		return
	}
	defer qRows.Close()

	quotas := make(map[string]int)
	var allocatedPlaces int
	for qRows.Next() {
		var region string
		var maxPlaces int
		if err := qRows.Scan(&region, &maxPlaces); err != nil {
			continue
		}
		quotas[region] = maxPlaces
		allocatedPlaces += maxPlaces
	}

	var totalPlaces int
	err = config.DB.QueryRow("SELECT total_places FROM system_settings").Scan(&totalPlaces)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки общего количества мест"})
		return
	}

	userRows, err := config.DB.Query("SELECT id, phone, email, role, region FROM users ORDER BY role DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки пользователей"})
		return
	}
	defer userRows.Close()

	var userList []models.User
	for userRows.Next() {
		var user models.User
		if err := userRows.Scan(&user.ID, &user.Phone, &user.Email, &user.Role, &user.Region); err != nil {
			continue
		}
		userList = append(userList, user)
	}

	regionRows, err := config.DB.Query("SELECT region FROM quotas ORDER BY region ASC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки районов"})
		return
	}
	defer regionRows.Close()

	var regions []string
	for regionRows.Next() {
		var region string
		if err := regionRows.Scan(&region); err != nil {
			continue
		}
		regions = append(regions, region)
	}

	bRows, err := config.DB.Query("SELECT name, priority FROM benefits ORDER BY priority DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки льгот"})
		return
	}
	defer bRows.Close()

	var benefits []models.Benefit
	for bRows.Next() {
		var benefit models.Benefit
		if err := bRows.Scan(&benefit.Name, &benefit.Priority); err != nil {
			continue
		}
		benefits = append(benefits, benefit)
	}

	c.HTML(http.StatusOK, "admin.html", gin.H{
		"quotas":           quotas,
		"total_places":     totalPlaces,
		"allocated_places": allocatedPlaces,
		"regions":          regions,
		"users":            userList,
		"benefits":         benefits,
	})
}

func SetQuota(c *gin.Context) {
	region := c.PostForm("region")
	maxPlacesStr := c.PostForm("max_places")

	maxPlaces, err := strconv.Atoi(maxPlacesStr)
	if err != nil || maxPlaces < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректное число мест (должно быть больше 0)"})
		return
	}

	var totalLimit int
	err = config.DB.QueryRow("SELECT total_places FROM system_settings").Scan(&totalLimit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения общего лимита мест"})
		return
	}

	var currentApplications int
	err = config.DB.QueryRow("SELECT COUNT(*) FROM applications WHERE region = $1 AND deleted_at IS NULL", region).Scan(&currentApplications)
	if err != nil {
		fmt.Println("Ошибка получения текущего количества заявок:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка проверки заявок"})
		return
	}

	if maxPlaces < currentApplications {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Нельзя установить %d мест, потому что уже подано %d заявок", maxPlaces, currentApplications),
		})
		return
	}

	var currentTotal int
	err = config.DB.QueryRow("SELECT COALESCE(SUM(max_places), 0) FROM quotas WHERE region <> $1", region).Scan(&currentTotal)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения суммарной квоты"})
		return
	}

	newTotal := currentTotal + maxPlaces
	if newTotal > totalLimit {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Общее количество мест (%d) превышает лимит (%d)", newTotal, totalLimit)})
		return
	}

	_, err = config.DB.Exec("INSERT INTO quotas (region, max_places) VALUES ($1, $2) ON CONFLICT (region) DO UPDATE SET max_places = EXCLUDED.max_places", region, maxPlaces)
	if err != nil {
		fmt.Println("Ошибка обновления квоты:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления квоты"})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func SetTotalPlaces(c *gin.Context) {
	totalPlacesStr := c.PostForm("total_places")

	totalPlaces, err := strconv.Atoi(totalPlacesStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректное значение общего лимита мест"})
		return
	}

	var allocatedPlaces int
	err = config.DB.QueryRow("SELECT COALESCE(SUM(max_places), 0) FROM quotas").Scan(&allocatedPlaces)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения суммарной квоты"})
		return
	}

	if totalPlaces < allocatedPlaces {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Общий лимит (%d) не может быть меньше уже выделенных мест (%d)", totalPlaces, allocatedPlaces)})
		return
	}

	_, err = config.DB.Exec("UPDATE system_settings SET total_places = $1", totalPlaces)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления общего лимита"})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func DeleteRegion(c *gin.Context) {
	region := c.PostForm("region")

	var exists bool
	err := config.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM quotas WHERE region = $1)", region).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Такого района нет в системе"})
		return
	}

	_, err = config.DB.Exec("DELETE FROM quotas WHERE region = $1", region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка удаления района"})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func AddBenefit(c *gin.Context) {
	name := c.PostForm("benefit_name")
	priority := c.PostForm("priority")

	_, err := config.DB.Exec("INSERT INTO benefits (name, priority) VALUES ($1, $2)", name, priority)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка добавления льготы"})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func UpdateBenefitPriority(c *gin.Context) {
	name := c.PostForm("benefit_name")
	priority := c.PostForm("priority")

	_, err := config.DB.Exec("UPDATE benefits SET priority = $1 WHERE name = $2", priority, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления приоритета"})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func DeleteBenefit(c *gin.Context) {
	name := c.PostForm("benefit_name")

	_, err := config.DB.Exec("DELETE FROM benefits WHERE name = $1", name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка удаления льготы"})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func CreateManager(c *gin.Context) {
	session := sessions.Default(c)
	userRole := session.Get("user_role")

	if userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещен"})
		return
	}

	userPhone := c.PostForm("phone")
	email := c.PostForm("email")
	password := c.PostForm("password")
	region := c.PostForm("region")

	if userPhone == "" || email == "" || password == "" || region == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Все поля обязательны"})
		return
	}

	var existingManager int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'manager' AND region = $1", region).Scan(&existingManager)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка проверки менеджера"})
		return
	}
	if existingManager > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Менеджер для этого района уже существует"})
		return
	}

	hashedPassword, err := services.HashPassword(password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка хэширования пароля"})
		return
	}

	_, err = config.DB.Exec("INSERT INTO users (phone, email, password, role, region) VALUES ($1, $2, $3, 'manager', $4)",
		userPhone, email, hashedPassword, region)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания менеджера"})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func DeleteManager(c *gin.Context) {
	managerID := c.PostForm("manager_id")

	_, err := config.DB.Exec("DELETE FROM users WHERE id = $1 AND role = 'manager'", managerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка удаления менеджера"})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func UpdateManager(c *gin.Context) {
	managerID := c.PostForm("manager_id")
	userPhone := c.PostForm("phone")
	email := c.PostForm("email")
	region := c.PostForm("region")

	if managerID == "" || userPhone == "" || email == "" || region == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Все поля обязательны"})
		return
	}

	_, err := config.DB.Exec("UPDATE users SET phone = $1, email = $2, region = $3 WHERE id = $4 AND role = 'manager'",
		userPhone, email, region, managerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления данных менеджера"})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}
