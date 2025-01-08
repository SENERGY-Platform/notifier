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

package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/SENERGY-Platform/notifier/pkg/auth"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	mailpit "github.com/axllent/mailpit/server/apiv1"
)

type FromTo = struct {
	Name  string
	Email string
}

type SendRequest mailpit.SendRequest

func (s *SendRequest) Send(remoteAddress string) (messageId string, err error) {
	body, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest(http.MethodPost, remoteAddress+"/api/v1/send", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode > 299 {
		return "", fmt.Errorf("unexpected statuscode %v : %v", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil

}

func (this *Controller) handleEmailNotificationUpdate(token auth.Token, notification model.Notification) {
	if len(token.Email) == 0 || !token.EmailVerified {
		return
	}
	email := SendRequest{
		To: []FromTo{{
			Email: token.Email,
		}},
		From: FromTo{
			Email: this.config.EmailFrom,
		},
		Subject: notification.Title,
		Text:    notification.Message,
	}
	_, err := email.Send(this.config.MailpitHostPort)
	if err != nil {
		log.Println("ERROR: Sending Email failed: " + err.Error())
	}
}
