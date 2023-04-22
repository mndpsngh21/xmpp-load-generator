package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	"./xmpp"
	"github.com/brianvoe/gofakeit"
)

const tagSent string = "sent"
const tagReceived string = "received"

// parameters
// parameters
var msPerMsgPerUser = 10000
var totalMsgPerUser = flag.Int("t", 2000, "total number of messages per user")
var sampleRate = flag.Int("r", 2000, "sample rate in milliseconds")
var imgFile = flag.String("o", "", "chart output of the result")
var numberOfUsers = 20000
var totalMsg = 0
var channelClosed bool
var loginChan = make(chan string)
var xmppClients = []*xmpp.Client{}
var resultChan = make(chan string, 100000000)
var latencyChan = make(chan int64, 100000000)

func main() {
	port := flag.Int("port", 5222, "port for connection")
	startIdx := flag.Int("startIdx", 0, "port for connect")
	batchSize := flag.Int("batchSize", 500, "port for connect")
	connections := flag.Int("connections", 28000, "port for connect")

	flag.Parse()
	totalMsg = *totalMsgPerUser * numberOfUsers
	fmt.Println("Please provide start index and connection per system:")
	fmt.Println("  Example: for run 1000 connection start will be 0 and connection will be 1000")
	fmt.Println("  			This will create 1000 connection on single system, please start from from total count running in environment")
	fmt.Println("Please provide startIndex")
	var startIdxVal int = *startIdx
	var connectionCountVal int = *connections
	var batchSizeVal int = *batchSize

	if connectionCountVal < batchSizeVal {
		batchSizeVal = connectionCountVal
	}
	var portVal int = *port

	numberOfUsers = startIdxVal + connectionCountVal
	fmt.Printf("Starting connections :: %v", numberOfUsers)
	// login all users sequencially
	host := "51.222.105.20:" + strconv.Itoa(portVal)
	startConnections(batchSizeVal, host)
	// start chat bot thread
	for _, xmppClient := range xmppClients {
		go startChatBot(xmppClient, resultChan, latencyChan)
	}

	// new thread to print out result per second
	sent := 0
	received := 0
	xValues := []float64{}
	ySentValues := []float64{}
	yReceivedValues := []float64{}

	exitChan := make(chan string)
	go func() {
		counter := 0

		for {
			select {
			case <-exitChan:
				return
			default:
				rate := 0.0
				if sent != 0 {
					rate = float64(received) / float64(sent)
				}
				xValues = append(xValues, float64(counter))
				ySentValues = append(ySentValues, float64(sent))
				yReceivedValues = append(yReceivedValues, float64(received))
				fmt.Printf("Time: %dms, Sent: %d, Received: %d, Rate: %f\n", counter, sent, received, rate)
				time.Sleep(time.Duration(*sampleRate) * time.Millisecond)
				counter += *sampleRate
			}
		}
	}()

	// read result from result channel
	for r := range resultChan {
		if r == tagSent {
			sent += 1
		} else if r == tagReceived {
			received += 1
			if received == totalMsg {
				time.Sleep(5 * time.Second) // sleep a while to let the output continue
				exitChan <- "exit"
				channelClosed = true
				close(exitChan)
				close(resultChan)
				close(latencyChan)
			}
		}
	}
	fmt.Println("Completed !")
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func startConnections(batchSize int, host string) {
	go func() {

		for i := 1; i <= numberOfUsers; i++ {

			for j := 1; j <= batchSize; j++ {
				go func(id int) {
					var err error
					user := fmt.Sprintf("u_%d@shikshapns", id)
					options := xmpp.Options{Host: host,
						User:          user,
						Password:      fmt.Sprintf("u_%d", id),
						NoTLS:         true,
						Debug:         false,
						Session:       false,
						Status:        "xa",
						StatusMessage: user + " is testing",
					}

					randomSleep(2) // login all users within 10 seconds
					xmppClient, err := options.NewClient()

					if err != nil {
						fmt.Print(err)
					} else {
						xmppClients = append(xmppClients, xmppClient)
						loginChan <- user
					}
				}(i + j)

			}
			i = i + batchSize
			time.Sleep(2 * time.Second)
		}
	}()

	// wait for all users to login
	totalLogin := 0
	for u := range loginChan {
		totalLogin += 1
		fmt.Printf("%s logs in (total: %d)\n", u, totalLogin)
		if totalLogin == numberOfUsers {
			break
		}
	}
}

func randomSleep(maxSecond float64) {
	x := time.Duration(maxSecond*float64(rand.Intn(1000))) * time.Millisecond
	time.Sleep(x)
}
func nowInUnixMilli() int64 {
	return time.Now().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}
func startChatBot(talk *xmpp.Client, resultChan chan<- string, latencyChan chan<- int64) {
	// receive message
	go func() {
		for {
			chat, err := talk.Recv()
			if err != nil {
				log.Fatal(err)
			}
			switch v := chat.(type) {
			case xmpp.Chat:
				if channelClosed {
					return
				}
				resultChan <- tagReceived
				// calculate the latency
				sentTime, _ := strconv.ParseInt(v.Text, 10, 64)
				latencyMs := nowInUnixMilli() - sentTime
				latencyChan <- latencyMs
			}
		}
	}()
	// random delay, upto 2 seconds
	randomSleep(5)
	// send message
	maxInterval := float64(msPerMsgPerUser) * 2.0 / 1000.0
	for i := 0; i < *totalMsgPerUser; i++ {
		randomUser := fmt.Sprintf("u_%d@shikshapns", rand.Intn(numberOfUsers)+1)
		sentence := gofakeit.SentenceSimple()
		msg := xmpp.Chat{Remote: randomUser, Type: "chat", Text: sentence}
		fmt.Println("sent ::", sentence, "to::", randomUser)
		_, err := talk.Send(msg)
		if err == nil {
			if !channelClosed {
				resultChan <- tagSent
				randomSleep(maxInterval)
			} else {
				break
			}
		}
	}

}
