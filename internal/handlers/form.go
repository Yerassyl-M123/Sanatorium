package handlers

import (
	"net/http"
	"project/config"

	"github.com/gin-gonic/gin"
)

func FormPage(c *gin.Context) {
	rows, err := config.DB.Query("SELECT region FROM quotas")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки списка районов"})
		return
	}
	defer rows.Close()

	var regions []string
	for rows.Next() {
		var region string
		rows.Scan(&region)
		regions = append(regions, region)
	}

	bRows, err := config.DB.Query("SELECT name FROM benefits")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки списка льгот"})
		return
	}
	defer bRows.Close()

	var benefits []string
	for bRows.Next() {
		var benefit string
		bRows.Scan(&benefit)
		benefits = append(benefits, benefit)
	}

	c.HTML(http.StatusOK, "form.html", gin.H{
		"regions":  regions,
		"benefits": benefits,
	})
}
