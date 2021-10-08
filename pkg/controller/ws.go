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
)

type WsSession struct {
	conn  *websocket.Conn
	token *auth.Token
	id    string
}

var unauthenticatedUserId = ""

func (this *Controller) HandleWs(conn *websocket.Conn) {
	connId := conn.RemoteAddr().String()
	session := WsSession{
		conn: conn,
		id:   connId,
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
			if err != nil && websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived, websocket.CloseGoingAway) {
				return
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

func (this *Controller) wsSend(conn *websocket.Conn, wsType string, payload interface{}) error {
	return conn.WriteJSON(model.EventMessage{
		Type:    wsType,
		Payload: payload,
	})
}

func (this *Controller) wsSendAuthRequest(conn *websocket.Conn) error {
	return this.wsSend(conn, model.WsAuthRequestType, nil)
}

func (this *Controller) handleWsAuth(session *WsSession, msg model.EventMessage) error {
	user := unauthenticatedUserId
	defer this.addSession(session, &user)
	err := this.removeSession(session, false)
	tokenString, ok := msg.Payload.(string)
	if !ok {
		return this.wsSendAuthRequest(session.conn)
	}
	token, err := auth.ParseAndValidateToken(tokenString, this.config.JwtSigningKey)
	if err != nil {
		return this.wsSendAuthRequest(session.conn)
	}
	if token.IsExpired() {
		return this.wsSendAuthRequest(session.conn)
	}

	session.token = &token
	user = session.token.GetUserId()
	err = this.wsSend(session.conn, model.WsAuthOkType, nil)
	return err
}

func (this *Controller) handleWsNotificationUpdate(userId string, notification model.Notification) {
	sessions, ok := this.sessions[userId]
	if !ok {
		return
	}
	for _, session := range sessions {
		session := session // thread safety
		go func() {
			if session.token == nil || session.token.IsExpired() {
				err := this.wsSendAuthRequest(session.conn)
				if err != nil {
					log.Println("ERROR: unable to send auth request", err)
				}
				return
			}
			err := this.wsSend(session.conn, model.WsUpdateSetType, notification)
			if err != nil {
				log.Println("ERROR: unable to notify session", session.id)
			}
		}()
	}
}

func (this *Controller) handleWsRefresh(session *WsSession) error {
	if session.token == nil || session.token.IsExpired() {
		return this.wsSendAuthRequest(session.conn)
	}
	list, err, _ := this.ListNotifications(*session.token, persistence.ListOptions{
		Limit:  1000000,
		Offset: 0,
	})
	if err != nil {
		return err
	}
	return this.wsSend(session.conn, model.WsListType, list.Notifications)
}

func (this *Controller) removeSession(session *WsSession, close bool) (err error) {
	user := ""
	if session.token != nil {
		user = session.token.GetUserId()
	}
	this.sessionsMux.Lock()
	defer this.sessionsMux.Unlock()
	for i, s := range this.sessions[user] {
		if s.id == session.id {
			this.sessions[user] = remove(this.sessions[user], i)
			if close {
				return s.conn.Close()
			}
			return nil
		}
	}

	return errors.New("session not found")
}

func remove(s []WsSession, i int) []WsSession {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func (this *Controller) addSession(session *WsSession, userId *string) {
	this.sessionsMux.Lock()
	defer this.sessionsMux.Unlock()
	l, ok := this.sessions[*userId]
	if !ok {
		l = []WsSession{}
	}
	this.sessions[*userId] = append(l, *session)
}
