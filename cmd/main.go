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
	Username string `json:"user"`
	Password string `json:"pass"`
	Broker   string `json:"broker"`
	Port     int    `json:"port"`
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
		log.Fatal(err)
	}
	var creds Credentials
	err = json.Unmarshal(creds_data, &creds)
	if err != nil {
		log.Fatal(err)
	}

	shouldDraw := true

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
		if msg.Topic() == "home-assistant/signage/display/control" {
			shouldDraw = string(msg.Payload()) == "payload_on"
			publishDisplayStatus(client, shouldDraw)
		}
	})

	// Create the MQTT client.
	client := mqtt.NewClient(options)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	// Subscribe to the topic.
	subscribe(client, "home-assistant/signage/control", 0)
	subscribe(client, "home-assistant/signage/display/control", 0)

	// Publish the availability.
	publishAvailable(client)

	// Create a new SPI port.
	host.Init()

	p, err := spireg.Open("")
	if err != nil {
		log.Fatal(err)
	}
	defer p.Close()

	// Create a new Unicorn HAT HD.
	display, err := unicornhd.New(p)
	if err != nil {
		log.Fatal(err)
	}

	// Load the font
	fontBytes, err := fonts.ReadFile("fonts/UbuntuMono-Regular.ttf")
	if err != nil {
		log.Fatal(err)
	}

	// Load the images
	imageBytes, err := images.ReadFile("images/redScreen.png")
	if err != nil {
		log.Fatal(err)
	}
	redImg, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		log.Fatal(err)
	}
	// Load the images
	imageBytes, err = images.ReadFile("images/orangeScreen.png")
	if err != nil {
		log.Fatal(err)
	}
	orgImg, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		log.Fatal(err)
	}
	imageBytes, err = images.ReadFile("images/blueScreen.png")
	if err != nil {
		log.Fatal(err)
	}
	bluImg, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		log.Fatal(err)
	}

	// Clear the display.
	display.Halt()

	// Every 10 minutes, publish the current state.
	ticker := time.NewTicker(10 * time.Minute)
	quit := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Printf("Ticker: Publishing current state")
				publishAvailable(client)
				publishDisplayStatus(client, shouldDraw)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
	// Wait for messages.
	for {
		select {
		case command := <-textToDraw:
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
					if shouldDraw {
						display.Draw(image.Rect(0, 0, 16, 16), imgToDraw, image.Point{0, 0})
						time.Sleep(150 * time.Millisecond)
						display.Halt()
						time.Sleep(150 * time.Millisecond)
					} else {
						display.Halt()
					}
				}
			}
			for x := -16; true; x++ {
				if shouldDraw {
					textimage, err := unicornsignage.ImageFromText(command.Messsage, fontBytes, x)
					if err != nil {
						log.Fatal(err)
					}
					display.Draw(image.Rect(0, 0, 16, 16), textimage, image.Point{0, 0})
					time.Sleep(2 * time.Millisecond)
					if (x > 16) && isFullyBlack(textimage) {
						break
					}
				} else {
					display.Halt()
				}
			}
		}
	}
}

func isFullyBlack(img image.Image) bool {
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
	publish(client, "home-assistant/signage/display/availability", 1, "online", false)
}

func publishDisplayStatus(client mqtt.Client, status bool) {
	if status {
		publish(client, "home-assistant/signage/display/contact", 1, "payload_on", false)
	} else {
		publish(client, "home-assistant/signage/display/contact", 1, "payload_off", false)
	}
}
