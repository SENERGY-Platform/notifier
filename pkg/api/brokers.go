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
	endpoints = append(endpoints, BrokersEndpoints)
}

func BrokersEndpoints(_ configuration.Config, control Controller, router *mux.Router) {
	resource := "/brokers"

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

		result, err, errCode := control.ListBrokers(token, options)
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
		err, errCode := control.DeleteMultipleBrokers(token, ids)
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}
		writer.WriteHeader(http.StatusOK)
		return
	}).Methods(http.MethodDelete)

	router.HandleFunc(resource, func(writer http.ResponseWriter, request *http.Request) {
		token, err := auth.GetParsedToken(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		broker := model.Broker{}
		err = json.NewDecoder(request.Body).Decode(&broker)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		result, err, errCode := control.CreateBroker(token, broker)
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
	}).Methods(http.MethodPost)

	router.HandleFunc(resource+"/{id}", func(writer http.ResponseWriter, request *http.Request) {
		id := mux.Vars(request)["id"]
		token, err := auth.GetParsedToken(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		result, err, errCode := control.ReadBroker(token, id)
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

	router.HandleFunc(resource+"/{id}", func(writer http.ResponseWriter, request *http.Request) {
		id := mux.Vars(request)["id"]
		token, err := auth.GetParsedToken(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		err, errCode := control.DeleteMultipleBrokers(token, []string{id})
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}
		writer.WriteHeader(http.StatusOK)
		return
	}).Methods(http.MethodDelete)

	router.HandleFunc(resource+"/{id}", func(writer http.ResponseWriter, request *http.Request) {
		id := mux.Vars(request)["id"]
		broker := model.Broker{}
		err := json.NewDecoder(request.Body).Decode(&broker)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if broker.Id == "" || id == "" {
			http.Error(writer, "empty id not allowed", http.StatusBadRequest)
			return
		}
		if broker.Id != id {
			http.Error(writer, "expect path id == body._id", http.StatusBadRequest)
			return
		}
		token, err := auth.GetParsedToken(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		broker.UserId = token.GetUserId()
		result, err, errCode := control.SetBroker(token, broker)
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
	}).Methods(http.MethodPut)

}
