package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Authenticate(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"message": "authentication failed",
	})
}
