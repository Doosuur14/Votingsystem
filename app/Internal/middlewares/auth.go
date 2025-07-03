package middlewares

import (
	"context"
	"log"
	"net/http"

	"firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func AuthMiddleware(authClient *auth.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		idToken, err := c.Cookie("idToken")
		if err != nil {
			log.Println("No idToken cookie found")
			if c.Request.URL.Path == "/polls" {
				c.Redirect(http.StatusSeeOther, "/login?expired=true")
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Please log in"})
			}
			c.Abort()
			return
		}

		token, err := authClient.VerifyIDToken(context.Background(), idToken)
		if err != nil {
			log.Printf("Invalid token: %v", err)
			c.SetCookie("idToken", "", -1, "/", "localhost", false, true) // Clear expired token
			if c.Request.URL.Path == "/polls" {
				c.Redirect(http.StatusSeeOther, "/login?expired=true")
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token, please log in again"})
			}
			c.Abort()
			return
		}

		c.Set("uid", token.UID)
		c.Next()
	}
}

func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		csrfToken, err := c.Cookie("csrf_token")
		if err != nil || csrfToken == "" {
			csrfTokenStr := uuid.New().String()
			c.SetCookie("csrf_token", csrfTokenStr, 3600, "/", "localhost", false, true)
			c.Set("csrf_token", csrfTokenStr)
		} else {
			c.Set("csrf_token", csrfToken)
		}

		if c.Request.Method == "POST" && c.Request.URL.Path != "/login" && c.Request.URL.Path != "/register" {
			formToken := c.PostForm("csrf_token")
			cookieToken, err := c.Cookie("csrf_token")
			log.Printf("CSRF check: Form token=%s, Cookie token=%s, Error=%v", formToken, cookieToken, err)
			if err != nil || formToken == "" || formToken != cookieToken {
				log.Println("Invalid CSRF token")
				if c.Request.URL.Path == "/polls" {
					csrfTokenStr := cookieToken
					if csrfTokenStr == "" {
						csrfTokenStr = uuid.New().String()
						c.SetCookie("csrf_token", csrfTokenStr, 3600, "/", "localhost", false, true)
					}
					c.HTML(http.StatusBadRequest, "createpolls.html", gin.H{
						"Title":     "Create Poll",
						"Error":     "Invalid CSRF token, please try again",
						"CSRFToken": csrfTokenStr,
					})
				} else {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CSRF token"})
				}
				c.Abort()
				return
			}
			log.Println("CSRF token valid")
		}
	}
}
