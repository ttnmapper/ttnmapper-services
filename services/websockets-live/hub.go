// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "encoding/json"

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-messageChannel:
			for client := range h.clients {

				if client.applicationId == "" && client.deviceId == "" && client.userId == "" && client.experiment == "" {
					// Need to have at least one filter
					continue
				}

				if client.applicationId != "" {
					if message.AppID != client.applicationId {
						continue
					}
				}
				if client.deviceId != "" {
					if message.DevID != client.deviceId {
						continue
					}
				}
				if client.userId != "" {
					if message.UserId != client.userId {
						continue
					}
				}

				if client.experiment != "" {
					if message.Experiment != client.experiment {
						continue
					}
				}

				data, err := json.Marshal(message)
				if err != nil {
					continue
				}

				select {
				case client.send <- data:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
