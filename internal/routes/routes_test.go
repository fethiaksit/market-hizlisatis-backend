package routes

import (
	"testing"

	"github.com/gin-gonic/gin"

	"hizlisatis-backend/internal/config"
)

func TestSetupDoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("route setup panicked: %v", r)
		}
	}()

	Setup(nil, config.Config{Port: "8082", JWTSecret: "test"})
}
