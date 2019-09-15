package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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

var doUpdate = make(chan string)

var wsupgrader = websocket.Upgrader {
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

const (
	RootFolder = "."
)

func main() {
	ctx := context.Background()
	start := time.Now()

	log.Println("Starting northhacking2...")

	router = gin.Default()

	// setup routes

	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.LoadHTMLGlob(RootFolder + "/src/assets/html/*")
	router.Static("/images", RootFolder+ "/assets/images")
	router.GET("/", func (c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})

	router.GET("/session", func (c *gin.Context) {
		c.HTML(http.StatusOK, "session.html", gin.H{
			"rootDomain": "localhost:3000",
		})
	})

	router.GET("/live-chat", func (c *gin.Context) {
		c.String(http.StatusOK, liveChatHtml)
	})

	router.GET("/live-summary", func (c *gin.Context) {
		c.String(http.StatusOK, liveSummaryHtml)
	})

	router.GET("/stream-audio", func(c *gin.Context) {
		newAudioReceive(c.Writer, c.Request)
	})

	router.GET("/time", func(c *gin.Context) {
		seconds := int(time.Since(start).Seconds())
		minutes := int(seconds / 60)
		seconds = int(seconds % 60)

		var sec string
		if seconds < 10 {
			sec = "0" + strconv.Itoa(seconds)
		} else {
			sec = strconv.Itoa(seconds)
		}
		c.String(http.StatusOK, strconv.Itoa(minutes) + ":" + sec)
	})

	srv := &http.Server {
		Addr: ":3000",
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

/*
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
			Encoding:                            speechpb.RecognitionConfig_LINEAR16,
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

	log.Println(resp.String())
	log.Println("Received from google! " + strconv.Itoa(len(resp.Results)))

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

	// append things
	rawText += " " + transcript
	liveChatHtml += `

				<div class="card gradient-shadow gradient-45deg-reverse z-depth-1">
                        <div class="row nunito valign-wrapper">
                            <div class="col s1"></div>
                            <div class="col s1">
                                <img src="https://previews.123rf.com/images/punphoto/punphoto1211/punphoto121100083/16291629-colorful-abstract-water-color-art-hand-paint-background.jpg" class="circle responsive-img">
                            </div>

                            <div class="col s10">
                                <div class="card-content white-text nunito" class="style=">
                                        <b>` + user + `</b>
                                        <br/>
                                        ` + transcript + `
                                </div>
                            </div>
                        </div>
                    </div>

`
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
		if err != nil {
			log.Println("Error receiving audio: " + err.Error())
			return
		}
		log.Println("Received audio!")

		//log.Printf(string(r))

		go speechToText("dude", r)
	}

}*/

func newAudioReceive(w http.ResponseWriter, hr *http.Request) {
	wsupgrader.CheckOrigin = func(r *http.Request) bool {return true}
	conn, err := wsupgrader.Upgrade(w, hr, nil)
	if err != nil {
		fmt.Println("Failed to set websocket upgrade: %+v", err)
		return
	}

	// google init
	ctx := context.Background()

	client, err := speech.NewClient(ctx)
	if err != nil {
		log.Println("Failed to create google speech client: %v", err)
		return
	}
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		log.Println("Failed to recognize stream: %v", err)
		return
	}

	log.Println("Sending initial request...")
	// Send the initial configuration message.
	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:        speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz: 16000,
					LanguageCode:    "en-US",
				},
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	log.Println("Done sending initial request.")

	// get the name of the session
	_, sr, err := conn.ReadMessage()
	if err != nil {
		log.Println("Error receiving audio: " + err.Error())
		return
	}
	user := strings.ReplaceAll(string(sr), "name=", "")
	log.Println("Got user cookie: " + user)

	go func() {
		var buf []byte
		for {
			_, r, err := conn.NextReader()
			if err != nil {
				log.Println("Reader fail: " + err.Error())
				return
			}
			buf, err = ioutil.ReadAll(r)

			if err == io.EOF {
				if err := stream.CloseSend(); err != nil {
					log.Fatalf("Could not close stream: %v", err)
				}
				return
			}

			// log.Println("Sending to google...")

			if err := stream.Send(&speechpb.StreamingRecognizeRequest{
				StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
					AudioContent: buf,
				},
			}); err != nil {
				log.Printf("Could not send audio: %v", err)
				return
			}
			// log.Println("Sent to google!")
		}
	}()

	// concurrently receive crap
	for {
		log.Println("Received from google!")
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Cannot stream results: %v", err)
		}
		if err := resp.Error; err != nil {
			// Workaround while the API doesn't give a more informative error.
			if err.Code == 3 || err.Code == 11 {
				log.Print("WARNING: Speech recognition request exceeded limit of 60 seconds.")
			}
			log.Println("Could not recognize: %v", err)
			return
		}
		for _, result := range resp.Results {
			fmt.Printf("Result: %+v\n", result)
		}

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


		image := "https://previews.123rf.com/images/punphoto/punphoto1211/punphoto121100083/16291629-colorful-abstract-water-color-art-hand-paint-background.jpg"

		if user == "Rohan" {
			image = "https://media.licdn.com/dms/image/C4E03AQGkhEWibv1y3g/profile-displayphoto-shrink_200_200/0?e=1574294400&v=beta&t=X6UebBepKK8bWMys_1BjFbmsqCdj9CORphR6FPo38Vk"
		} else if user == "Raymond" {
			image = "https://media.licdn.com/dms/image/C4D03AQF0BYrFkX1JIg/profile-displayphoto-shrink_200_200/0?e=1574294400&v=beta&t=izhL8vnWDdLQYeQ6yWkAFu0nqa-0kQyW8CLV3oF_BMk"
		} else if user == "Nick" {
			image = "https://media.licdn.com/dms/image/C5603AQGJpYaNEa0l-A/profile-displayphoto-shrink_200_200/0?e=1574294400&v=beta&t=-pWctaTTVlSjBCYsAPU5_3fS_mpfWN1jjyHFXCyr9PA"
		} else if user == "Devin" {
			image = "https://scontent.fyyz1-2.fna.fbcdn.net/v/t1.0-1/p160x160/65608370_1323260984498761_5942930897262084096_n.jpg?_nc_cat=110&_nc_oc=AQmiZofMeVOOnWkZpZAMWE2jXATJbQ_C5D97dZmnP2wFxsHpBYtyToiKH9k8lCOOKC8&_nc_ht=scontent.fyyz1-2.fna&oh=1853bed21e320be990190a74b53f38bd&oe=5DF91763"
		}

		// append things
		rawText += " " + transcript + "."
		liveChatHtml += `
				<div class="card gradient-shadow gradient-45deg-reverse z-depth-1">
                        <div class="row nunito valign-wrapper">
                            <div class="col s1"></div>
                            <div class="col s1">
                                <img src="` + image + `" class="circle responsive-img">
                            </div>

                            <div class="col s10">
                                <div class="card-content white-text nfont" class="style=">
                                        <b>` + user + `</b>
                                        <br/>
                                        ` + transcript + `
                                </div>
                            </div>
                        </div>
                    </div>
		`
		liveSummaryHtml = getSummary(rawText)
	}
}

func updateVars() {
	for {

	}
}