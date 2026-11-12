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
	"runtime/debug"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/SENERGY-Platform/notifier/pkg/auth"
	"github.com/SENERGY-Platform/notifier/pkg/log"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	"github.com/gorilla/websocket"
)

type WsSession struct {
	conn *websocket.Conn
	// token is written by the session's read loop and read by notification goroutines.
	token atomic.Pointer[auth.Token]
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
			log.Logger.Error("controller removeSession failed", attributes.ErrorKey, err)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	err := this.startPing(ctx, &session)
	if err != nil {
		log.Logger.Error("start ws ping failed", attributes.ErrorKey, err)
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
					log.Logger.Error("unexpected websocket error", "session_id", session.id, attributes.ErrorKey, err)
					return
				}
			}
			if t == websocket.CloseMessage {
				return
			}
			err = json.Unmarshal(bytes, &msg)
			if err != nil {
				if this.config.Debug {
					log.Logger.Debug("ignore client ws message of unknown structure")
				}
				continue
			}

			switch msg.Type {
			case model.WsAuthType:
				err = this.handleWsAuth(&session, msg)
				if err != nil {
					log.Logger.Error("handleWsAuth failed", attributes.ErrorKey, err)
					return
				}
			case model.WsRefreshType:
				err = this.handleWsRefresh(&session)
				if err != nil {
					log.Logger.Error("handleWsRefresh failed", attributes.ErrorKey, err)
					return
				}
			default:
				if this.config.Debug {
					log.Logger.Debug("ignore client ws message", "message", msg)
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
	// The session stays in the map while the token is validated, so notifications fanned out meanwhile still reach it.
	var token *auth.Token
	if tokenString, ok := msg.Payload.(string); ok {
		parsed, err := auth.ParseAndValidateToken(tokenString, this.config.JwtSigningKey)
		if err == nil && !parsed.IsExpired() {
			token = &parsed
		}
	}
	if err := this.replaceSessionToken(session, token); err != nil {
		return err
	}
	if token == nil {
		return this.wsSendAuthRequest(session)
	}
	return this.wsSend(session, model.WsAuthOkType, nil)
}

// replaceSessionToken moves the session from the key of its current token to the key of the new one ("" for nil)
// and stores the token in the same critical section. A failed re-auth passes nil, so removeSession keeps finding
// the session under the key it is stored under.
func (this *Controller) replaceSessionToken(session *WsSession, token *auth.Token) error {
	newUser := unauthenticatedUserId
	if token != nil {
		newUser = token.GetUserId()
	}
	this.sessionsMux.Lock()
	defer this.sessionsMux.Unlock()
	oldUser := unauthenticatedUserId
	if old := session.token.Load(); old != nil {
		oldUser = old.GetUserId()
	}
	list := this.sessions[oldUser]
	idx := slices.IndexFunc(list, func(s *WsSession) bool { return s.id == session.id })
	if idx < 0 {
		return errors.New("session not found")
	}
	if oldUser != newUser {
		this.sessions[oldUser] = remove(list, idx)
		this.sessions[newUser] = append(this.sessions[newUser], session)
	}
	session.token.Store(token)
	return nil
}

func (this *Controller) handleWsNotificationUpdate(userId string, notification model.Notification) {
	// A copy taken under the lock, since addSession and removeSession change the map and reorder the slice.
	this.sessionsMux.Lock()
	sessions := slices.Clone(this.sessions[userId])
	this.sessionsMux.Unlock()
	for i := range sessions {
		i := i // thread safety
		go func() {
			if token := sessions[i].token.Load(); token == nil || token.IsExpired() {
				err := this.wsSendAuthRequest(sessions[i])
				if err != nil {
					log.Logger.Error("unable to send auth request", attributes.ErrorKey, err)
				}
				return
			}
			err := this.wsSend(sessions[i], model.WsUpdateSetType, notification)
			if err != nil {
				log.Logger.Error("unable to notify session", "session_id", sessions[i].id, attributes.ErrorKey, err)
			}
		}()
	}
}

func (this *Controller) handleWsNotificationDelete(userId string, ids []string) {
	// A copy taken under the lock, since addSession and removeSession change the map and reorder the slice.
	this.sessionsMux.Lock()
	sessions := slices.Clone(this.sessions[userId])
	this.sessionsMux.Unlock()
	for i := range sessions {
		i := i // thread safety
		go func() {
			if token := sessions[i].token.Load(); token == nil || token.IsExpired() {
				err := this.wsSendAuthRequest(sessions[i])
				if err != nil {
					log.Logger.Error("unable to send auth request", attributes.ErrorKey, err)
				}
				return
			}
			for _, id := range ids {
				err := this.wsSend(sessions[i], model.WsUpdateDeleteType, id)
				if err != nil {
					log.Logger.Error("unable to notify session", "session_id", sessions[i].id, attributes.ErrorKey, err)
				}
			}
		}()
	}
}

func (this *Controller) handleWsRefresh(session *WsSession) error {
	token := session.token.Load()
	if token == nil || token.IsExpired() {
		return this.wsSendAuthRequest(session)
	}
	list, err, _ := this.ListNotifications(*token, persistence.ListOptions{
		Limit:  1000000,
		Offset: 0,
	}, model.ChannelWebsocket)
	if err != nil {
		return err
	}
	return this.wsSend(session, model.WsListType, list.Notifications)
}

func (this *Controller) removeSession(session *WsSession, close bool) (err error) {
	user := ""
	if token := session.token.Load(); token != nil {
		user = token.GetUserId()
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

func (this *Controller) startPing(ctx context.Context, session *WsSession) (err error) {
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
				session.mutex.Lock()
				err := session.conn.WriteMessage(websocket.PingMessage, nil)
				session.mutex.Unlock()
				if err != nil {
					log.Logger.Error("sending ws ping failed", attributes.ErrorKey, err)
					return
				}
			}
		}
	}()
	return nil
}
