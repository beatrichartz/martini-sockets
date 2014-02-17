package sockets

import (
	"github.com/gorilla/websocket"
	"github.com/codegangsta/martini"
	"net/http/httptest"
	"net/http"
	"testing"
	"time"
	"sync"
	"strings"
	"io"
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

var (
	once sync.Once
	recvMessages []*Message
	recvCount int
	recvDone bool
	sendMessages []*Message
	sendCount int
	sendDone bool
	recvStrings []string
	recvStringsCount int
	recvStringsDone bool
	sendStrings []string
	sendStringsCount int
	sendStringsDone bool
)

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

func expectStatusCode(t *testing.T, expectedStatusCode int, actualStatusCode int) {
	if actualStatusCode != expectedStatusCode {
		t.Errorf("Expected StatusCode %d, but received %d", expectedStatusCode, actualStatusCode)
	}
}

func expectIsDone(t *testing.T, done bool) {
	if !done {
		t.Errorf("Expected to be done, but was not")
	}
}

func startServer() {
	m := martini.Classic()
	
	m.Get(recvPath, JSON(Message{}), func(context martini.Context, receiver <-chan *Message, done <-chan bool) int {
		for {
			select {
			case msg := <- receiver:
				recvMessages = append(recvMessages, msg)
			case <-done:
				recvDone = true
				return http.StatusOK
			}
		}
		
		return http.StatusOK
	})
	
	m.Get(sendPath, JSON(Message{}), func(context martini.Context, sender chan<- *Message, done <-chan bool, disconnect chan<- int) int {
		ticker := time.NewTicker(1*time.Millisecond)
		bomb   := time.After(4*time.Millisecond)
		
		for {
			select {
			case <-ticker.C:
				sender<- &Message{"Hello World"}
			case <-done:
				ticker.Stop()
				sendDone = true
				return http.StatusOK
			case <-bomb:
				disconnect<-websocket.CloseGoingAway
				return http.StatusOK
			}
		}
		
		return http.StatusOK
	})
	
	m.Get(recvStringsPath, Messages(), func(context martini.Context, receiver <-chan string, done <-chan bool) int {
		for {
			select {
			case msg := <- receiver:
				recvStrings = append(recvStrings, msg)
			case <-done:
				recvStringsDone = true
				return http.StatusOK
			}
		}
		
		return http.StatusOK
	})
	
	m.Get(sendStringsPath, Messages(), func(context martini.Context, sender chan<- string, done <-chan bool, disconnect chan<- int) int {
		ticker := time.NewTicker(1*time.Millisecond)
		bomb   := time.After(4*time.Millisecond)
		
		for {
			select {
			case <-ticker.C:
				sender<- "Hello World"
			case <-done:
				ticker.Stop()
				sendStringsDone = true
				return http.StatusOK
			case <-bomb:
				disconnect<-websocket.CloseGoingAway
				
				return http.StatusOK
			}
		}
		
		return http.StatusOK
	})

	go m.Run()
	time.Sleep(5*time.Millisecond)
}

func connectSocket(t *testing.T, path string) (*websocket.Conn, *http.Response) {
	header := make(http.Header)
	header.Add("Origin", host)
	ws, resp, err := websocket.DefaultDialer.Dial(endpoint + path, header)
	if err != nil {
		t.Fatalf("Connecting the socket failed: %s", err.Error())
	}
	
	return ws, resp
}

func TestStringReceive(t *testing.T) {
	once.Do(startServer)
	expectStringsToBeEmpty(t, recvStrings)
	
	ws, resp := connectSocket(t, recvStringsPath)


	ticker := time.NewTicker(time.Millisecond)
	
	for {
		<- ticker.C
		s := "Hello World"
		err := ws.WriteMessage(websocket.TextMessage, []byte(s))
		if err != nil {
			t.Errorf("Writing to the socket failed with %s", err.Error())
		}
		recvStringsCount++
		if recvStringsCount == 4 {
			ws.Close()
			return
		}
	}
		
	expectStringsToHaveArrived(t, 3, recvStrings)
	expectStatusCode(t, http.StatusSwitchingProtocols, resp.StatusCode)
	expectIsDone(t, recvStringsDone)
}

func TestStringSend(t *testing.T) {
	once.Do(startServer)
	expectStringsToBeEmpty(t, sendStrings)
	
	ws, resp := connectSocket(t, sendStringsPath)
	defer ws.Close()

	for {
		_, msgArray, err := ws.ReadMessage()
		msg := string(msgArray)
		sendStrings = append(sendStrings, msg)
		if err != nil && err != io.EOF {
			t.Errorf("Receiving from the socket failed with %v", err)
		}
		if sendStringsCount == 3 {
			return
		}
		sendStringsCount++
	}
	expectStringsToHaveArrived(t, 3, sendStrings)
	expectStatusCode(t, http.StatusSwitchingProtocols, resp.StatusCode)
	expectIsDone(t, sendStringsDone)
}

func TestJSONReceive(t *testing.T) {
	once.Do(startServer)
	expectMessagesToBeEmpty(t, recvMessages)
	
	ws, resp := connectSocket(t, recvPath)
	
	message := &Message{"Hello World"}
	
	ticker := time.NewTicker(time.Millisecond)
	
	for {
		<- ticker.C
		err := ws.WriteJSON(message)
		if err != nil {
			t.Errorf("Writing to the socket failed with %v", err)
		}
		recvCount++
		if recvCount == 4 {
			ws.Close()
			return
		}
	}
	
	expectMessagesToHaveArrived(t, 3, recvMessages)
	expectStatusCode(t, http.StatusSwitchingProtocols, resp.StatusCode)
	expectIsDone(t, recvDone)
}

func TestJSONSend(t *testing.T) {
	once.Do(startServer)
	expectMessagesToBeEmpty(t, sendMessages)
	
	ws, resp := connectSocket(t, sendPath)
	defer ws.Close()
	
	for {
		msg := &Message{}
		err := ws.ReadJSON(msg)
		sendMessages = append(sendMessages, msg)
		if err != nil && err != io.EOF {
			t.Errorf("Receiving from the socket failed with %v", err)
		}
		if sendCount == 3 {
			return
		}
		sendCount++
	}
	expectMessagesToHaveArrived(t, 3, sendMessages)
	expectStatusCode(t, http.StatusSwitchingProtocols, resp.StatusCode)
	expectIsDone(t, sendDone)
}

func TestCrossOrigin(t *testing.T) {
	header := make(http.Header)
	header.Add("Origin", "http://somewhere.com")
	_, resp, err := websocket.DefaultDialer.Dial(endpoint + sendPath, header)
	if err == nil {
		t.Fatalf("Connecting to the socket succeeded with a cross origin request")
	}
	expectStatusCode(t, http.StatusForbidden, resp.StatusCode)
}

func TestUnallowedMethods(t *testing.T) {
	m := martini.Classic()
	
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/test", strings.NewReader(""))

	if err != nil {
		t.Error(err)
	}
	
	m.Any("/test", Messages(), func() int {
		return http.StatusOK
	})
	
	m.ServeHTTP(recorder, req)
	expectStatusCode(t, http.StatusMethodNotAllowed, recorder.Code)
}