package handlers

import (
	"html/template"
	"log"
	"net/http"

	"fakidoosuurdoris/app/Internal/services"

	"github.com/gin-gonic/gin"
)

type App struct {
	AuthService *services.AuthService
	Templates   *template.Template
}

func (app *App) RenderHome(c *gin.Context) {
	c.HTML(http.StatusOK, "home.html", gin.H{
		"Title": "Voting System",
	})
}

func (app *App) RenderRegister(c *gin.Context) {
	csrfToken, _ := c.Get("csrf_token")
	c.HTML(http.StatusOK, "register.html", gin.H{
		"Title":     "Register",
		"CSRFToken": csrfToken,
	})
}

func (app *App) RenderLogin(c *gin.Context) {
	updated := c.Query("updated") == "true"
	passwordUpdated := c.Query("password_updated") == "true"
	expired := c.Query("expired") == "true"
	loggedOut := c.Query("logged_out") == "true"
	c.HTML(http.StatusOK, "login.html", gin.H{
		"Title":           "Login",
		"Updated":         updated,
		"PasswordUpdated": passwordUpdated,
		"Expired":         expired,
		"LoggedOut":       loggedOut,
	})
}

func Register(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			FirstName string `form:"firstname" binding:"required"`
			LastName  string `form:"lastname" binding:"required"`
			Email     string `form:"email" binding:"required,email"`
			Password  string `form:"password" binding:"required,min=6"`
		}

		if err := c.ShouldBind(&input); err != nil {
			log.Printf("Registration form binding error: %v, Form data: %+v", err, c.Request.PostForm)
			csrfToken, _ := c.Get("csrf_token")
			c.HTML(http.StatusBadRequest, "register.html", gin.H{
				"Title":     "Register",
				"Error":     "Please fill all fields correctly",
				"CSRFToken": csrfToken,
				"Input":     input,
			})
			return
		}

		uid, err := app.AuthService.Register(c.Request.Context(), input.FirstName, input.LastName, input.Email, input.Password, "user")
		if err != nil {
			log.Printf("Registration failed: %v", err)
			csrfToken, _ := c.Get("csrf_token")
			c.HTML(http.StatusInternalServerError, "register.html", gin.H{
				"Title":     "Register",
				"Error":     "Registration failed: " + err.Error(),
				"CSRFToken": csrfToken,
				"Input":     input,
			})
			return
		}
		idToken, _, err := app.AuthService.Login(c.Request.Context(), c.PostForm("idToken"))
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			return
		}
		c.SetCookie("idToken", idToken, 3600, "/", "localhost", false, true)
		c.Set("uid", uid)

		c.Redirect(http.StatusSeeOther, "/polls-list")
	}
}

func Login(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		idToken := c.PostForm("idToken")
		if idToken == "" {
			log.Printf("No idToken provided, Form data: %+v", c.Request.PostForm)
			c.HTML(http.StatusBadRequest, "login.html", gin.H{
				"Title": "Login",
				"Error": "Incorrect email or password",
			})
			return
		}

		log.Printf("Got idToken: %s...", idToken[:10])

		token, uid, err := app.AuthService.Login(c.Request.Context(), idToken)
		if err != nil {
			log.Printf("Login error: %v", err)
			c.HTML(http.StatusUnauthorized, "login.html", gin.H{
				"Title": "Login",
				"Error": "Invalid idToken. Please try again.",
			})
			return
		}

		role, err := app.AuthService.GetUserRole(c.Request.Context(), uid)
		if err != nil {
			log.Printf("Failed to get user role: %v", err)
			c.HTML(http.StatusInternalServerError, "login.html", gin.H{
				"Title": "Login",
				"Error": "Could not verify user role",
			})
			return
		}

		c.SetCookie("idToken", token, 84600, "/", "localhost", false, true)
		c.Set("uid", uid)

		if role == "admin" {
			c.Redirect(http.StatusSeeOther, "/my-polls")
		} else {
			c.Redirect(http.StatusSeeOther, "/polls-list")
		}

	}

}

func (app *App) MakeAdmin(c *gin.Context) {
	idToken := c.PostForm("idToken")
	if idToken == "" {
		c.HTML(http.StatusUnauthorized, "make_admin.html", gin.H{
			"Title": "Make Admin",
			"Error": "Please log in",
		})
		return
	}

	_, adminID, err := app.AuthService.Login(c.Request.Context(), idToken)
	if err != nil {
		log.Printf("Login error: %v", err)
		c.HTML(http.StatusUnauthorized, "make_admin.html", gin.H{
			"Title": "Make Admin",
			"Error": "Invalid idToken",
		})
		return
	}

	var input struct {
		Email string `form:"email" binding:"required,email"`
	}
	if err := c.ShouldBind(&input); err != nil {
		c.HTML(http.StatusBadRequest, "make_admin.html", gin.H{
			"Title": "Make Admin",
			"Error": "Invalid email address",
		})
		return
	}

	if err := app.AuthService.SetAdminRole(c.Request.Context(), input.Email, adminID); err != nil {
		log.Printf("Failed to set admin role: %v", err)
		c.HTML(http.StatusInternalServerError, "make_admin.html", gin.H{
			"Title": "Make Admin",
			"Error": "Failed to set admin role: " + err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "make_admin.html", gin.H{
		"Title":   "Make Admin",
		"Message": "Admin role assigned to " + input.Email,
	})
}

func (app *App) RenderMakeAdmin(c *gin.Context) {
	idToken := c.PostForm("idToken")
	if idToken == "" {
		idToken, _ = c.Cookie("idToken")
	}
	if idToken == "" {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"Title": "Login",
			"Error": "Please log in",
		})
		return
	}

	isAdmin, err := app.AuthService.IsAdmin(c.Request.Context(), idToken)
	if err != nil || !isAdmin {
		log.Printf("Unauthorized access to admin page: %v", err)
		c.HTML(http.StatusForbidden, "home.html", gin.H{
			"Title": "Home",
			"Error": "Admin access required",
		})
		return
	}

	csrfToken, _ := c.Get("csrf_token")
	c.HTML(http.StatusOK, "make_admin.html", gin.H{
		"Title":     "Make Admin",
		"CSRFToken": csrfToken,
	})
}
