package ticker

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TickController struct {
	Scheduler *TickScheduler
}

func NewTickController(scheduler *TickScheduler) *TickController {
	return &TickController{Scheduler: scheduler}
}

func (tc *TickController) ConfigureEngine(r *gin.Engine) error {
	r.POST("/tick", tc.CreateTick)
	r.GET("/healthy", tc.Healthy)
	return nil
}

func (tc *TickController) CreateTick(c *gin.Context) {
	var inputTick TickDTO

	if err := c.ShouldBindJSON(&inputTick); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	//log.Printf("http tick received %+v\n", inputTick)
	err := tc.Scheduler.ScheduleTickAt(context.Background(), inputTick.Item, inputTick.At)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	c.JSON(http.StatusOK, gin.H{})
}

func (tc *TickController) Healthy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}
