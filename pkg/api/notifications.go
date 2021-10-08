/*
 * Copyright 2021 InfAI (CC SES)
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
	"encoding/json"
	"github.com/SENERGY-Platform/notifier/pkg/auth"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strconv"
)

func init() {
	endpoints = append(endpoints, NotificationsEndpoints)
}

func NotificationsEndpoints(_ configuration.Config, control Controller, router *mux.Router) {
	resource := "/"

	router.HandleFunc(resource, func(writer http.ResponseWriter, request *http.Request) {
		token, err := auth.GetParsedToken(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		options := persistence.ListOptions{}
		limitStr := request.URL.Query().Get("limit")
		if limitStr == "" {
			limitStr = "100"
		}
		options.Limit, err = strconv.Atoi(limitStr)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		offsetStr := request.URL.Query().Get("offset")
		if offsetStr == "" {
			offsetStr = "0"
		}
		options.Offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}

		result, err, errCode := control.ListNotifications(token, options)
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		err = json.NewEncoder(writer).Encode(result)
		if err != nil {
			log.Println("ERROR: unable to encode response", err)
		}
		return
	}).Methods(http.MethodGet, http.MethodOptions)

	router.HandleFunc(resource, func(writer http.ResponseWriter, request *http.Request) {
		token, err := auth.GetParsedToken(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		ids := []string{}
		err = json.NewDecoder(request.Body).Decode(&ids)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		err, errCode := control.DeleteMultipleNotifications(token, ids)
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}
		writer.WriteHeader(http.StatusOK)
		return
	}).Methods(http.MethodDelete, http.MethodOptions)

	router.HandleFunc(resource, func(writer http.ResponseWriter, request *http.Request) {
		notification := model.Notification{}
		err := json.NewDecoder(request.Body).Decode(&notification)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		var token auth.Token
		if len(auth.GetAuthToken(request)) > 0 {
			token, err = auth.GetParsedToken(request)
			if err != nil {
				http.Error(writer, err.Error(), http.StatusUnauthorized)
				return
			}
		} else {
			// access without token = admin
			token = auth.Token{
				Sub: notification.UserId,
			}
		}
		if notification.UserId == "" {
			notification.UserId = token.GetUserId()
		}
		if token.GetUserId() != notification.UserId {
			http.Error(writer, "You may only send messages to yourself", http.StatusUnauthorized)
			return
		}
		result, err, errCode := control.CreateNotification(token, notification)
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		err = json.NewEncoder(writer).Encode(result)
		if err != nil {
			log.Println("ERROR: unable to encode response", err)
		}
		return
	}).Methods(http.MethodPut, http.MethodPost, http.MethodOptions) //legacy uses PUT

	router.HandleFunc(resource+"{id}", func(writer http.ResponseWriter, request *http.Request) {
		id := mux.Vars(request)["id"]
		token, err := auth.GetParsedToken(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		result, err, errCode := control.ReadNotification(token, id)
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		err = json.NewEncoder(writer).Encode(result)
		if err != nil {
			log.Println("ERROR: unable to encode response", err)
		}
		return
	}).Methods(http.MethodGet, http.MethodOptions)

	router.HandleFunc(resource+"{id}", func(writer http.ResponseWriter, request *http.Request) {
		id := mux.Vars(request)["id"]
		token, err := auth.GetParsedToken(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		err, errCode := control.DeleteMultipleNotifications(token, []string{id})
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}
		writer.WriteHeader(http.StatusOK)
		return
	}).Methods(http.MethodDelete, http.MethodOptions)

	router.HandleFunc(resource+"{id}", func(writer http.ResponseWriter, request *http.Request) {
		id := mux.Vars(request)["id"]
		notification := model.Notification{}
		err := json.NewDecoder(request.Body).Decode(&notification)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if notification.Id == "" {
			notification.Id = id
		}
		if notification.Id != id {
			http.Error(writer, "expect path id == body._id", http.StatusBadRequest)
			return
		}
		token, err := auth.GetParsedToken(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		result, err, errCode := control.SetNotification(token, notification)
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		err = json.NewEncoder(writer).Encode(result)
		if err != nil {
			log.Println("ERROR: unable to encode response", err)
		}
		return
	}).Methods(http.MethodPut, http.MethodPost, http.MethodOptions) //legacy uses POST

}
