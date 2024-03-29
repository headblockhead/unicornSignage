package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/headblockhead/unicornsignage"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/unicornhd"
	"periph.io/x/host/v3"
)

type Credentials struct {
	Username               string `json:"user"`
	Password               string `json:"pass"`
	Broker                 string `json:"broker"`
	Port                   int    `json:"port"`
	OpenWeatherApiKey      string `json:"openweatherapikey"`
	OpenWeatherApiLocation string `json:"openweatherlocation"`
}

type Command struct {
	Messsage string   `json:"msg"`
	Priority Priority `json:"priority"`
}

type Priority string

const (
	PriorityNone        Priority = "0.0"
	PriorityInfo        Priority = "1.0"
	PriorityWarning     Priority = "2.0"
	PriorityExclamation Priority = "3.0"
)

//go:embed fonts
var fonts embed.FS

//go:embed images
var images embed.FS

func main() {
	// Create a channel to receive messages
	textToDraw := make(chan Command, 10)

	// Load credentials
	creds_data, err := ioutil.ReadFile("./creds.json")
	if err != nil {
		log.Println(err)
	}
	var creds Credentials
	err = json.Unmarshal(creds_data, &creds)
	if err != nil {
		log.Println(err)
	}

	// Create the MQTT options.
	options := mqtt.NewClientOptions()
	options.AddBroker(fmt.Sprintf("tcp://%s:%d", creds.Broker, creds.Port))
	options.SetClientID("go_mqtt_client_weather")
	options.SetUsername(creds.Username)
	options.SetPassword(creds.Password)
	options.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		// Runs when a message that is subscribed to is received.
		log.Printf("Received message: %s on topic: %s", msg.Payload(), msg.Topic())
		if msg.Topic() == "home-assistant/signage/control" {
			var cmd Command
			err = json.Unmarshal(msg.Payload(), &cmd)
			if err != nil {
				log.Printf("failed to unmarshal payload: %q", string(msg.Payload()))
				return
			}
			textToDraw <- cmd
		}
	})

	// Create the MQTT client.
	client := mqtt.NewClient(options)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	// Subscribe to the topic.
	subscribe(client, "home-assistant/signage/control", 0)

	// Publish the availability.
	publishAvailable(client)

	// Create a new SPI port.
	host.Init()

	p, err := spireg.Open("")
	if err != nil {
		log.Println(err)
	}
	defer p.Close()

	// Create a new Unicorn HAT HD.
	display, err := unicornhd.New(p)
	if err != nil {
		log.Println(err)
	}

	// Load the font
	fontBytes, err := fonts.ReadFile("fonts/UbuntuMono-Regular.ttf")
	if err != nil {
		log.Println(err)
	}

	// Load the images
	imageBytes, err := images.ReadFile("images/redScreen.png")
	if err != nil {
		log.Println(err)
	}
	redImg, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		log.Println(err)
	}
	// Load the images
	imageBytes, err = images.ReadFile("images/orangeScreen.png")
	if err != nil {
		log.Println(err)
	}
	orgImg, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		log.Println(err)
	}
	imageBytes, err = images.ReadFile("images/blueScreen.png")
	if err != nil {
		log.Println(err)
	}
	bluImg, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		log.Println(err)
	}

	// Clear the display.
	display.Halt()

	// Every 10 minutes, publish the current state and update the weather info.
	ticker := time.NewTicker(10 * time.Minute)

	// Every hour, if the API used to be down, check if the API is still down.
	apiErrorChecker := time.NewTicker(30 * time.Minute)
	hasHadError := false

	oldWeatherImage, err := unicornsignage.GetWeatherImageFromAPI(creds.OpenWeatherApiKey, creds.OpenWeatherApiLocation, images)
	if err != nil {
		log.Println(err)
		log.Println("Error getting image from API. Trying again in 30 minutes.")
		hasHadError = true
	}

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Printf("Ticker: Restarting the MQTT connection.")
				client.Disconnect(250) // allow 250ms for the disconnect to complete
				if token := client.Connect(); token.Wait() && token.Error() != nil {
					log.Printf("Ticker: Error restarting MQTT: %s", token.Error())
				}
				log.Println("Ticker: Publishing current state")
				publishAvailable(client)

				log.Printf("Ticker: Re-subscribing to topics")
				subscribe(client, "home-assistant/signage/control", 0)

				if !hasHadError {
					log.Println("Ticker: Updating the display image")
					oldWeatherImage, err = unicornsignage.GetWeatherImageFromAPI(creds.OpenWeatherApiKey, creds.OpenWeatherApiLocation, images)
					if err != nil {
						log.Println(err)
						log.Println("Error getting image from API. Trying again in 30 minutes.")
						hasHadError = true
					}
				} else {
					log.Println("Ticker: API is not up, not attempting to update the display image")
				}
				log.Printf("Ticker: Done")
			case <-apiErrorChecker.C:
				if hasHadError {
					log.Println("API Error Ticker: Testing API status")
					oldWeatherImage, err = unicornsignage.GetWeatherImageFromAPI(creds.OpenWeatherApiKey, creds.OpenWeatherApiLocation, images)
					if err != nil {
						log.Println(err)
						log.Println("API Error Ticker: API is still returning error")
						hasHadError = true
					} else {
						log.Println("API Error Ticker: API is not returning error anymore!")
						hasHadError = false
					}
				}
			}
		}
	}()

	isShowingText := false

	if !hasHadError {
		go displayCurrentWeather(display, fontBytes, &creds, &textToDraw, &oldWeatherImage, &isShowingText)
	}

	// Wait for messages.
	for {
		select {
		case command := <-textToDraw:
			isShowingText = true
			var imgToDraw image.Image
			switch command.Priority {
			case PriorityExclamation:
				imgToDraw = redImg
			case PriorityInfo:
				imgToDraw = bluImg
			case PriorityWarning:
				imgToDraw = orgImg
			default:
				imgToDraw = nil
			}
			if imgToDraw != nil {
				for i := 0; i < 15; i++ {
					rotatedImage, err := unicornsignage.RotateImage90(imgToDraw)
					if err != nil {
						log.Println(err)
					}
					display.Draw(image.Rect(0, 0, 16, 16), rotatedImage, image.Point{0, 0})
					time.Sleep(150 * time.Millisecond)
					display.Halt()
					time.Sleep(150 * time.Millisecond)

				}
			}
			for x := -16; true; x++ {

				textimage, err := unicornsignage.ImageFromText(command.Messsage, fontBytes, x, 15)
				if err != nil {
					log.Println(err)
				}
				display.Draw(image.Rect(0, 0, 16, 16), textimage, image.Point{0, 0})
				time.Sleep(2 * time.Millisecond)
				if (x > 16) && imageIsFullyBlack(textimage) {
					break
				}

			}
			isShowingText = false
			if !hasHadError {
				go displayCurrentWeather(display, fontBytes, &creds, &textToDraw, &oldWeatherImage, &isShowingText)
			}
		}
	}
}

func displayCurrentWeather(display *unicornhd.Dev, fontBytes []byte, creds *Credentials, textToDraw *chan Command, oldWeatherImage *image.Image, isShowingText *bool) {
	time.Sleep(100 * time.Millisecond)
	for {
		// If we are currently showing text on the screen, stop trying to show the weather.
		if *isShowingText {
			return
		}
		time.Sleep(30 * time.Second)
		// If it is after 9PM, or before 7AM, don't show the weather either.
		if time.Now().Local().Hour() >= 21 || time.Now().Local().Hour() < 7 {
			newImage := image.NewNRGBA(image.Rect(0, 0, 16, 16))
			display.Draw(image.Rect(0, 0, 16, 16), newImage, image.Point{0, 0})
			continue
		}
		rotatedImage, err := unicornsignage.RotateImage90(*oldWeatherImage)
		if err != nil {
			log.Println(err)
		}
		display.Draw(image.Rect(0, 0, 16, 16), rotatedImage, image.Point{0, 0})
	}
}

func imageIsFullyBlack(img image.Image) bool {
	for x := img.Bounds().Min.X; x < img.Bounds().Dx(); x++ {
		for y := img.Bounds().Min.Y; y < img.Bounds().Dy(); y++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if (r != 0) || (g != 0) || (b != 0) {
				return false
			}
		}
	}
	return true
}

func publish(client mqtt.Client, topic string, qos byte, payload string, retain bool) {
	token := client.Publish(topic, qos, retain, payload)
	token.Wait()
}

func subscribe(client mqtt.Client, topic string, qos byte) {
	token := client.Subscribe(topic, qos, nil)
	token.Wait()
	log.Printf("Subscribed to topic %s", topic)
}

func publishAvailable(client mqtt.Client) {
	publish(client, "home-assistant/signage/availability", 1, "online", false)
}
