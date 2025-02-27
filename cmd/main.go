package main

import (
	"project/config"
	"project/internal/handlers"
	"project/internal/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	db := config.ConnectDB()
	defer db.Close()

	server := gin.Default()
	server.Static("/uploads", "./uploads")

	store := cookie.NewStore([]byte("secret"))
	store.Options(sessions.Options{
		MaxAge: 5 * 60,
		// HttpOnly: true,
		Secure: false,
		// SameSite: http.SameSiteNoneMode,
	})
	server.Use(sessions.Sessions("mysession", store))
	server.Use(middleware.SessionTimeout())

	server.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	server.LoadHTMLGlob("templates/*")

	server.GET("/", middleware.AuthRequired(handlers.HomePage))
	server.GET("/form", middleware.AuthRequired(handlers.FormPage))
	server.POST("/submit-form", middleware.AuthRequired(handlers.SubmitForm))
	server.GET("/profile", middleware.AuthRequired(handlers.ProfilePage))
	server.POST("/reject-application-user", middleware.AuthRequired(handlers.RejectApplicationUser))
	server.POST("/update-profile", middleware.AuthRequired(handlers.UpdateProfile))
	server.POST("/update-password", middleware.AuthRequired(handlers.UpdatePassword))

	server.GET("/admin", middleware.AuthMiddleware("admin"), handlers.AdminPage)
	server.POST("/set-quota", middleware.AuthMiddleware("admin"), handlers.SetQuota)
	server.POST("/set-total-places", middleware.AuthMiddleware("admin"), handlers.SetTotalPlaces)
	server.POST("/delete-region", middleware.AuthMiddleware("admin"), handlers.DeleteRegion)
	server.POST("/add-benefit", middleware.AuthMiddleware("admin"), handlers.AddBenefit)
	server.POST("/update-benefit-priority", middleware.AuthMiddleware("admin"), handlers.UpdateBenefitPriority)
	server.POST("/delete-benefit", middleware.AuthMiddleware("admin"), handlers.DeleteBenefit)
	server.POST("/create-manager", middleware.AuthMiddleware("admin"), handlers.CreateManager)
	server.POST("/update-manager", middleware.AuthMiddleware("admin"), handlers.UpdateManager)
	server.POST("/delete-manager", middleware.AuthMiddleware("admin"), handlers.DeleteManager)

	server.POST("/approve-application", middleware.AuthMiddleware("manager"), handlers.ApproveApplication)
	server.POST("/reject-application", middleware.AuthMiddleware("manager"), handlers.RejectApplication)
	server.POST("/promote-application", middleware.AuthMiddleware("manager"), handlers.PromoteApplication)
	server.GET("/manager", middleware.AuthMiddleware("manager"), handlers.ManagerPage)

	server.GET("/signUpPage", handlers.SignUpPage)
	server.POST("/signup", handlers.SignUp)
	server.GET("/signInPage", handlers.SignInPage)
	server.POST("/signin", handlers.SignIn)
	server.GET("/logout", handlers.Logout)

	server.GET("/user", middleware.AuthRequired(handlers.GetUser))
	server.GET("/regions", handlers.GetRegions)
	server.GET("/applications", handlers.GetApplications)

	config.StartCronJobs()

	server.Run(":8080")
}
