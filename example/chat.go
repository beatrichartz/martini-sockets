package main

import (
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"github.com/beatrichartz/martini-sockets"
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
	name    string
	clients []*Client
}

// Client stores all the channels available in the handler in a struct.
type Client struct {
	Name       string
	in         <-chan *Message
	out        chan<- *Message
	done       <-chan bool
	err        <-chan error
	disconnect chan<- int
}

// A simple Message struct
type Message struct {
	Typ  string `json:"typ"`
	From string `json:"from"`
	Text string `json:"text"`
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
	for _, c := range r.clients {
		if c != client {
			c.out <- &Message{"status", client.Name, "Joined this chat"}
		}
	}
	r.Unlock()
}

// Remove a client from a room
func (r *Room) removeClient(client *Client) {
	r.Lock()
	defer r.Unlock()

	for index, c := range r.clients {
		if c == client {
			r.clients = append(r.clients[:index], r.clients[(index+1):]...)
		} else {
			c.out <- &Message{"status", client.Name, "Left this chat"}
		}
	}
}

// Message all the other clients in the same room
func (r *Room) messageOtherClients(client *Client, msg *Message) {
	r.Lock()
	msg.From = client.Name
		
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
	m.Get("/sockets/rooms/:name/:clientname", sockets.JSON(Message{}), func(params martini.Params, receiver <-chan *Message, sender chan<- *Message, done <-chan bool, disconnect chan<- int, err <-chan error) (int,string) {
		client := &Client{params["clientname"], receiver, sender, done, err, disconnect}
		r := chat.getRoom(params["name"])
		r.appendClient(client)

		// A single select can be used to do all the messaging
		for {
			select {
			case <-client.err:
				// Don't try to do this:
				// client.out <- &Message{"system", "system", "There has been an error with your connection"}
				// The socket connection is already long gone.
				// Use the error for statistics etc
			case msg := <-client.in:
				r.messageOtherClients(client, msg)
			case <-client.done:
				r.removeClient(client)
				return 200, "OK"
			}
		}
	})

	m.Run()
}
