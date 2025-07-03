package handlers

import (
	"fakidoosuurdoris/app/Internal/models"
	"fakidoosuurdoris/app/Internal/services"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type VoteHandler struct {
	PollService *services.PollService
	AuthService *services.AuthService
	Templates   *template.Template
}

func NewVoteHandler(pollService *services.PollService, authService *services.AuthService, templates *template.Template) *VoteHandler {
	return &VoteHandler{
		PollService: pollService,
		AuthService: authService,
		Templates:   templates,
	}
}

func (h *VoteHandler) RenderVote(c *gin.Context) {
	uid, exists := c.Get("uid")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login?expired=true")
		return
	}

	pollID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "vote.html", gin.H{
			"Title": "Vote",
			"Error": "Invalid poll ID",
		})
		return
	}

	poll, err := h.PollService.GetPoll(c.Request.Context(), pollID)
	if err != nil {
		c.HTML(http.StatusNotFound, "vote.html", gin.H{
			"Title": "Vote",
			"Error": "Poll not found",
		})
		return
	}

	now := time.Now()
	if poll.StartDate.After(now) || (poll.EndDate != nil && poll.EndDate.Before(now)) {
		c.HTML(http.StatusBadRequest, "vote.html", gin.H{
			"Title": "Vote",
			"Error": "Poll is not active",
		})
		return
	}

	hasVoted, err := h.PollService.HasVoted(c.Request.Context(), pollID, uid.(string))
	if err != nil {
		c.HTML(http.StatusInternalServerError, "vote.html", gin.H{
			"Title": "Vote",
			"Error": "Could not verify vote status",
		})
		return
	}
	if hasVoted {
		c.HTML(http.StatusForbidden, "vote.html", gin.H{
			"Title": "Vote",
			"Error": "You have already voted",
		})
		return
	}

	if !poll.IsAnonymous {
		hasVoted, err := h.PollService.HasVoted(c.Request.Context(), pollID, uid.(string))
		if err != nil {
			c.HTML(http.StatusInternalServerError, "vote.html", gin.H{
				"Title": "Vote",
				"Error": "Could not verify vote status",
			})
			return
		}
		if hasVoted {
			c.HTML(http.StatusForbidden, "vote.html", gin.H{
				"Title": "Vote",
				"Error": "You have already voted",
			})
			return
		}
	}

	var options []models.Option
	if poll.QuestionType != "text" {
		options, err = h.PollService.GetPollOptions(c.Request.Context(), pollID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "vote.html", gin.H{
				"Title": "Vote",
				"Error": "Could not load poll options",
			})
			return
		}
	}

	role, err := h.AuthService.GetUserRole(c.Request.Context(), uid.(string))
	if err != nil {
		log.Printf("Failed to fetch role: %v", err)
		role = ""
	}

	csrfToken, _ := c.Get("csrf_token")
	c.HTML(http.StatusOK, "vote.html", gin.H{
		"Title":     "Vote",
		"Poll":      poll,
		"Options":   options,
		"CSRFToken": csrfToken,
		"Role":      role,
	})
}

func (h *VoteHandler) Vote(c *gin.Context) {
	uid, exists := c.Get("uid")
	if !exists {
		c.HTML(http.StatusUnauthorized, "vote.html", gin.H{
			"Title": "Vote",
			"Error": "Please log in",
		})
		return
	}

	pollID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "vote.html", gin.H{
			"Title": "Vote",
			"Error": "Invalid poll ID",
		})
		return
	}

	poll, err := h.PollService.GetPoll(c.Request.Context(), pollID)
	if err != nil {
		c.HTML(http.StatusNotFound, "vote.html", gin.H{
			"Title": "Vote",
			"Error": "Poll not found",
		})
		return
	}

	var input struct {
		OptionIDs  []int64 `form:"option_ids[]"`
		TextAnswer string  `form:"text_answer"`
	}
	if err := c.ShouldBind(&input); err != nil {
		c.HTML(http.StatusBadRequest, "vote.html", gin.H{
			"Title": "Vote",
			"Poll":  poll,
			"Error": "Invalid vote data",
		})
		return
	}

	if poll.QuestionType == "single_choice" && len(input.OptionIDs) != 1 {
		c.HTML(http.StatusBadRequest, "vote.html", gin.H{
			"Title": "Vote",
			"Poll":  poll,
			"Error": "Select exactly one option",
		})
		return
	}
	if poll.QuestionType == "multiple_choice" && len(input.OptionIDs) == 0 {
		c.HTML(http.StatusBadRequest, "vote.html", gin.H{
			"Title": "Vote",
			"Poll":  poll,
			"Error": "Select at least one option",
		})
		return
	}
	if poll.QuestionType == "scale" && (len(input.OptionIDs) != 1 || input.OptionIDs[0] < 1 || input.OptionIDs[0] > 5) {
		c.HTML(http.StatusBadRequest, "vote.html", gin.H{
			"Title": "Vote",
			"Poll":  poll,
			"Error": "Select a scale value between 1 and 5",
		})
		return
	}
	if poll.QuestionType == "text" && input.TextAnswer == "" {
		c.HTML(http.StatusBadRequest, "vote.html", gin.H{
			"Title": "Vote",
			"Poll":  poll,
			"Error": "Text response cannot be empty",
		})
		return
	}

	err = h.PollService.RecordVote(c.Request.Context(), pollID, uid.(string), input.OptionIDs, input.TextAnswer)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "vote.html", gin.H{
			"Title": "Vote",
			"Poll":  poll,
			"Error": "Failed to record vote: " + err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/polls-list")
}
