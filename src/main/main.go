package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"github.com/gorilla/websocket"
	"os"
	"os/signal"
	"time"
)

var (
	router *gin.Engine

	// stuff
)

var wsupgrader = websocket.Upgrader {
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}


const (
	RootFolder = "."
)

func main() {
	log.Println("Starting northhacking2...")

	router = gin.Default()

	// setup routes

	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.LoadHTMLGlob(RootFolder + "/src/assets/html/*")
	router.Static("/images", RootFolder+ "/assets/images")
	router.GET("/", func (c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			//"flowTotal": totalVol,
			//"flowRate": flowVol,
		})
	})

	router.GET("/session", func (c *gin.Context) {

	})

	router.GET("/stream-audio", func(c *gin.Context) {
		audioReceive(c.Writer, c.Request)
	})

	srv := &http.Server {
		Addr: ":3001",
		Handler: router,
	}

	go func () {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen %s\n", err)
		}
	}()

	// listen for sigint to shutdown gracefully
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutting down SHAD2019-Sulfur...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server shutdown: ", err)
	}
	log.Println("goodbye, northhacking.")

}

func speechToText(b []byte) {

}

func audioReceive(w http.ResponseWriter, r *http.Request) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Failed to set websocket upgrade: %+v", err)
		return
	}

	for {
		_, r, err := conn.ReadMessage()
		log.Println("Received audio!")
		if err != nil {
			log.Println(err)
			return
		}
		go speechToText(r)
	}
}