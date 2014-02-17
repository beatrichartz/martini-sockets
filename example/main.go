package main

import (
   "github.com/codegangsta/martini"
   "github.com/martini-contrib/render"
   "github.com/beatrichartz/sockets"
   "log"
   "sync"
)

// Chat top level
type Chat struct {
	sync.Mutex
	rooms []*Room
}

// Room level
type Room struct {
	sync.Mutex
	name string
	clients []*Client
}

// Client stores all the channels available in the handler in a struct.
type Client struct {
	in <-chan *Message
	out chan<- *Message
	done <-chan bool
	err <-chan error
	disconnect chan<- int
}

// A simple Message struct
type Message struct {
	Name    string    `json:"name"`
	Text    string    `json:"text"`
}

// Get the room for the given name
func (c *Chat) getRoom(name string) *Room {
	c.Lock()
	defer c.Unlock()
	
	for _, room := range c.rooms {
		if room.name == name {
			return room
		}
	}
	
	r := &Room{sync.Mutex{}, name, make([]*Client, 0)}
	c.rooms = append(c.rooms, r)
	
	return r
}

// Add a client to a room
func (r *Room) appendClient(client *Client) {
	r.Lock()
	r.clients = append(r.clients, client)
	r.Unlock()
}

// Remove a client from a room
func (r *Room) removeClient(client *Client) {
	r.Lock()
	defer r.Unlock()
	
	for index, c := range r.clients {
		if c == client {
			r.clients = append(r.clients[:index], r.clients[(index+1):]...)
			break
		}
	}
}

// Message all the other clients in the same room
func (r *Room) messageOtherClients(client *Client, msg *Message) {
	r.Lock()
	for _, c := range r.clients {
		if c != client {
			c.out <- msg
		}
	}
	defer r.Unlock()
}

func newChat() *Chat {
	return &Chat{sync.Mutex{}, make([]*Room, 0)}
}

// the chat
var chat *Chat

func main() {
	m := martini.Classic()
	
	// Use Renderer
	m.Use(render.Renderer(render.Options{
		Layout: "layout",
	}))
	
	// Create the chat
	chat = newChat()
	
	// Index
	m.Get("/", func(r render.Render) {
    r.HTML(200, "index", "")
	})
	
	// render the room
	m.Get("/rooms/:name", func(r render.Render, params martini.Params) {
    r.HTML(200, "room", map[string]map[string]string{"room": map[string]string{"name": params["name"]}})
  })
	
	// This is the sockets connection for the room, it is a json mapping to sockets.
	m.Get("/sockets/rooms/:name", sockets.JSON(Message{}), func(params martini.Params, receiver <-chan *Message, sender chan<- *Message, done <-chan bool, disconnect chan<- int, err <-chan error) {
		client := &Client{receiver, sender, done, err, disconnect}
		r := chat.getRoom(params["name"])
		r.appendClient(client)
		
		// A single select can be used to do all the messaging
		for {
			select {
			case err := <-client.err:
				client.out <- &Message{"", "There has been an error with your connection: " + err.Error()}
			case msg := <-client.in:
				log.Printf("got message %v", msg)
				r.messageOtherClients(client, msg)
			case <-client.done:
				r.removeClient(client)
				log.Printf("client disconnected")
				return
			}
		}
	})

	m.Run()
}