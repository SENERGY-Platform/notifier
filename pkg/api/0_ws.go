package api

import (
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

func init() {
	endpoints = append(endpoints, WsEndpoints)
}

func WsEndpoints(_ configuration.Config, control Controller, router *mux.Router) {
	resource := "/ws"

	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	router.HandleFunc(resource, func(writer http.ResponseWriter, request *http.Request) {
		c, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			log.Print("ERROR:", err)
			return
		}
		defer c.Close()
		control.HandleWs(c)
	}).Methods(http.MethodGet).Headers("Upgrade", "websocket")

}
