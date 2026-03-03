package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ListQuery represents query parameters for list endpoints.
type ListQuery struct {
	Page   int    `form:"page"  binding:"required"`
	Limit  int    `form:"limit"`
	Search string `form:"search"`
}

// AuthHeaders represents custom headers for authentication context.
type AuthHeaders struct {
	TenantID  string `header:"X-Tenant-ID" binding:"required"`
	RequestID string `header:"X-Request-ID"`
}

func main() {
	r := gin.Default()
	r.GET("/products", ListProducts)
	r.Run()
}

// ListProducts returns a paginated list of products.
func ListProducts(c *gin.Context) {
	var q ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var h AuthHeaders
	if err := c.ShouldBindHeader(&h); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"page": q.Page})
}
