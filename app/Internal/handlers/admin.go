package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"fakidoosuurdoris/app/Internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
)

type AdminHandler struct {
	PollService *services.PollService
	AuthService *services.AuthService
	Templates   *template.Template
}

func NewAdminHandler(pollService *services.PollService, authService *services.AuthService, templates *template.Template) *AdminHandler {
	return &AdminHandler{
		PollService: pollService,
		AuthService: authService,
		Templates:   templates,
	}
}

func (h *AdminHandler) RenderAdminDashboard(c *gin.Context) {
	c.Redirect(http.StatusSeeOther, "/admin/polls")
}

func (h *AdminHandler) RenderAdminPolls(c *gin.Context) {
	uid, exists := c.Get("uid")
	if !exists {
		c.HTML(http.StatusUnauthorized, "admin_polls.html", gin.H{
			"Title": "Poll Details",
			"Error": "Please log in",
		})
		return
	}
	userID := uid.(string)

	role, err := h.AuthService.GetUserRole(c.Request.Context(), userID)
	if err != nil || role != "admin" {
		c.HTML(http.StatusForbidden, "admin_polls.html", gin.H{
			"Title": "Poll Details",
			"Error": "Access denied",
		})
		return
	}

	polls, err := h.PollService.GetAllPolls(c.Request.Context())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin_polls.html", gin.H{
			"Title": "Poll Details",
			"Error": "Failed to load polls",
		})
		return
	}

	csrfToken, _ := c.Get("csrf_token")
	c.HTML(http.StatusOK, "admin_polls.html", gin.H{
		"Title":     "Poll Details",
		"Polls":     polls,
		"CSRFToken": csrfToken,
		"Role":      role,
	})
}

func (h *AdminHandler) RenderAdminUsers(c *gin.Context) {
	uid, exists := c.Get("uid")
	if !exists {
		c.HTML(http.StatusUnauthorized, "admin_users.html", gin.H{
			"Title": "User Details",
			"Error": "Please log in",
		})
		return
	}
	userID := uid.(string)

	role, err := h.AuthService.GetUserRole(c.Request.Context(), userID)
	if err != nil || role != "admin" {
		c.HTML(http.StatusForbidden, "admin_users.html", gin.H{
			"Title": "User Details",
			"Error": "Access denied",
		})
		return
	}

	users, err := h.AuthService.GetAllUsers(c.Request.Context())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin_users.html", gin.H{
			"Title": "User Details",
			"Error": "Failed to load users",
		})
		return
	}

	csrfToken, _ := c.Get("csrf_token")
	c.HTML(http.StatusOK, "admin_users.html", gin.H{
		"Title":     "User Details",
		"Users":     users,
		"CSRFToken": csrfToken,
		"Role":      role,
	})
}

func (h *AdminHandler) RenderMakeAdmin(c *gin.Context) {
	uid, exists := c.Get("uid")
	if !exists {
		c.HTML(http.StatusUnauthorized, "make_admin.html", gin.H{
			"Title": "Assign Admin Role",
			"Error": "Please log in",
		})
		return
	}
	userID := uid.(string)

	role, err := h.AuthService.GetUserRole(c.Request.Context(), userID)
	if err != nil || role != "admin" {
		c.HTML(http.StatusForbidden, "make_admin.html", gin.H{
			"Title": "Assign Admin Role",
			"Error": "Access denied",
		})
		return
	}

	csrfToken, _ := c.Get("csrf_token")
	c.HTML(http.StatusOK, "make_admin.html", gin.H{
		"Title":     "Assign Admin Role",
		"CSRFToken": csrfToken,
		"Role":      role,
	})
}

func (h *AdminHandler) GetPollSummary(c *gin.Context) {
	uid, exists := c.Get("uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Please log in"})
		return
	}
	userID := uid.(string)

	role, err := h.AuthService.GetUserRole(c.Request.Context(), userID)
	if err != nil || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	pollID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid poll ID"})
		return
	}

	summary, err := h.PollService.GetPollSummary(c.Request.Context(), pollID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load summary: %v", err)})
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *AdminHandler) DownloadPollSummary(c *gin.Context) {
	uid, exists := c.Get("uid")
	if !exists {
		c.String(http.StatusUnauthorized, "Please log in")
		return
	}
	userID := uid.(string)

	role, err := h.AuthService.GetUserRole(c.Request.Context(), userID)
	if err != nil || role != "admin" {
		c.String(http.StatusForbidden, "Access denied")
		return
	}

	pollID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid poll ID")
		return
	}

	format := c.Query("format")
	if format != "csv" && format != "json" && format != "pdf" {
		c.String(http.StatusBadRequest, "Invalid format. Use csv, json, or pdf")
		return
	}

	summary, err := h.PollService.GetPollSummary(c.Request.Context(), pollID)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to load summary: %v", err))
		return
	}

	if format == "csv" {
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=poll_%d_summary.csv", pollID))

		w := csv.NewWriter(c.Writer)
		defer w.Flush()

		w.Write([]string{"Poll ID", "Title", "Question Type", "Is Anonymous"})
		w.Write([]string{strconv.FormatInt(summary.PollID, 10), summary.Title, summary.QuestionType, strconv.FormatBool(summary.IsAnonymous)})

		w.Write([]string{"", "", "", ""})
		w.Write([]string{"Voters"})
		w.Write([]string{"User ID", "Email"})
		for _, voter := range summary.Voters {
			var email string
			if voter.Email == "" {
				email = "Anonymous"
			} else {
				email = voter.Email
			}
			w.Write([]string{voter.UserID, email})
		}

		w.Write([]string{"", "", "", ""})
		w.Write([]string{"Results"})
		if summary.QuestionType == "text" {
			w.Write([]string{"Text Answer", "Count"})
			for text, count := range summary.Results.(map[string]int) {
				w.Write([]string{text, strconv.Itoa(count)})
			}
		} else if summary.QuestionType == "scale" {
			w.Write([]string{"Scale Value", "Count"})
			for value, count := range summary.Results.(map[int]int) {
				w.Write([]string{strconv.Itoa(value), strconv.Itoa(count)})
			}
		} else {
			w.Write([]string{"Option", "Count"})
			for option, count := range summary.Results.(map[string]int) {
				w.Write([]string{option, strconv.Itoa(count)})
			}
		}
	} else if format == "json" {
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=poll_%d_summary.json", pollID))
		enc := json.NewEncoder(c.Writer)
		enc.SetIndent("", "  ")
		enc.Encode(summary)
	} else if format == "pdf" {
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 16)
		pdf.Cell(40, 10, fmt.Sprintf("Poll Summary: %s", summary.Title))
		pdf.Ln(10)
		pdf.SetFont("Arial", "", 12)
		pdf.Cell(40, 10, fmt.Sprintf("Poll ID: %d", summary.PollID))
		pdf.Ln(10)
		pdf.Cell(40, 10, fmt.Sprintf("Question Type: %s", summary.QuestionType))
		pdf.Ln(10)
		pdf.Cell(40, 10, fmt.Sprintf("Anonymous: %v", summary.IsAnonymous))
		pdf.Ln(10)

		pdf.Ln(5)
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(40, 10, "Voters")
		pdf.Ln(10)
		pdf.SetFont("Arial", "", 12)
		for _, voter := range summary.Voters {
			email := voter.Email
			if email == "" {
				email = "Anonymous"
			}
			pdf.Cell(40, 10, fmt.Sprintf("User ID: %s, Email: %s", voter.UserID, email))
			pdf.Ln(10)
		}

		pdf.Ln(5)
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(40, 10, "Results")
		pdf.Ln(10)
		pdf.SetFont("Arial", "", 12)
		if summary.QuestionType == "text" {
			for text, count := range summary.Results.(map[string]int) {
				pdf.Cell(40, 10, fmt.Sprintf("Answer: %s, Count: %d", text, count))
				pdf.Ln(10)
			}
		} else if summary.QuestionType == "scale" {
			for value, count := range summary.Results.(map[int]int) {
				pdf.Cell(40, 10, fmt.Sprintf("Scale: %d, Count: %d", value, count))
				pdf.Ln(10)
			}
		} else {
			for option, count := range summary.Results.(map[string]int) {
				pdf.Cell(40, 10, fmt.Sprintf("Option: %s, Count: %d", option, count))
				pdf.Ln(10)
			}
		}

		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=poll_%d_summary.pdf", pollID))
		pdf.Output(c.Writer)
	}
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	uid, exists := c.Get("uid")
	if !exists {
		c.HTML(http.StatusUnauthorized, "admin_users.html", gin.H{
			"Title": "User Details",
			"Error": "Please log in",
		})
		return
	}
	adminID := uid.(string)

	role, err := h.AuthService.GetUserRole(c.Request.Context(), adminID)
	if err != nil || role != "admin" {
		c.HTML(http.StatusForbidden, "admin_users.html", gin.H{
			"Title": "User Details",
			"Error": "Access denied",
		})
		return
	}

	userID := c.Param("id")
	if userID == adminID {
		c.HTML(http.StatusBadRequest, "admin_users.html", gin.H{
			"Title":         "User Details",
			"Error":         "Cannot delete your own account",
			"Role":          role,
			"CurrentUserID": adminID,
		})
		return
	}

	err = h.AuthService.DeleteUser(c.Request.Context(), userID, adminID)
	if err != nil {
		log.Printf("User deletion failed: %v", err)
		c.HTML(http.StatusBadRequest, "admin_users.html", gin.H{
			"Title":         "User Details",
			"Error":         fmt.Sprintf("Could not delete user: %v", err),
			"Role":          role,
			"CurrentUserID": adminID,
		})
		return
	}

	users, err := h.AuthService.GetAllUsers(c.Request.Context())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin_users.html", gin.H{
			"Title":         "User Details",
			"Error":         "Failed to load users",
			"Role":          role,
			"CurrentUserID": adminID,
		})
		return
	}

	csrfToken, _ := c.Get("csrf_token")
	c.HTML(http.StatusOK, "admin_users.html", gin.H{
		"Title":         "User Details",
		"Message":       "User deleted successfully",
		"Users":         users,
		"CSRFToken":     csrfToken,
		"Role":          role,
		"CurrentUserID": adminID,
	})
}
