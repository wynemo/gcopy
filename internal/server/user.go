package server

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mileusna/useragent"
	"gopkg.in/gomail.v2"
)

// shareCodeStore stores share codes with their expiration time
// key: code (string), value: expiresAt (time.Time)
var shareCodeStore sync.Map

const userSessionName = "user_session"

type Login struct {
	Email string `form:"email" json:"email" xml:"email"  binding:"required,email"`
	Code  string `form:"code" json:"code" xml:"password" binding:"omitempty,numeric,len=6"`
}

type ShareCodeLogin struct {
	Code string `form:"code" json:"code" binding:"required"`
}

func (s *Server) emailCodeHandler(c *gin.Context) {
	var login Login
	if err := c.ShouldBindJSON(&login); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		return
	}

	var code string
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 6; i++ {
		code += strconv.Itoa(r.Intn(10))
	}
	ua := useragent.Parse(c.Request.Header.Get("User-Agent"))
	language := c.Request.Header.Get("Accept-Language")
	var subject, body string
	if strings.HasPrefix(language, "zh-CN") {
		subject = fmt.Sprintf("%s是您的验证码", code)
		body = fmt.Sprintf("请输入您的验证码: %s. 该验证码有效期5分钟. 为保护您的账户, 请不要分享这个验证码.", code)
		if ua.OS != "" {
			body += "<br>请求自 " + ua.OS
			if ua.Name != "" {
				body += fmt.Sprintf(" %s", ua.Name)
			}
			body += "."
		}
	} else {
		subject = fmt.Sprintf("%s is your verification code", code)
		body = fmt.Sprintf("Enter the verification code when prompted: %s. Code will expire in 5 minutes. To protect your account, do not share this code.", code)
		if ua.OS != "" {
			body += "<br>Requested from " + ua.OS
			if ua.Name != "" {
				body += fmt.Sprintf(" %s", ua.Name)
			}
			body += "."
		}
	}
	message := gomail.NewMessage()
	message.SetHeader("From", message.FormatAddress(s.config.SMTPSender, "GCopy"))
	message.SetHeader("To", login.Email)
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", body)

	dialer := gomail.NewDialer(s.config.SMTPHost, s.config.SMTPPort, s.config.SMTPUsername, s.config.SMTPPassword)
	dialer.SSL = s.config.SMTPSSL
	if err := dialer.DialAndSend(message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	session, err := s.sessionStore.Get(c.Request, userSessionName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	session.Values["email"] = login.Email
	session.Values["code"] = code
	session.Values["loggedIn"] = false
	session.Values["validateAt"] = time.Now().Unix()
	if err = session.Save(c.Request, c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Success"})
}

func (s *Server) loginHandler(c *gin.Context) {
	var login Login
	if err := c.ShouldBindJSON(&login); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		return
	}

	session, err := s.sessionStore.Get(c.Request, userSessionName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	if session.Values["loggedIn"] == true {
		c.JSON(http.StatusOK, gin.H{
			"email":    login.Email,
			"loggedIn": true,
		})
		return
	}

	if login.Email == session.Values["email"] && login.Code == session.Values["code"] && time.Now().Unix()-session.Values["validateAt"].(int64) <= 5*60 {
		session.Values["loggedIn"] = true
		session.Values["validateAt"] = time.Now().Unix()
		if err = session.Save(c.Request, c.Writer); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"email":    login.Email,
			"loggedIn": true,
		})
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
}

func (s *Server) logoutHandler(c *gin.Context) {
	session, err := s.sessionStore.Get(c.Request, userSessionName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	for key := range session.Values {
		delete(session.Values, key)
	}
	if err = session.Save(c.Request, c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Success"})
}

func (s *Server) getUserHandler(c *gin.Context) {
	session, err := s.sessionStore.Get(c.Request, userSessionName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	if session.Values["loggedIn"] != true {
		c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
		return
	}

	loginType, _ := session.Values["loginType"].(string)

	if loginType == "code" {
		shareCode, ok := session.Values["shareCode"].(string)
		if !ok || shareCode == "" {
			c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
			return
		}
		if err = session.Save(c.Request, c.Writer); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"shareCode": shareCode,
			"loggedIn":  true,
			"loginType": "code",
		})
		return
	}

	// Default to email login
	email, ok := session.Values["email"].(string)
	if !ok || email == "" {
		c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
		return
	}
	if err = session.Save(c.Request, c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"email":     email,
		"loggedIn":  true,
		"loginType": "email",
	})
}

func (s *Server) verifyAuthMiddleware(c *gin.Context) {
	session, err := s.sessionStore.Get(c.Request, userSessionName)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	if session.Values["loggedIn"] != true {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}

	var subject string
	loginType, _ := session.Values["loginType"].(string)

	if loginType == "code" {
		shareCode, ok := session.Values["shareCode"].(string)
		if !ok || shareCode == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
			return
		}
		subject = "code:" + shareCode
	} else {
		// Default to email login (backward compatible)
		email, ok := session.Values["email"].(string)
		if !ok || email == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
			return
		}
		subject = email
	}

	c.Set("subject", subject)
	c.Next()
}

func (s *Server) shareCodeLoginHandler(c *gin.Context) {
	var login ShareCodeLogin
	if err := c.ShouldBindJSON(&login); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		return
	}

	now := time.Now()
	expiresAt := now.Add(5 * time.Minute)

	// Check if code exists and is still valid
	if val, ok := shareCodeStore.Load(login.Code); ok {
		storedExpiresAt := val.(time.Time)
		if now.After(storedExpiresAt) {
			// Code expired, create new entry (new user group)
			shareCodeStore.Store(login.Code, expiresAt)
		}
		// Code exists and valid, user joins existing group
	} else {
		// Code doesn't exist, create new entry
		shareCodeStore.Store(login.Code, expiresAt)
	}

	session, err := s.sessionStore.Get(c.Request, userSessionName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	// Clear any existing email login data
	delete(session.Values, "email")
	delete(session.Values, "code")
	delete(session.Values, "validateAt")

	// Set share code login session
	session.Values["loggedIn"] = true
	session.Values["loginType"] = "code"
	session.Values["shareCode"] = login.Code
	session.Options.MaxAge = 8 * 60 * 60 // 8 hours

	if err = session.Save(c.Request, c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"shareCode": login.Code,
		"loggedIn":  true,
	})
}

func (s *Server) refreshShareCodeHandler(c *gin.Context) {
	session, err := s.sessionStore.Get(c.Request, userSessionName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	// Verify user is logged in with share code
	if session.Values["loggedIn"] != true || session.Values["loginType"] != "code" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}

	shareCode, ok := session.Values["shareCode"].(string)
	if !ok || shareCode == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}

	// Refresh the expiration time
	expiresAt := time.Now().Add(5 * time.Minute)
	shareCodeStore.Store(shareCode, expiresAt)

	c.JSON(http.StatusOK, gin.H{
		"shareCode": shareCode,
		"expiresIn": 300,
	})
}
