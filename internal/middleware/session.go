package middleware

import (
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func SessionTimeout() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		lastActivity := session.Get("last_activity")

		if lastActivity != nil {
			lastActivityTime := lastActivity.(int64)
			if time.Now().Unix()-lastActivityTime > 5*60 {
				session.Clear()
				session.Save()
				c.Redirect(http.StatusFound, "/signInPage")
				c.Abort()
				return
			}
		}

		session.Set("last_activity", time.Now().Unix())
		session.Save()
		c.Next()

	}
}
