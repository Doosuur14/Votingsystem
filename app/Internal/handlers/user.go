package handlers

import (
	"html/template"
	"log"
	"net/http"

	"fakidoosuurdoris/app/Internal/services"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	UserService *services.UserService
	Templates   *template.Template
}

func NewUserHandler(userService *services.UserService, templates *template.Template) *UserHandler {
	return &UserHandler{
		UserService: userService,
		Templates:   templates,
	}
}

func (h *UserHandler) invalidateSession(c *gin.Context, redirectReason string) {
	c.SetCookie("idToken", "", -1, "/", "localhost", false, true)
	c.Redirect(http.StatusSeeOther, "/login?"+redirectReason+"=true")
}

func (h *UserHandler) Logout(c *gin.Context) {
	log.Printf("Logout: Clearing idToken")
	h.invalidateSession(c, "logged_out")
}

func (h *UserHandler) RenderProfile(c *gin.Context) {
	uid, exists := c.Get("uid")
	if !exists {
		c.HTML(http.StatusUnauthorized, "home.html", gin.H{"Error": "Please log in"})
		return
	}

	user, err := h.UserService.GetUserByID(c.Request.Context(), uid.(string))
	if err != nil {
		log.Printf("Failed to fetch user: %v", err)
		c.HTML(http.StatusInternalServerError, "profile.html", gin.H{"Error": "Could not load profile"})
		return
	}

	totalPolls, err := h.UserService.GetUserPollsCount(c.Request.Context(), uid.(string))
	if err != nil {
		log.Printf("Failed to fetch poll count: %v", err)
		totalPolls = 0
	}

	csrfToken, _ := c.Get("csrf_token")
	c.HTML(http.StatusOK, "profile.html", gin.H{
		"Title":      "Profile",
		"User":       user,
		"TotalPolls": totalPolls,
		"CSRFToken":  csrfToken,
	})
}

func (h *UserHandler) RenderEditProfile(c *gin.Context) {
	uid, exists := c.Get("uid")
	log.Printf("RenderEditProfile: uid=%v, exists=%v", uid, exists)
	if !exists {
		log.Printf("RenderEditProfile: No uid in context, redirecting to login")
		c.Redirect(http.StatusSeeOther, "/login?expired=true")
		return
	}
	userID := uid.(string)

	user, err := h.UserService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		log.Printf("RenderEditProfile: Failed to fetch user %s: %v", userID, err)
		c.HTML(http.StatusInternalServerError, "profile_edit.html", gin.H{
			"Error": "Could not load profile",
		})
		return
	}

	role, err := h.UserService.GetUserRole(c.Request.Context(), userID)
	if err != nil {
		log.Printf("RenderEditProfile: Failed to fetch role for user %s: %v", userID, err)
		c.HTML(http.StatusInternalServerError, "profile_edit.html", gin.H{
			"Error": "Could not load user role",
		})
		return
	}

	csrfToken, _ := c.Get("csrf_token")
	data := gin.H{
		"Title":     "Edit Profile",
		"User":      user,
		"CSRFToken": csrfToken,
		"Role":      role,
	}
	log.Printf("RenderEditProfile: Rendering profile_edit.html for user %s with role: %s, data: %+v", userID, role, data)

	c.HTML(http.StatusOK, "profile_edit.html", data)
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	uid, exists := c.Get("uid")
	if !exists {
		log.Printf("No uid found in context for POST /profile, Cookies: %+v", c.Request.Cookies())
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"Title": "Login",
			"Error": "Please log in to update your profile",
		})
		return
	}

	uidStr, ok := uid.(string)
	if !ok {
		log.Printf("Invalid uid type in context: %T", uid)
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"Title": "Login",
			"Error": "Internal server error",
		})
		return
	}

	var input struct {
		FirstName string `form:"firstname" binding:"required"`
		LastName  string `form:"lastname" binding:"required"`
		Email     string `form:"email" binding:"required,email"`
	}

	if err := c.ShouldBind(&input); err != nil {
		log.Printf("Form binding error: %v, Form data: %+v", err, c.Request.PostForm)
		csrfToken, ok := c.Get("csrf_token")
		if !ok {
			log.Printf("CSRF token missing for POST /profile")
			c.HTML(http.StatusInternalServerError, "profile_edit.html", gin.H{
				"Title": "Edit Profile",
				"Error": "Internal server error",
			})
			return
		}
		user, err := h.UserService.GetUserByID(c.Request.Context(), uidStr)
		if err != nil {
			log.Printf("Failed to fetch user for UID %s: %v", uidStr, err)
			c.HTML(http.StatusInternalServerError, "profile_edit.html", gin.H{
				"Title":     "Edit Profile",
				"Error":     "Failed to load user data",
				"CSRFToken": csrfToken,
			})
			return
		}
		c.HTML(http.StatusBadRequest, "profile_edit.html", gin.H{
			"Title":     "Edit Profile",
			"Error":     "Invalid input",
			"User":      user,
			"Input":     input,
			"CSRFToken": csrfToken,
		})
		return
	}

	err := h.UserService.UpdateUser(c.Request.Context(), uidStr, input.FirstName, input.LastName, input.Email)
	if err != nil {
		log.Printf("Profile update failed for UID %s: %v", uidStr, err)
		csrfToken, ok := c.Get("csrf_token")
		if !ok {
			log.Printf("CSRF token missing for POST /profile")
			c.HTML(http.StatusInternalServerError, "profile_edit.html", gin.H{
				"Title": "Edit Profile",
				"Error": "Internal server error",
			})
			return
		}
		user, err := h.UserService.GetUserByID(c.Request.Context(), uidStr)
		if err != nil {
			log.Printf("Failed to fetch user for UID %s: %v", uidStr, err)
			c.HTML(http.StatusInternalServerError, "profile_edit.html", gin.H{
				"Title":     "Edit Profile",
				"Error":     "Failed to load user data",
				"CSRFToken": csrfToken,
			})
			return
		}
		c.HTML(http.StatusInternalServerError, "profile_edit.html", gin.H{
			"Title":     "Edit Profile",
			"Error":     "Could not update profile: " + err.Error(),
			"User":      user,
			"Input":     input,
			"CSRFToken": csrfToken,
		})
		return
	}

	// h.invalidateSession(c, "User credential edited")
	//c.Redirect(http.StatusSeeOther, "/profile?updated=true")
	c.Redirect(http.StatusSeeOther, "/profile")
}

func (h *UserHandler) RenderChangePassword(c *gin.Context) {
	uid, exists := c.Get("uid")
	log.Printf("RenderChangePassword: uid=%v, exists=%v", uid, exists)
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login?expired=true")
		return
	}

	csrfToken, _ := c.Get("csrf_token")
	c.HTML(http.StatusOK, "profile_password.html", gin.H{
		"Title":     "Change Password",
		"CSRFToken": csrfToken,
	})
}

func (h *UserHandler) UpdatePassword(c *gin.Context) {
	uid, exists := c.Get("uid")
	log.Printf("UpdatePassword: uid=%v, exists=%v", uid, exists)
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login?expired=true")
		return
	}

	var input struct {
		CurrentPassword string `form:"current_password" binding:"required"`
		NewPassword     string `form:"new_password" binding:"required,min=6"`
		ConfirmPassword string `form:"confirm_password" binding:"required"`
	}

	if err := c.ShouldBind(&input); err != nil {
		log.Printf("Form binding error: %v, Form data: %+v", err, c.Request.PostForm)
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusBadRequest, "profile_password.html", gin.H{
			"Title":     "Change Password",
			"Error":     "Invalid input",
			"Input":     input,
			"CSRFToken": csrfToken,
		})
		return
	}

	if input.NewPassword != input.ConfirmPassword {
		log.Printf("Password mismatch")
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusBadRequest, "profile_password.html", gin.H{
			"Title":     "Change Password",
			"Error":     "New password and confirmation do not match",
			"Input":     input,
			"CSRFToken": csrfToken,
		})
		return
	}

	err := h.UserService.UpdatePassword(c.Request.Context(), uid.(string), input.NewPassword)
	if err != nil {
		log.Printf("Password update failed: %v", err)
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusInternalServerError, "profile_password.html", gin.H{
			"Title":     "Change Password",
			"Error":     "Could not update password",
			"Input":     input,
			"CSRFToken": csrfToken,
		})
		return
	}

	// h.invalidateSession(c, "password_updated")
	c.Redirect(http.StatusSeeOther, "/profile")
}
