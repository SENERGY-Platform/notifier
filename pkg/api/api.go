/*
 * Copyright 2019 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api

import (
	"context"
	"github.com/SENERGY-Platform/notifier/pkg/api/util"
	"github.com/SENERGY-Platform/notifier/pkg/auth"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"reflect"
	"runtime"
	"sync"
	"time"
)

var endpoints = []func(config configuration.Config, control Controller, router *mux.Router){}

func Start(ctx context.Context, wg *sync.WaitGroup, config configuration.Config, control Controller) {
	log.Println("start api")
	router := mux.NewRouter().StrictSlash(true)
	for _, e := range endpoints {
		log.Println("add endpoints: " + runtime.FuncForPC(reflect.ValueOf(e).Pointer()).Name())
		e(config, control, router)
	}
	log.Println("add logging and cors")
	router.Use(util.CorsMiddleware, util.LoggerMiddleware)
	server := &http.Server{Addr: ":" + config.ApiPort, Handler: router, WriteTimeout: 10 * time.Second, ReadTimeout: 2 * time.Second, ReadHeaderTimeout: 2 * time.Second}
	wg.Add(1)
	go func() {
		log.Println("Listening on ", server.Addr)
		if err := server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				log.Println("ERROR: api server error", err)
				log.Fatal(err)
			} else {
				log.Println("closing api server")
			}
			wg.Done()
		}
	}()

	go func() {
		<-ctx.Done()
		log.Println("DEBUG: api shutdown", server.Shutdown(context.Background()))
	}()
}

type Controller interface {
	ListNotifications(token auth.Token, options persistence.ListOptions, channel model.Channel) (result model.NotificationList, err error, errCode int)
	ReadNotification(token auth.Token, id string) (result model.Notification, err error, errCode int)
	CreateNotification(token *auth.Token, notification model.Notification, ignoreDuplicatesWithinSeconds *int64) (result model.Notification, err error, errCode int)
	SetNotification(token auth.Token, notification model.Notification) (result model.Notification, err error, errCode int)
	DeleteMultipleNotifications(token auth.Token, ids []string) (err error, errCode int)
	HandleWs(conn *websocket.Conn)

	ListBrokers(token auth.Token, options persistence.ListOptions) (result model.BrokerList, err error, errCode int)
	ReadBroker(token auth.Token, id string) (result model.Broker, err error, errCode int)
	CreateBroker(token auth.Token, broker model.Broker) (result model.Broker, err error, errCode int)
	SetBroker(token auth.Token, broker model.Broker) (result model.Broker, err error, errCode int)
	DeleteMultipleBrokers(token auth.Token, ids []string) (err error, errCode int)

	GetPlatformBroker(token auth.Token) (platformBroker model.PlatformBroker, err error, errCode int)
	SetPlatformBroker(token auth.Token, platformBroker model.PlatformBroker) (result model.PlatformBroker, err error, errCode int)
	DeletePlatformBroker(token auth.Token) (err error, errCode int)

	PutFcmToken(token auth.Token, fcmToken string) (err error, errCode int)
	DeleteFcmToken(token auth.Token, fcmToken string) (err error, errCode int)

	GetSettings(token auth.Token) (settings model.Settings, err error, errCode int)
	SetSettings(token auth.Token, settings model.Settings) (result model.Settings, err error, errCode int)
}
