// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocketslive

import (
	"ttnmapper-services/pkg/types"
)

// Hub owns client registration and delivers packets matching all supplied filters.
// Its state is accessed only by the run loop.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{clients: make(map[*Client]bool), broadcast: make(chan []byte), register: make(chan *Client), unregister: make(chan *Client)}
}
func (h *Hub) run() { h.runMessages(messageChannel, nil) }

// A nil stop channel keeps the production loop running. Tests can stop it cleanly.
func (h *Hub) runMessages(messages <-chan types.TtnMapperUplinkMessage, stop <-chan struct{}) {
	defer func() {
		for client := range h.clients {
			h.removeClient(client)
		}
	}()
	for {
		select {
		case <-stop:
			return
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			h.removeClient(client)
		case message, ok := <-messages:
			if !ok {
				return
			}
			h.deliver(message)
		}
	}
}
func (h *Hub) removeClient(client *Client) {
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
}
func (c *Client) matches(message types.TtnMapperUplinkMessage) bool {
	if c.applicationId == "" && c.deviceId == "" && c.userId == "" && c.experiment == "" {
		return false
	}
	return (c.applicationId == "" || c.applicationId == message.AppID) &&
		(c.deviceId == "" || c.deviceId == message.DevID) &&
		(c.userId == "" || c.userId == message.UserId) &&
		(c.experiment == "" || c.experiment == message.Experiment)
}
func (h *Hub) deliver(message types.TtnMapperUplinkMessage) {
	data, err := marshalLiveMessage(message)
	if err != nil {
		return
	}
	for client := range h.clients {
		if !client.matches(message) {
			continue
		}
		select {
		case client.send <- data:
		default:
			h.removeClient(client)
		}
	}
}
