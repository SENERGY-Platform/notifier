package mock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/gorilla/mux"
	uuid "github.com/satori/go.uuid"
	"log"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
)

func MockVault(config *configuration.Config, ctx context.Context) (err error) {
	router, err := getVaultRouter()
	if err != nil {
		return err
	}
	server := httptest.NewServer(router)
	config.VaultUrl = server.URL
	go func() {
		<-ctx.Done()
		server.Close()
	}()
	return nil
}

func getVaultRouter() (router *mux.Router, err error) {
	defer func() {
		if r := recover(); r != nil && err == nil {
			log.Printf("%s: %s", r, debug.Stack())
			err = errors.New(fmt.Sprint("Recovered Error: ", r))
		}
	}()
	router = mux.NewRouter()

	token := map[string]interface{}{
		"request_id":     "000",
		"lease_id":       "123",
		"lease_duration": 0,
		"renewable":      true,
		"auth": map[string]interface{}{
			"client_token":   "token",
			"accessor":       "accessor",
			"entity_id":      "123",
			"renewable":      true,
			"orphan":         true,
			"lease_duration": 3600,
		},
	}
	data := make(map[string]map[string]interface{})

	router.HandleFunc("/v1/auth/jwt/login", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		token["request_id"] = uuid.NewV4()
		token["lease_id"] = uuid.NewV4()
		err = json.NewEncoder(writer).Encode(token)
		if err != nil {
			log.Println("ERROR: unable to encode response", err)
		}
	}).Methods(http.MethodPost)

	router.HandleFunc("/v1/auth/token/renew-self", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		requestData := map[string]interface{}{}
		_ = json.NewDecoder(request.Body).Decode(&requestData)
		token["request_id"] = uuid.NewV4()
		token["lease_id"] = uuid.NewV4()
		err = json.NewEncoder(writer).Encode(token)
		if err != nil {
			log.Println("ERROR: unable to encode response", err)
		}
	}).Methods(http.MethodPut)

	router.HandleFunc("/v1/{engine}/data/{key}", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		err = json.NewEncoder(writer).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"data": data[mux.Vars(request)["engine"]][mux.Vars(request)["key"]],
			},
		})
		if err != nil {
			log.Println("ERROR: unable to encode response", err)
		}
	}).Methods(http.MethodGet)

	router.HandleFunc("/v1/{engine}/metadata/{key}", func(writer http.ResponseWriter, request *http.Request) {
		if data[mux.Vars(request)["engine"]] == nil {
			data[mux.Vars(request)["engine"]] = make(map[string]interface{})
		}
		delete(data[mux.Vars(request)["engine"]], mux.Vars(request)["key"])
	}).Methods(http.MethodDelete)

	router.HandleFunc("/v1/{engine}/metadata", func(writer http.ResponseWriter, request *http.Request) {
		keys := []string{}
		for k, _ := range data[mux.Vars(request)["engine"]] {
			keys = append(keys, k)
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		err = json.NewEncoder(writer).Encode(map[string]interface{}{
			"data": map[string]interface{}{"keys": keys},
		})
		if err != nil {
			log.Println("ERROR: unable to encode response", err)
		}
	}).Methods("LIST", http.MethodGet)

	router.HandleFunc("/v1/{engine}/data/{key}", func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		requestData := map[string]interface{}{}
		_ = json.NewDecoder(request.Body).Decode(&requestData)
		if data[mux.Vars(request)["engine"]] == nil {
			data[mux.Vars(request)["engine"]] = make(map[string]interface{})
		}
		data[mux.Vars(request)["engine"]][mux.Vars(request)["key"]] = requestData["data"]
	}).Methods(http.MethodPut)

	return
}
