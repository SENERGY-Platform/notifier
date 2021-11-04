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

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SENERGY-Platform/notifier/pkg/auth"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	"github.com/gorilla/websocket"
	"log"
	"runtime/debug"
	"sync"
	"time"
)

type WsSession struct {
	conn  *websocket.Conn
	token *auth.Token
	id    string
	mutex sync.Mutex
}

var unauthenticatedUserId = ""

func (this *Controller) HandleWs(conn *websocket.Conn) {
	connId := conn.RemoteAddr().String()
	session := WsSession{
		conn:  conn,
		id:    connId,
		mutex: sync.Mutex{},
	}
	this.addSession(&session, &unauthenticatedUserId) // userId not known yet
	defer func() {
		err := this.removeSession(&session, true)
		if err != nil {
			log.Println("ERROR: Controller.removeSession", err)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	err := this.startPing(ctx, conn)
	if err != nil {
		log.Println("ERROR:", err)
		debug.PrintStack()
		cancel()
		return
	}

	go func() {
		defer cancel()
		for {
			msg := model.EventMessage{}
			t, bytes, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					return
				} else {
					log.Println("Unexpected error", session.id, err.Error())
					return
				}
			}
			if t == websocket.CloseMessage {
				return
			}
			err = json.Unmarshal(bytes, &msg)
			if err != nil {
				if this.config.Debug {
					log.Println("DEBUG: ignore client ws message of unknown structure")
				}
				continue
			}

			switch msg.Type {
			case model.WsAuthType:
				err = this.handleWsAuth(&session, msg)
				if err != nil {
					log.Println("ERROR: handleWsAuth:", err)
					return
				}
			case model.WsRefreshType:
				err = this.handleWsRefresh(&session)
				if err != nil {
					log.Println("ERROR: handleWsRefresh:", err)
					return
				}
			default:
				if this.config.Debug {
					log.Println("DEBUG: ignore client ws message", msg)
				}
			}
		}
	}()
	<-ctx.Done()
}

func (this *Controller) wsSend(session *WsSession, wsType string, payload interface{}) error {
	session.mutex.Lock()
	defer session.mutex.Unlock()
	return session.conn.WriteJSON(model.EventMessage{
		Type:    wsType,
		Payload: payload,
	})
}

func (this *Controller) wsSendAuthRequest(session *WsSession) error {
	return this.wsSend(session, model.WsAuthRequestType, nil)
}

func (this *Controller) handleWsAuth(session *WsSession, msg model.EventMessage) error {
	user := unauthenticatedUserId
	defer this.addSession(session, &user)
	err := this.removeSession(session, false)
	tokenString, ok := msg.Payload.(string)
	if !ok {
		return this.wsSendAuthRequest(session)
	}
	token, err := auth.ParseAndValidateToken(tokenString, this.config.JwtSigningKey)
	if err != nil {
		return this.wsSendAuthRequest(session)
	}
	if token.IsExpired() {
		return this.wsSendAuthRequest(session)
	}

	session.token = &token
	user = session.token.GetUserId()
	err = this.wsSend(session, model.WsAuthOkType, nil)
	return err
}

func (this *Controller) handleWsNotificationUpdate(userId string, notification model.Notification) {
	sessions, ok := this.sessions[userId]
	if !ok {
		return
	}
	for i := range sessions {
		i := i // thread safety
		go func() {
			if sessions[i].token == nil || sessions[i].token.IsExpired() {
				err := this.wsSendAuthRequest(sessions[i])
				if err != nil {
					log.Println("ERROR: unable to send auth request", err)
				}
				return
			}
			err := this.wsSend(sessions[i], model.WsUpdateSetType, notification)
			if err != nil {
				log.Println("ERROR: unable to notify session", sessions[i].id)
			}
		}()
	}
}

func (this *Controller) handleWsNotificationDelete(userId string, ids []string) {
	sessions, ok := this.sessions[userId]
	if !ok {
		return
	}
	for i := range sessions {
		i := i // thread safety
		go func() {
			if sessions[i].token == nil || sessions[i].token.IsExpired() {
				err := this.wsSendAuthRequest(sessions[i])
				if err != nil {
					log.Println("ERROR: unable to send auth request", err)
				}
				return
			}
			for _, id := range ids {
				err := this.wsSend(sessions[i], model.WsUpdateDeleteType, id)
				if err != nil {
					log.Println("ERROR: unable to notify session", sessions[i].id)
				}
			}
		}()
	}
}

func (this *Controller) handleWsRefresh(session *WsSession) error {
	if session.token == nil || session.token.IsExpired() {
		return this.wsSendAuthRequest(session)
	}
	list, err, _ := this.ListNotifications(*session.token, persistence.ListOptions{
		Limit:  1000000,
		Offset: 0,
	})
	if err != nil {
		return err
	}
	return this.wsSend(session, model.WsListType, list.Notifications)
}

func (this *Controller) removeSession(session *WsSession, close bool) (err error) {
	user := ""
	if session.token != nil {
		user = session.token.GetUserId()
	}
	this.sessionsMux.Lock()
	defer this.sessionsMux.Unlock()
	for i := range this.sessions[user] {
		if this.sessions[user][i].id == session.id {
			this.sessions[user] = remove(this.sessions[user], i)
			if close {
				return session.conn.Close()
			}
			return nil
		}
	}

	return errors.New("session not found")
}

func remove(s []*WsSession, i int) []*WsSession {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func (this *Controller) addSession(session *WsSession, userId *string) {
	this.sessionsMux.Lock()
	defer this.sessionsMux.Unlock()
	l, ok := this.sessions[*userId]
	if !ok {
		l = []*WsSession{}
	}
	this.sessions[*userId] = append(l, session)
}

func (this *Controller) startPing(ctx context.Context, conn *websocket.Conn) (err error) {
	pingPeriod, err := time.ParseDuration(this.config.WsPingPeriod)
	if err != nil {
		return err
	}
	go func() {
		ticker := time.NewTicker(pingPeriod)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := conn.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					log.Println("ERROR: sending ws ping:", err)
					return
				}
			}
		}
	}()
	return nil
}
