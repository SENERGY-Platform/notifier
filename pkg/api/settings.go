/*
 * Copyright 2024 InfAI (CC SES)
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
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func init() {
	endpoints = append(endpoints, PlatformBrokerEndpoints)
}

func SettingsEndpoints(_ configuration.Config, control Controller, router *mux.Router) {
	resource := "/settings"

	router.HandleFunc(resource, func(writer http.ResponseWriter, request *http.Request) {
		token, err := auth.GetParsedToken(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}

		result, err, errCode := control.GetSettings(token)
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
		var settings model.Settings
		err = json.NewDecoder(request.Body).Decode(&settings)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		result, err, errCode := control.SetSettings(token, settings)
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
