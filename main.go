package main

import (
	"fakidoosuurdoris/app/Internal/handlers"
	"fakidoosuurdoris/app/Internal/middlewares"
	"fakidoosuurdoris/app/Internal/services"
	"fakidoosuurdoris/app/config"

	"context"
	"database/sql"
	"html/template"
	"log"

	firebase "firebase.google.com/go/v4"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"google.golang.org/api/option"
)

func methodOverride() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "POST" {
			method := c.PostForm("_method")
			if method == "DELETE" {
				c.Request.Method = "DELETE"
			}
		}
		c.Next()
	}
}

func main() {
	config.LoadEnv()

	credentialsPath := config.GetEnv("FIREBASE_CREDENTIALS", "./serviceAccountKey.json")
	fbApp, err := firebase.NewApp(context.Background(), nil, option.WithCredentialsFile(credentialsPath))
	if err != nil {
		log.Fatalf("Failed to initialize Firebase: %v", err)
	}
	authClient, err := fbApp.Auth(context.Background())
	if err != nil {
		log.Fatalf("Failed to initialize Firebase Auth: %v", err)
	}

	connStr := config.GetEnv("DATABASE_URL", "")
	if connStr == "" {
		log.Fatal("DATABASE_URL not set in .env")
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal("Failed to parse templates:", err)
	}

	r := gin.Default()
	r.SetHTMLTemplate(tmpl)

	authService := services.NewAuthService(db, fbApp, authClient) // Fixed: Pass only authClient
	pollService := services.NewPollService(db, authService)
	userService := services.NewUserService(db, authClient)

	app := &handlers.App{
		AuthService: authService,
		Templates:   tmpl,
	}

	userHandler := handlers.NewUserHandler(userService, tmpl)
	pollHandler := handlers.NewPollHandler(pollService, authService, tmpl, db)
	voteHandler := handlers.NewVoteHandler(pollService, authService, tmpl)
	adminHandler := handlers.NewAdminHandler(pollService, authService, tmpl)

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	r.Use(middlewares.CSRFMiddleware())
	r.Use(methodOverride())

	r.GET("/", app.RenderHome)
	r.GET("/home", app.RenderHome)

	r.GET("/register", app.RenderRegister)
	r.POST("/register", handlers.Register(app))

	r.GET("/login", app.RenderLogin)
	r.POST("/login", handlers.Login(app))

	protected := r.Group("/")
	protected.Use(middlewares.AuthMiddleware(authClient), middlewares.CSRFMiddleware())
	{
		protected.GET("/polls", pollHandler.RenderCreatePoll)
		protected.POST("/polls", pollHandler.CreatePoll)
		protected.GET("/my-polls", pollHandler.RenderMyPolls)
		protected.GET("/polls-list", pollHandler.RenderPollsList)
		protected.GET("/polls/edit/:id", pollHandler.RenderEditPoll)
		protected.POST("/polls/update/:id", pollHandler.UpdatePoll)
		protected.POST("/polls/delete/:id", pollHandler.DeletePoll)
		protected.GET("/profile", userHandler.RenderProfile)
		protected.POST("/profile", userHandler.UpdateProfile)
		protected.GET("/profile/edit", userHandler.RenderEditProfile)
		protected.GET("/profile/password", userHandler.RenderChangePassword)
		protected.POST("/profile/password", userHandler.UpdatePassword)
		protected.GET("/logout", userHandler.Logout)
		protected.GET("/admin", app.RenderMakeAdmin)
		protected.POST("/admin/make", app.MakeAdmin)
		protected.GET("/vote/:id", voteHandler.RenderVote)
		protected.POST("/vote/:id", voteHandler.Vote)
		protected.GET("/admin/polls", adminHandler.RenderAdminPolls)
		protected.GET("/admin/users", adminHandler.RenderAdminUsers)
		protected.GET("/api/admin/polls/:id/summary", adminHandler.GetPollSummary)
		protected.GET("/api/admin/polls/:id/summary/download", adminHandler.DownloadPollSummary)
		protected.GET("/admin/users/:id/delete", adminHandler.DeleteUser)
	}

	api := r.Group("/api")
	api.POST("/register", handlers.Register(app))
	api.POST("/login", handlers.Login(app))
	api.POST("/polls", middlewares.AuthMiddleware(authClient), pollHandler.CreatePoll)
	protectedAPI := api.Group("")
	protectedAPI.Use(middlewares.AuthMiddleware(authClient))

	port := config.GetPort()
	log.Printf("Server starting on http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
