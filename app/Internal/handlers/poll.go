package handlers

import (
	"database/sql"
	"fakidoosuurdoris/app/Internal/models"
	"fakidoosuurdoris/app/Internal/services"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type PollHandler struct {
	PollService *services.PollService
	AuthService *services.AuthService
	Templates   *template.Template
	DB          *sql.DB
}

func NewPollHandler(pollService *services.PollService, authService *services.AuthService, templates *template.Template, db *sql.DB) *PollHandler {
	return &PollHandler{
		PollService: pollService,
		AuthService: authService,
		Templates:   templates,
		DB:          db,
	}
}

func (h *PollHandler) RenderCreatePoll(c *gin.Context) {
	log.Println("Showing create poll page")
	csrfToken, _ := c.Get("csrf_token")
	uid, exists := c.Get("uid")
	var role string
	if exists {
		role, _ = h.PollService.AuthService.GetUserRole(c.Request.Context(), uid.(string))
	}
	c.HTML(http.StatusOK, "createpolls.html", gin.H{
		"Title":     "Create Poll",
		"CSRFToken": csrfToken,
		"Role":      role,
	})
}

func (h *PollHandler) CreatePoll(c *gin.Context) {

	uid, exists := c.Get("uid")
	if !exists {
		log.Println("User not logged in")
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusUnauthorized, "createpolls.html", gin.H{
			"Title":     "Create Poll",
			"Error":     "Please log in",
			"CSRFToken": csrfToken,
		})
		return
	}

	var role string
	err := h.DB.QueryRow("SELECT role FROM users WHERE id = $1", uid.(string)).Scan(&role)
	if err != nil || role != "admin" {
		log.Printf("Unauthorized: User %s is not admin (role: %s, err: %v)", uid, role, err)
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusForbidden, "createpolls.html", gin.H{
			"Title":     "Create Poll",
			"Error":     "Admin access required",
			"CSRFToken": csrfToken,
		})
		return
	}

	var input struct {
		Title        string   `form:"title" binding:"required"`
		QuestionType string   `form:"question_type" binding:"required,oneof=single_choice multiple_choice scale text"`
		Options      []string `form:"options[]"`
		StartDate    string   `form:"start_date" binding:"required"`
		EndDate      string   `form:"end_date"`
		IsAnonymous  string   `form:"is_anonymous"`
	}

	if err := c.ShouldBind(&input); err != nil {
		log.Printf("Form binding error: %v, Form data: %+v", err, c.Request.PostForm)
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusBadRequest, "createpolls.html", gin.H{
			"Title":     "Create Poll",
			"Error":     "Error occurred while trying to parse information",
			"Input":     input,
			"CSRFToken": csrfToken,
		})
		return
	}

	isAnonymous := input.IsAnonymous == "on"
	log.Printf("Checkbox value: %q → anonymous = %v", input.IsAnonymous, isAnonymous)

	var options []string
	if input.QuestionType == "single_choice" || input.QuestionType == "multiple_choice" {
		hasValidOption := false
		for _, opt := range input.Options {
			if trimmed := strings.TrimSpace(opt); trimmed != "" {
				options = append(options, trimmed)
				hasValidOption = true
			}
		}
		if !hasValidOption {
			log.Printf("No valid options provided for %s poll", input.QuestionType)
			csrfToken, _ := c.Get("csrf_token")
			c.HTML(http.StatusBadRequest, "createpolls.html", gin.H{
				"Title":     "Create Poll",
				"Error":     "At least one non-empty option is required for single/multiple choice polls",
				"Input":     input,
				"CSRFToken": csrfToken,
			})
			return
		}
	} else if input.QuestionType == "scale" {
		// hasValidOption := false
		for _, opt := range input.Options {
			if trimmed := strings.TrimSpace(opt); trimmed != "" {
				options = append(options, trimmed)
				// hasValidOption = true
			}
		}
		// if !hasValidOption {
		// 	log.Printf("No valid label provided for scale poll")
		// 	csrfToken, _ := c.Get("csrf_token")
		// 	c.HTML(http.StatusBadRequest, "createpolls.html", gin.H{
		// 		"Title":     "Create Poll",
		// 		"Error":     "A label (e.g., Rating) is required for scale polls",
		// 		"Input":     input,
		// 		"CSRFToken": csrfToken,
		// 	})
		// 	return
		// }
	}

	startDate, err := time.Parse("2006-01-02T15:04", input.StartDate)
	if err != nil {
		startDate, err = time.Parse("2006-01-02T15:04:05", input.StartDate+":00")
		if err != nil {
			log.Printf("Invalid start date: %v, Input: %s", err, input.StartDate)
			csrfToken, _ := c.Get("csrf_token")
			c.HTML(http.StatusBadRequest, "createpolls.html", gin.H{
				"Title":     "Create Poll",
				"Error":     "Invalid start date format",
				"CSRFToken": csrfToken,
			})
			return
		}
	}

	var endDate *time.Time
	if input.EndDate != "" {
		t, err := time.Parse("2006-01-02T15:04", input.EndDate)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05", input.EndDate+":00")
			if err != nil {
				log.Printf("Invalid end date: %v, Input: %s", err, input.EndDate)
				csrfToken, _ := c.Get("csrf_token")
				c.HTML(http.StatusBadRequest, "createpolls.html", gin.H{
					"Title":     "Create Poll",
					"Error":     "Invalid end date format",
					"CSRFToken": csrfToken,
				})
				return
			}
		}
		endDate = &t
	}

	poll, err := h.PollService.CreatePoll(c.Request.Context(), input.Title, input.QuestionType, options, isAnonymous, uid.(string), startDate, endDate)
	if err != nil {
		log.Printf("Poll creation failed: %v", err)
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusInternalServerError, "createpolls.html", gin.H{
			"Title":     "Create Poll",
			"Error":     "Could not create poll",
			"CSRFToken": csrfToken,
		})
		return
	}
	log.Printf("Poll created with ID: %v", poll.ID)
	log.Printf("Creating poll: title=%s, anonymous=%v", input.Title, isAnonymous)
	c.Redirect(http.StatusSeeOther, "/my-polls")
}

func (h *PollHandler) RenderMyPolls(c *gin.Context) {
	uid, exists := c.Get("uid")
	log.Printf("RenderMyPolls: uid=%v, exists=%v", uid, exists)
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login?expired=true")
		return
	}

	var role string
	role, err := h.AuthService.GetUserRole(c.Request.Context(), uid.(string))
	if err != nil {
		log.Printf("Failed to fetch role: %v", err)
	}

	polls, err := h.PollService.GetUserPolls(c.Request.Context(), uid.(string))
	if err != nil {
		log.Printf("Failed to fetch polls: %v", err)
		c.HTML(http.StatusInternalServerError, "my_polls.html", gin.H{
			"Error":     "Could not load polls",
			"CSRFToken": c.GetString("csrf_token"),
		})
		return
	}

	pollData := make([]struct {
		Poll    models.Poll
		Options []models.Option
	}, len(polls))
	for i, poll := range polls {
		pollData[i].Poll = poll
		if poll.QuestionType != "text" {
			options, err := h.PollService.GetPollOptions(c.Request.Context(), poll.ID)
			if err != nil {
				log.Printf("Failed to fetch options for poll %d: %v", poll.ID, err)
				pollData[i].Options = nil
			} else {
				pollData[i].Options = options
			}
		}
	}

	csrfToken, _ := c.Get("csrf_token")
	c.HTML(http.StatusOK, "my_polls.html", gin.H{
		"Title":     "My Polls",
		"Polls":     pollData,
		"CSRFToken": csrfToken,
		"Role":      role,
	})
}

func (h *PollHandler) RenderEditPoll(c *gin.Context) {
	pollIDStr := c.Param("id")
	pollID, err := strconv.ParseInt(pollIDStr, 10, 64)
	if err != nil {
		log.Printf("Invalid poll ID: %v", err)
		c.HTML(http.StatusBadRequest, "home.html", gin.H{"Error": "Invalid poll ID"})
		return
	}
	log.Printf("Rendering edit poll page for poll ID: %d", pollID)

	poll, err := h.PollService.GetPoll(c.Request.Context(), pollID)
	if err != nil {
		log.Printf("Poll not found: %v", err)
		c.HTML(http.StatusNotFound, "home.html", gin.H{"Error": "Poll not found"})
		return
	}

	uid, exists := c.Get("uid")
	if !exists {
		log.Println("User not logged in")
		c.HTML(http.StatusUnauthorized, "home.html", gin.H{"Error": "Please log in"})
		return
	}
	if poll.UserID != uid.(string) {
		log.Printf("Unauthorized access attempt by user %s for poll %d", uid.(string), pollID)
		c.HTML(http.StatusForbidden, "home.html", gin.H{"Error": "Unauthorized"})
		return
	}

	var role string
	err = h.DB.QueryRow("SELECT role FROM users WHERE id = $1", uid.(string)).Scan(&role)
	if err != nil || role != "admin" {
		log.Printf("Unauthorized: User %s is not admin (role: %s, err: %v)", uid, role, err)
		c.HTML(http.StatusForbidden, "home.html", gin.H{
			"Error": "Admin access required",
		})
		return
	}

	options, err := h.PollService.GetPollOptions(c.Request.Context(), pollID)
	if err != nil {
		log.Printf("Failed to fetch options for poll %d: %v", pollID, err)
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusInternalServerError, "edit_xpoll.html", gin.H{
			"Title":     "Edit Poll",
			"Error":     "Could not load poll options",
			"Poll":      poll,
			"CSRFToken": csrfToken,
		})
		return
	}

	csrfToken, _ := c.Get("csrf_token")
	log.Println("Rendering editpoll.html")
	c.HTML(http.StatusOK, "edit_poll.html", gin.H{
		"Title":     "Edit Poll",
		"Poll":      poll,
		"Options":   options,
		"CSRFToken": csrfToken,
	})
}

func (h *PollHandler) UpdatePoll(c *gin.Context) {
	pollIDStr := c.Param("id")
	pollID, err := strconv.ParseInt(pollIDStr, 10, 64)
	if err != nil {
		log.Printf("Invalid poll ID: %v", err)
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusBadRequest, "edit_poll.html", gin.H{
			"Title":     "Edit Poll",
			"Error":     "Invalid poll ID",
			"CSRFToken": csrfToken,
		})
		return
	}

	poll, err := h.PollService.GetPoll(c.Request.Context(), pollID)
	if err != nil {
		log.Printf("Poll not found: %v", err)
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusNotFound, "edit_poll.html", gin.H{
			"Title":     "Edit Poll",
			"Error":     "Poll not found",
			"CSRFToken": csrfToken,
		})
		return
	}

	uid, exists := c.Get("uid")
	if !exists {
		log.Println("User not logged in")
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusUnauthorized, "edit_poll.html", gin.H{
			"Title":     "Edit Poll",
			"Error":     "Please log in",
			"Poll":      poll,
			"CSRFToken": csrfToken,
		})
		return
	}
	if poll.UserID != uid.(string) {
		log.Printf("Unauthorized access attempt by user %s for poll %d", uid.(string), pollID)
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusForbidden, "edit_poll.html", gin.H{
			"Title":     "Edit Poll",
			"Error":     "Unauthorized",
			"Poll":      poll,
			"CSRFToken": csrfToken,
		})
		return
	}

	var role string
	err = h.DB.QueryRow("SELECT role FROM users WHERE id = $1", uid.(string)).Scan(&role)
	if err != nil || role != "admin" {
		log.Printf("Unauthorized: User %s is not admin (role: %s, err: %v)", uid, role, err)
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusForbidden, "edit_poll.html", gin.H{
			"Title":     "Edit Poll",
			"Error":     "Admin access required",
			"Poll":      poll,
			"CSRFToken": csrfToken,
		})
		return
	}

	var input struct {
		Title        string   `form:"title" binding:"required"`
		QuestionType string   `form:"question_type" binding:"required,oneof=single_choice multiple_choice scale text"`
		Options      []string `form:"options[]"`
		StartDate    string   `form:"start_date" binding:"required"`
		EndDate      string   `form:"end_date"`
		IsAnonymous  string   `form:"is_anonymous"`
	}

	if err := c.ShouldBind(&input); err != nil {
		log.Printf("Form binding error: %v, Form data: %+v", err, c.Request.PostForm)
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusBadRequest, "edit_poll.html", gin.H{
			"Title":     "Edit Poll",
			"Error":     "Please fill all required fields",
			"Input":     input,
			"Poll":      poll,
			"Options":   input.Options,
			"CSRFToken": csrfToken,
		})
		return
	}

	isAnonymous := input.IsAnonymous == "on"
	log.Printf("Checkbox value: %q → anonymous = %v", input.IsAnonymous, isAnonymous)

	var options []string
	if input.QuestionType == "single_choice" || input.QuestionType == "multiple_choice" {
		hasValidOption := false
		for _, opt := range input.Options {
			if trimmed := strings.TrimSpace(opt); trimmed != "" {
				options = append(options, trimmed)
				hasValidOption = true
			}
		}
		if !hasValidOption {
			log.Printf("No valid options provided for %s poll", input.QuestionType)
			csrfToken, _ := c.Get("csrf_token")
			c.HTML(http.StatusBadRequest, "edit_poll.html", gin.H{
				"Title":     "Edit Poll",
				"Error":     "At least one non-empty option is required for single/multiple choice polls",
				"Input":     input,
				"Poll":      poll,
				"Options":   input.Options,
				"CSRFToken": csrfToken,
			})
			return
		}
	} else if input.QuestionType == "scale" {
		//hasValidOption := false
		for _, opt := range input.Options {
			if trimmed := strings.TrimSpace(opt); trimmed != "" {
				options = append(options, trimmed)
				//hasValidOption = true
			}
		}

	}

	startDate, err := time.Parse("2006-01-02T15:04", input.StartDate)
	if err != nil {
		startDate, err = time.Parse("2006-01-02T15:04:05", input.StartDate+":00")
		if err != nil {
			log.Printf("Invalid start date: %v, Input: %s", err, input.StartDate)
			csrfToken, _ := c.Get("csrf_token")
			c.HTML(http.StatusBadRequest, "edit_poll.html", gin.H{
				"Title":     "Edit Poll",
				"Error":     "Invalid start date format",
				"Input":     input,
				"Poll":      poll,
				"Options":   input.Options,
				"CSRFToken": csrfToken,
			})
			return
		}
	}

	var endDate *time.Time
	if input.EndDate != "" {
		t, err := time.Parse("2006-01-02T15:04", input.EndDate)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05", input.EndDate+":00")
			if err != nil {
				log.Printf("Invalid end date: %v, Input: %s", err, input.EndDate)
				csrfToken, _ := c.Get("csrf_token")
				c.HTML(http.StatusBadRequest, "edit_poll.html", gin.H{
					"Title":     "Edit Poll",
					"Error":     "Invalid end date format",
					"Input":     input,
					"Poll":      poll,
					"Options":   input.Options,
					"CSRFToken": csrfToken,
				})
				return
			}
		}
		endDate = &t
	}

	err = h.PollService.UpdatePoll(c.Request.Context(), pollID, input.Title, input.QuestionType, options, startDate, endDate, isAnonymous, uid.(string))
	if err != nil {
		log.Printf("Poll update failed: %v", err)
		csrfToken, _ := c.Get("csrf_token")
		c.HTML(http.StatusInternalServerError, "edit_poll.html", gin.H{
			"Title":     "Edit Poll",
			"Error":     "Could not update poll",
			"Input":     input,
			"Poll":      poll,
			"Options":   input.Options,
			"CSRFToken": csrfToken,
		})
		return
	}
	log.Printf("Updating poll: title=%s, anonymous=%v", input.Title, isAnonymous)

	c.Redirect(http.StatusSeeOther, "/my-polls")
}

func (h *PollHandler) DeletePoll(c *gin.Context) {
	pollIDStr := c.Param("id")
	pollID, err := strconv.ParseInt(pollIDStr, 10, 64)
	if err != nil {
		log.Printf("Invalid poll ID: %v", err)
		c.HTML(http.StatusBadRequest, "my_polls.html", gin.H{
			"Error":     "Invalid poll ID",
			"CSRFToken": c.GetString("csrf_token"),
		})
		return
	}

	uid, exists := c.Get("uid")
	if !exists {
		c.HTML(http.StatusUnauthorized, "my_polls.html", gin.H{
			"Error":     "Please log in",
			"CSRFToken": c.GetString("csrf_token"),
		})
		return
	}

	var role string
	err = h.DB.QueryRow("SELECT role FROM users WHERE id = $1", uid.(string)).Scan(&role)
	if err != nil || role != "admin" {
		log.Printf("Unauthorized: User %s is not admin (role: %s, err: %v)", uid, role, err)
		c.HTML(http.StatusForbidden, "my_polls.html", gin.H{
			"Error":     "Admin access required",
			"CSRFToken": c.GetString("csrf_token"),
		})
		return
	}

	err = h.PollService.DeletePoll(c.Request.Context(), pollID, uid.(string))
	if err != nil {
		log.Printf("Poll deletion failed: %v", err)
		c.HTML(http.StatusBadRequest, "my_polls.html", gin.H{
			"Error":     "Could not delete poll",
			"CSRFToken": c.GetString("csrf_token"),
		})
		return
	}
	c.Redirect(http.StatusSeeOther, "/my-polls")
}

func (h *PollHandler) RenderPollsList(c *gin.Context) {
	uid, exists := c.Get("uid")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login?expired=true")
		return
	}

	polls, err := h.PollService.GetAllPolls(c.Request.Context())
	if err != nil {
		log.Printf("Failed to fetch polls: %v", err)
		c.HTML(http.StatusInternalServerError, "polls_list.html", gin.H{
			"Title": "Polls List",
			"Error": "Could not load polls",
		})
		return
	}

	role, err := h.AuthService.GetUserRole(c.Request.Context(), uid.(string))
	if err != nil {
		log.Printf("Failed to fetch role: %v", err)
	}

	c.HTML(http.StatusOK, "polls_list.html", gin.H{
		"Title": "Polls List",
		"Polls": polls,
		"Role":  role,
	})
}
