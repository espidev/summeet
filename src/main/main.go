package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"github.com/gorilla/websocket"
	"os"
	"os/signal"
	"strings"
	"time"

	speech "cloud.google.com/go/speech/apiv1"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"
)

var (
	router *gin.Engine

	// stuff
	rawText string // no names attached
	liveChatHtml string
	liveSummaryHtml string
)

var wsupgrader = websocket.Upgrader {
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

const (
	RootFolder = "."
)

func main() {
	ctx := context.Background()

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
		c.HTML(http.StatusOK, "meetings.html", gin.H{})
	})

	router.GET("/live-chat", func (c *gin.Context) {
		c.String(http.StatusOK, /*liveChatHtml*/ rawText)
	})

	router.GET("/live-summary", func (c *gin.Context) {
		c.String(http.StatusOK, /*liveSummaryHtml*/ rawText)
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
	log.Println("Shutting down northhacking...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server shutdown: ", err)
	}
	log.Println("goodbye, northhacking.")

}

func speechToText(user string, b []byte) {
	log.Println("Sending to google...") // TODO

	ctx := context.Background()

	client, err := speech.NewClient(ctx)
	if err != nil {
		log.Println("Failed to create google speech client: %v", err)
		return
	}

	resp, err := client.Recognize(ctx, &speechpb.RecognizeRequest{
		Config: &speechpb.RecognitionConfig{
			Encoding:                            speechpb.RecognitionConfig_OGG_OPUS,
			SampleRateHertz:                     16000,
			LanguageCode:                        "en-US",
			EnableAutomaticPunctuation:          true,
		},
		Audio: &speechpb.RecognitionAudio {
			AudioSource: &speechpb.RecognitionAudio_Content{Content: b},
		},
	})

	if err != nil {
		log.Println("Failed to create google speech client: %v", err)
		return
	}

	log.Println("Received from google...")

	var confidence float32
	var transcript string

	for _, result := range resp.Results {
		for _, alt := range result.Alternatives {
			if confidence < alt.Confidence {
				confidence = alt.Confidence
				transcript = alt.Transcript
			}
			fmt.Printf("\"%v\" (confidence=%3f)\n", alt.Transcript, alt.Confidence)
		}
	}

	rawText += " " + transcript
	liveSummaryHtml = getSummary(rawText)
}

// use http basic auth (username:password@localhost:3001)
func audioReceive(w http.ResponseWriter, hr *http.Request) {
	wsupgrader.CheckOrigin = func(r *http.Request) bool {return true}
	conn, err := wsupgrader.Upgrade(w, hr, nil)
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

		// get basic auth username
		s := strings.SplitN(hr.Header.Get("Authorization"), " ", 2)
		if len(s) != 2 {
			http.Error(w, "Not authorized", 401)
			return
		}

		b, err := base64.StdEncoding.DecodeString(s[1])
		if err != nil {
			http.Error(w, err.Error(), 401)
			return
		}

		pair := strings.SplitN(string(b), ":", 2)
		if len(pair) != 2 {
			http.Error(w, "Not authorized", 401)
			return
		}

		go speechToText(pair[0], r)
	}
}