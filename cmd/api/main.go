package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Set up Gin in release mode for cleaner logs
	gin.SetMode(gin.ReleaseMode)

	// 2. Create a new router with default middleware (logger and recovery)
	router := gin.Default()

	// 3. Define the one and only health check endpoint
	//
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// 4. Start the server on port 8080
	// We'll hardcode the port for now. We can use a config file later.
	const serverAddress = ":8080"

	// A simple print statement. We'll add a real logger in the next step.
	println("Server starting on http://localhost" + serverAddress)

	// ListenAndServe starts the server
	if err := router.Run(serverAddress); err != nil {
		// If it fails, print the error and exit
		println("Error starting server:", err.Error())
	}
}
