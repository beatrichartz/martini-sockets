package sockets

import (
	"github.com/gorilla/websocket"
	"github.com/codegangsta/martini"
	"net/http"
	"testing"
	"time"
	"sync"
	"log"
	"github.com/rakyll/coop"
)

const (
	host string = "http://localhost:3000"
	endpoint string = "ws://localhost:3000"
	recvPath string = "/receiver"
	sendPath string = "/sender"
	pingPath string = "/ping"
	recvStringsPath string = "/strings/receiver"
	sendStringsPath string = "/strings/sender"
	pingStringsPath string = "/strings/ping"
)

type Message struct {
	Text string `json:"text"`
}

var once sync.Once
var recvMessages []*Message
var recvCount int
var recvDone bool
var sendMessages []*Message
var sendCount int
var pingCount int
var pingMessages []*Message

var recvStrings []string
var recvStringsCount int
var recvStringsDone bool
var sendStrings []string
var sendStringsCount int

// Test Helpers
func expectStringsToBeEmpty(t *testing.T, strings []string) {
	if len(strings) > 0 {
		t.Errorf("Expected strings array to be empty, but they contained %d values", len(strings))
	}
}

func expectMessagesToBeEmpty(t *testing.T, messages []*Message) {
	if len(messages) > 0 {
		t.Errorf("Expected messages array to be empty, but they contained %d values", len(messages))
	}
}

func expectStringsToHaveArrived(t *testing.T, count int, strings []string) {
	if len(strings) < count {
		t.Errorf("Expected strings array to contain 3 values, but contained %d", len(strings))
	} else {
		for i, s := range strings {
			if s != "Hello World" {
				t.Errorf("Expected string %d to be \"Hello World\", but was \"%v\"", i+1, s)
			}
		}
	}

}

func expectMessagesToHaveArrived(t *testing.T, count int, messages []*Message) {
	if len(messages) < count {
		t.Errorf("Expected messages array to contain 3 values, but contained %d", len(messages))
	} else {
		for i, m := range messages {
			if m.Text != "Hello World" {
				t.Errorf("Expected message %d to contain \"Hello World\", but contained %v", i+1, m)
			}
		}
	}
}

func expectPingsToHaveBeenExecuted(t *testing.T, count int, messages []*Message) {
	if len(messages) < count {
		t.Errorf("Expected messages array to contain 3 ping values, but contained %d", len(messages))
	} else {
		for i, m := range messages {
			if m.Text != "" {
				t.Errorf("Expected message %d to contain \"\", but contained %v", i+1, m)
			}
		}
	}
}

func startServer() {
	log.Println("starting Server for tests")
	
	m := martini.Classic()
	
	m.Get(recvPath, Bind(Message{}), func(context martini.Context, receiver <-chan *Message, done chan bool) int {
		for {
			select {
			case msg := <- receiver:
				recvMessages = append(recvMessages, msg)
			case <-done:
				log.Println("done sig received on " + recvPath)
				return http.StatusOK
			}
		}
		
		return http.StatusOK
	})
	
	m.Get(sendPath, Bind(Message{}), func(context martini.Context, sender chan<- *Message, done chan bool) int {
		ticker := time.NewTicker(3*time.Millisecond)
		
		for {
			select {
			case <-ticker.C:
				sender<- &Message{"Hello World"}
			case <-done:
				return http.StatusOK
			}
		}
		
		return http.StatusOK
	})
	
	m.Get(recvStringsPath, Messages(), func(context martini.Context, receiver <-chan string, done chan bool) int {
		for {
			select {
			case msg := <- receiver:
				recvStrings = append(recvStrings, msg)
			case <-done:
				return http.StatusOK
			}
		}
		
		return http.StatusOK
		
	})
	
	m.Get(sendStringsPath, Messages(), func(context martini.Context, sender chan<- string, done chan bool) int {
		for {
			select {
			case <-time.After(3*time.Millisecond):
				sender<- "Hello World"
			case <-done:
				return http.StatusOK
			}
		}
		
		return http.StatusOK
	})
	
	go m.Run()
}

func connectSocket(t *testing.T, path string) *websocket.Conn {
	header := make(http.Header)
	header.Add("Origin", host)
	ws, _, err := websocket.DefaultDialer.Dial(endpoint + path, header)
	if err != nil {
		t.Fatalf("Connecting the socket failed: %s", err.Error())
	}
	return ws
}

func TestStringReceive(t *testing.T) {
	once.Do(startServer)
	expectStringsToBeEmpty(t, recvStrings)
	
	ws := connectSocket(t, recvStringsPath)
	defer ws.Close()

	ch := coop.Until(time.Now().Add(4*time.Millisecond), time.Millisecond, func() {
		s := "Hello World"
		err := ws.WriteMessage(websocket.TextMessage, []byte(s))
		if err != nil {
			t.Errorf("Writing to the socket failed with %s", err.Error())
		}
		recvStringsCount++
		if recvStringsCount == 4 {
			return
		}
	})
	<-ch
	
	expectStringsToHaveArrived(t, 3, recvStrings)
}

func TestStringSend(t *testing.T) {
	once.Do(startServer)
	expectStringsToBeEmpty(t, sendStrings)
	
	ws := connectSocket(t, sendStringsPath)
	defer ws.Close()

	for {
		_, msgArray, err := ws.ReadMessage()
		msg := string(msgArray)
		sendStrings = append(sendStrings, msg)
		if err != nil {
			t.Errorf("Receiving from the socket failed with %v", err)
		}
		if sendStringsCount == 3 {
			expectStringsToHaveArrived(t, 3, sendStrings)
			return
		}
		sendStringsCount++
	}
	
}

func TestBindReceive(t *testing.T) {
	once.Do(startServer)
	expectMessagesToBeEmpty(t, recvMessages)
	
	ws := connectSocket(t, recvPath)
	
	message := &Message{"Hello World"}
		
	ch := coop.Until(time.Now().Add(4*time.Millisecond), time.Millisecond, func() {
		err := ws.WriteJSON(message)
		if err != nil {
			t.Errorf("Writing to the socket failed with %v", err)
		}
		recvCount++
		if recvCount == 4 {
			return
		}
	})
	<-ch
	
	ws.Close()
	
	expectMessagesToHaveArrived(t, 3, recvMessages)
}

func TestBindSend(t *testing.T) {
	once.Do(startServer)
	expectMessagesToBeEmpty(t, sendMessages)
	
	ws := connectSocket(t, sendPath)
	defer ws.Close()
	
	for {
		msg := &Message{}
		err := ws.ReadJSON(msg)
		sendMessages = append(sendMessages, msg)
		if err != nil {
			t.Errorf("Receiving from the socket failed with %v", err)
		}
		if sendCount == 3 {
			expectMessagesToHaveArrived(t, 3, sendMessages)
			return
		}
		sendCount++
	}
	
	
}