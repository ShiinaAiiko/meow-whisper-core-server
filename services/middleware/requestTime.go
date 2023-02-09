package middleware

import (
	"github.com/cherrai/nyanyago-utils/nlog"
	"github.com/gin-gonic/gin"
)

var (
	log = nlog.New()
)

func RequestTime() gin.HandlerFunc {
	return func(c *gin.Context) {
		lt := log.Time()
		c.Next()
		lt.TimeEnd(c.Request.URL.Path + ", Request Time =>")
	}
}
