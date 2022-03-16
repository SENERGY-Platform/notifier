package api

import (
	"github.com/SENERGY-Platform/notifier/pkg/auth"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/gorilla/mux"
	"net/http"
)

func init() {
	endpoints = append(endpoints, FcmTokenEndpoints)
}

func FcmTokenEndpoints(_ configuration.Config, control Controller, router *mux.Router) {
	resource := "/fcm-tokens"

	router.HandleFunc(resource+"/{fcmToken}", func(writer http.ResponseWriter, request *http.Request) {
		token, err := auth.GetParsedToken(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		fcmToken := mux.Vars(request)["fcmToken"]

		err, errCode := control.PutFcmToken(token, fcmToken)
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}

	}).Methods(http.MethodPut, http.MethodOptions)

	router.HandleFunc(resource+"/{fcmToken}", func(writer http.ResponseWriter, request *http.Request) {
		token, err := auth.GetParsedToken(request)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		fcmToken := mux.Vars(request)["fcmToken"]

		err, errCode := control.DeleteFcmToken(token, fcmToken)
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}
	}).Methods(http.MethodDelete)
}
