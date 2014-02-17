# sockets

Sockets to channels binding for martini. This is currently (2/17/2014) still WIP.

[API Reference](http://godoc.org/github.com/beatrichartz/sockets)

## Description

Package `sockets` makes it fun to use websockets with Martini. Its aim is to provide an easy to use interface for socket handling which makes it possible to implement socket messaging with just one `select` statement listening to different channels.

#### JSON

`sockets.JSON` is a simple middleware that organizes websockets messages into any struct type you may wish for.

#### Messages

`sockets.Messages` is a simple middleware that organizes websockets messages into string channels.

## Usage

Have a look into the example directory to get a feeling for how to use the sockets package.

## Authors

* [Beat Richartz](https://github.com/beatrichartz)
