# sockets [![wercker status](https://app.wercker.com/status/4298e26d2bb869fc9b0134ad80ef5eb3/s/ "wercker status")](https://app.wercker.com/project/bykey/4298e26d2bb869fc9b0134ad80ef5eb3)

Sockets to channels binding for martini. This is currently (2/17/2014) still WIP.

[API Reference](http://godoc.org/github.com/beatrichartz/sockets)

## Description

Package `sockets` makes it fun to use websockets with Martini. Its aim is to provide an easy to use interface for socket handling which makes it possible to implement socket messaging with just one `select` statement listening to different channels.

#### JSON

`sockets.JSON` is a simple middleware that organizes websockets messages into any struct type you may wish for.

#### Messages

`sockets.Messages` is a simple middleware that organizes websockets messages into string channels.

## Usage

Have a look into the [example directory](https://github.com/beatrichartz/martini-sockets/tree/master/example) to get a feeling for how to use the sockets package.

This package essentially provides a binding of websockets to channels, which you can use as in the following, contrived example:

```go
m.Get("/", sockets.JSON(Message{}), func(params martini.Params, receiver <-chan *Message, sender chan<- *Message, done <-chan bool, disconnect chan<- int, errorChannel <-chan error) {
	ticker = time.After(30 * time.Minute)
	for {
		select {
		case msg := <-receiver:
			// here we simply echo the received message to the sender for demonstration purposes
			// In your app, collect the senders of different clients and do something useful with them
			sender <- msg
		case <- ticker:
			// This will close the connection after 30 minutes no matter what
			// To demonstrate use of the disconnect channel
			// You can use close codes according to RFC 6455
			disconnect <- websocket.CloseNormalClosure
		case <-client.done:
			// the client disconnected, so you should return / break if the done channel gets sent a message
			return
		case err := <-errorChannel:
			// Uh oh, we received an error. This will happen before a close if the client did not disconnect regularly.
			// Maybe useful if you want to store statistics
			client.out <- &Message{"", "There has been an error with your connection: " + err.Error()}
		}
	}
})
```

For a simple string messaging connection, use ``sockets.Message()``

## Options
You can configure the options for sockets by passing in ``sockets.Options`` as the second argument to ``sockets.JSON`` or as the first argument to ``sockets.Message``. Use it to configure the following options (defaults are used here):

```go
&sockets.Options{
	// The logger to use for socket logging
	Logger: log.New(os.Stdout, "[sockets] ", 0), // *log.Logger

	// The LogLevel for socket logging, possible values:
	// sockets.LogLevelError (0)
	// sockets.LogLevelWarning (1)
	// sockets.LogLevelInfo (2)
	// sockets.LogLevelDebug (3)
	
	LogLevel: sockets.LogLevelInfo, // int

	// The time to wait between writes before timing out the connection
	// When this is a zero value time instance, write will never time out
	WriteWait: 60 * time.Second, // time.Duration

	// The time to wait at maximum between receiving pings from the client.
	PongWait: 60 * time.Second, // time.Duration

	// The time to wait between sending pings to the client
	// Attention, this does have to be shorter than PongWait and WriteWait
	// unless you want connections to constantly time out.
	PingPeriod: (60 * time.Second * 8 / 10), // time.Duration

	// The maximum messages size for receiving and sending in bytes
	// Messages bigger than this will lead to panic and disconnect
	MaxMessageSize 65536 // int64

	// The send channel buffer
	// How many messages can be asynchronously hold before the channel blocks
	SendChannelBuffer int

	// The receiving channel buffer
	// How many messages can be asynchronously hold before the channel blocks
	RecvChannelBuffer int
}
```

## gorilla vs go.net websockets
The gorilla websocket package is a brilliant implementation of websockets which has [the following advantages](https://github.com/gorilla/websocket#protocol-compliance) over the go.net implementation.

## Authors

* [Beat Richartz](https://github.com/beatrichartz)
