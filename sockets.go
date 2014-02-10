//The sockets package implements ... bananas?
package sockets

import (
	"net"
	"os"
	"errors"
	"log"
	"sync"
	"time"
	"reflect"
	"regexp"
	
	"net/http"
	"github.com/codegangsta/martini"
	"github.com/gorilla/websocket"
)

const (
	defaultWriteWait      = 10 * time.Second
	defaultPongWait       = 60 * time.Second
	defaultPingPeriod     = (defaultPongWait * 9) / 10
	defaultMaxMessageSize = 512
)

type Options struct {
	// The logger to use for socket logging
	Logger         *log.Logger

	WriteWait      time.Duration
	PingPeriod     time.Duration
	PongWait       time.Duration
	MaxMessageSize int64
}

type Connection struct {
	*Options
	
	Disconnect chan bool
	
	ws        *websocket.Conn
	wg        sync.WaitGroup
	remoteAddr net.Addr
	done      chan bool
	doneSend  chan bool
	ticker    *time.Ticker
}

type StringConnection struct {
	Connection
	Sender   chan string
	Receiver chan string
}

type BoundConnection struct {
	Connection
	typ      reflect.Type
	Sender   reflect.Value
	Receiver reflect.Value
}

func Messages(options ...*Options) martini.Handler {
	o := newOptions(options)
	
	return func(context martini.Context, writer http.ResponseWriter, req *http.Request) (int, string) {
		status, err := checkRequest(req)
		if err != nil {
			return status, err.Error()
		}
		
		ws, err := doHandshake(writer, req)
		if err != nil {
			return http.StatusBadRequest, err.Error()
		}
		
		c := newStringConnection(ws, o)
		c.setSocketOptions()
		context.Set(reflect.ChanOf(reflect.SendDir, reflect.TypeOf(c.Sender).Elem()), reflect.ValueOf(c.Sender))
		context.Set(reflect.ChanOf(reflect.RecvDir, reflect.TypeOf(c.Receiver).Elem()), reflect.ValueOf(c.Receiver))
		context.Map(c.Disconnect)
				
		go c.send()
		go c.recv()
		
		context.Next()
		
		return http.StatusOK, "OK"
	}
}

func Bind(bindStruct interface{}, options ...*Options) martini.Handler {
	o := newOptions(options)
	
	return func(context martini.Context, writer http.ResponseWriter, req *http.Request) (int, string) {
		status, err := checkRequest(req)
		if err != nil {
			return status, err.Error()
		}
		
		ws, err := doHandshake(writer, req)
		if err != nil {
			return http.StatusBadRequest, err.Error()
		}
		
		c := newBoundConnection(bindStruct, ws, o)
		c.setSocketOptions()
		context.Set(reflect.ChanOf(reflect.SendDir, c.typ), c.Sender)
		context.Set(reflect.ChanOf(reflect.RecvDir, c.typ), c.Receiver)
		context.Map(c.Disconnect)
				
		go c.send()
		go c.recv()
		
		context.Next()
		
		return http.StatusOK, "OK"
	}
}

func (o *Options) log(message string, logVars ...interface{}) {
	o.Logger.Printf("[%s] " + message, logVars...)
}

func (c *Connection) log(message string, logVars ...interface{}) {
	c.Options.log(message, append([]interface{}{c.remoteAddr}, logVars...)...)
}

func (c *Connection) setSocketOptions() {
	c.ws.SetReadLimit(c.MaxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(c.PongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(c.PongWait));
		return nil
	})
}

func (c *Connection) Close() error {
	c.doneSend <- true
	close(c.doneSend)
	
	c.wg.Wait()
		
	if err := c.ws.Close(); err != nil {
		c.log("Connection could not be closed: %s", err.Error())
		return err
	}
	
	c.Disconnect <- true
	close(c.Disconnect)
		
	return nil
}

func (c *StringConnection) Close() error {
	err := c.Connection.Close()
	if err != nil {
		return err
	}
	
	close(c.Sender)
	close(c.Receiver)
	
	c.log("Connection closed")
	return nil
}

func (c *Connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(c.WriteWait))
	return c.ws.WriteMessage(mt, payload)
}

func (c *Connection) ping() error {
	return c.write(websocket.PingMessage, []byte{})
}

func (c *Connection) startTicker() {
	c.ticker = time.NewTicker(c.PingPeriod)
}

func (c *Connection) stopTicker() {
	c.ticker.Stop()
}

func (c *StringConnection) send() {
	c.wg.Add(1)
	
	c.startTicker()
	defer func() {
		c.stopTicker()
		c.wg.Done()
	}()
	
	for {
		select {
		case message, ok := <-c.Sender:
			if !ok {
				c.log("Sender channel has been closed")
				return
			}
			if err := c.write(websocket.TextMessage, []byte(message)); err != nil {
				c.log("Error writing to socket: %s", err)
				return
			}
		case <-c.ticker.C:
			if err := c.ping(); err != nil {
				c.log("Error pinging socket: %s", err)
				return
			}
		case <-c.doneSend:
			return
		}
	}
}

func (c *StringConnection) recv() {
	c.wg.Add(1)
	defer func() {
		c.wg.Done()
		c.Close()
	}()

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			c.log("Error reading from socket: %s", err)
			break
		}
		c.Receiver <-string(message)
	}
}

func (c *BoundConnection) Close() error {
	err := c.Connection.Close()
	if err != nil {
		return err
	}
	
	c.Sender.Close()
	c.Receiver.Close()
	
	return nil
}

func (c *BoundConnection) send() {
	c.wg.Add(1)
	
	c.startTicker()
	
	defer func() {
		c.stopTicker()
		c.wg.Done()
	}()
	
	cases    := make([]reflect.SelectCase, 3)
	cases[0] = reflect.SelectCase{reflect.SelectRecv, c.Sender, reflect.ValueOf(nil),}
	cases[1] = reflect.SelectCase{reflect.SelectRecv, reflect.ValueOf(c.ticker.C), reflect.ValueOf(nil),}
	cases[2] = reflect.SelectCase{reflect.SelectRecv, reflect.ValueOf(c.doneSend), reflect.ValueOf(nil),}
	
	for {
		chosen, message, ok := reflect.Select(cases)
		switch chosen {
		case 0:
			if !ok {
				c.log("Sender channel has been closed")
				return
			}
			if err := c.ws.WriteJSON(message.Interface()); err != nil {
				c.log("Error writing to socket: %s", err)
				return
			}
		case 1:
			if err := c.ping(); err != nil {
				c.log("Error pinging socket: %s", err)
				return
			}
		case 2:
			return
		}
	}
}

func (c *BoundConnection) recv() {
	c.wg.Add(1)
	defer func() {
		c.wg.Done()
		c.Close()
	}()
	
	for {
		message := c.newOfType()
		
		err := c.ws.ReadJSON(message.Interface())
		if err != nil {
			c.log("Error reading from socket: %s", err)
			break
		}
		
		c.Receiver.Send(message)
	}
}

func (c *BoundConnection) newOfType() reflect.Value {
	return reflect.New(c.typ.Elem())
}


func newBoundConnection(bindStruct interface{}, ws *websocket.Conn, o *Options) *BoundConnection {
	typ      := reflect.PtrTo(reflect.TypeOf(bindStruct))
	
	return &BoundConnection{
		newConnection(ws, o),
		typ,
		makeChanOfType(typ),
		makeChanOfType(typ),
	}
}

func newStringConnection(ws *websocket.Conn, o *Options) *StringConnection {
	return &StringConnection{
		newConnection(ws, o),
		make(chan string, 1024),
		make(chan string, 1024),
	}
}

func newConnection(ws *websocket.Conn, o *Options) Connection {
	c := Connection{
		o,
		make(chan bool),
		ws,
		sync.WaitGroup{},
		ws.RemoteAddr(),
		make(chan bool),
		make(chan bool),
		nil,
	}
	
	c.log("Connection established")
	
	return c
}

func newOptions(options []*Options) *Options {
	o := &Options{
		log.New(os.Stdout, "[sockets] ", 0),
		defaultWriteWait,
		defaultPongWait,
		defaultPingPeriod,
		defaultMaxMessageSize,
	}
	
	if len(options) == 0 {
		return o
	}
	
	optionsValue := reflect.ValueOf(*options[0])
	oValue       := reflect.Indirect(reflect.ValueOf(o))
	numFields    := optionsValue.NumField()
	
	for i := 0; i < numFields; i++ {
		if value := optionsValue.Field(i); value.IsValid() {
			oValue.Field(i).Set(value)
		}
	}
	
	return o
}

func makeChanOfType(typ reflect.Type) reflect.Value {
	return reflect.MakeChan(reflect.ChanOf(reflect.BothDir, typ), 1024)
}

func checkRequest(req *http.Request) (int, error) {
	if req.Method != "GET" {
		return http.StatusMethodNotAllowed, errors.New("Method not allowed")
	}
	if r, err := regexp.MatchString("https?://" + req.Host + "$", req.Header.Get("Origin")); !r || err != nil {
		return http.StatusForbidden, errors.New("Origin not allowed")
	}
	
	return http.StatusOK, nil
}

func doHandshake(writer http.ResponseWriter, req *http.Request) (*websocket.Conn, error) {
	ws, err := websocket.Upgrade(writer, req, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		return nil, errors.New("Handshake Failed")
	} else if err != nil {
		return nil, errors.New("Connection Failed")
	}
	
	return ws, nil
}