# sockets

Sockets channel binding for martini.

[API Reference](http://godoc.org/github.com/codegangsta/martini-contrib/sockets)



## Description

Package `sockets` makes it fun to use websockets with Martini.

#### Bind

`sockets.Bind` is a simple middleware that organizes websockets messages into any struct type you may wish for.

#### Messages

`sockets.Messages` is a simple middleware that organizes websockets messages into string channels.

## Usage

Let's build a simple chat for martini using the `sockets` package:

```go
package main

import (
   "net/http"
   
   "github.com/codegangsta/martini"
   "github.com/codegangsta/martini-contrib/sockets"
)

type Chat struct {
	rooms []*Room
}

type Room struct {
	name string
	clients []*Client
}

type Client struct {
	in <-chan *Message
	out chan<- *Message
}

type Message struct {
	Title   string    `json:"title"`
	Content string    `json:"content"`
}

func main() {
	m := martini.Classic()
	
	rooms := make([]*Room, 0)
	chat := Chat{rooms}
	
	m.Get("/room/:name", sockets.Bind(Message{}), func(params martini.Params, receiver <-chan *Message, sender chan<- *Message) {
		var r Room
		for _, room := range rooms {
			if room.name == params["name"] {
				r = room
				break
			}
		}
		if r == nil {
			r = Room{params["name"], make([]*Client)}
			rooms = append(rooms, &r)
		}
		client = Client{receiver, sender}
		for {
			msg := <-client.in
			for _, c := range r.clients {
				if c != client {
					c.out <- msg
				}
			}
		}
	})

	m.Run()
}
```

## Authors

* [Beat Richartz](https://github.com/beatrichartz)
