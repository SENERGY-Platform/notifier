/*
 * Copyright 2025 InfAI (CC SES)
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
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SENERGY-Platform/notifier/pkg/auth"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/log"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/gorilla/websocket"
)

// serverConn returns the server side of a websocket connection to a local test server.
func serverConn(t *testing.T) *websocket.Conn {
	t.Helper()
	connCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		connCh <- c
	}))
	t.Cleanup(srv.Close)
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	conn := <-connCh
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// signToken builds an RS256 token by hand so the test does not depend on a jwt library.
func signToken(t *testing.T, key *rsa.PrivateKey, sub string, exp time.Time) string {
	t.Helper()
	enc := base64.RawURLEncoding
	signingInput := enc.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." +
		enc.EncodeToString([]byte(fmt.Sprintf(`{"sub":%q,"exp":%d}`, sub, exp.Unix())))
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signingInput + "." + enc.EncodeToString(sig)
}

func TestWsReauthWithInvalidTokenKeepsSessionRemovable(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	validToken := signToken(t, key, "user-a", time.Now().Add(time.Hour))

	c := &Controller{
		config:   configuration.Config{JwtSigningKey: base64.StdEncoding.EncodeToString(pub)},
		sessions: map[string][]*WsSession{},
	}
	session := &WsSession{conn: serverConn(t), id: "test-session"}
	c.addSession(session, &unauthenticatedUserId)

	if err = c.handleWsAuth(session, model.EventMessage{Type: model.WsAuthType, Payload: validToken}); err != nil {
		t.Fatal(err)
	}
	if len(c.sessions["user-a"]) != 1 || len(c.sessions[""]) != 0 {
		t.Fatalf("after auth: %v", c.sessions)
	}

	if err = c.handleWsAuth(session, model.EventMessage{Type: model.WsAuthType, Payload: "invalid"}); err != nil {
		t.Fatal(err)
	}

	if err = c.removeSession(session, false); err != nil {
		t.Fatal(err)
	}
	for user, sessions := range c.sessions {
		if len(sessions) != 0 {
			t.Errorf("leftover sessions for user %q: %v", user, sessions)
		}
	}
}

// Run with -race: notifications for a user fan out while sessions of that user come and go.
func TestWsNotificationFanOutWhileSessionsChange(t *testing.T) {
	c := &Controller{sessions: map[string][]*WsSession{}}
	user := "user-a"
	token := &auth.Token{Sub: user, Expiration: time.Now().Add(time.Hour).Unix()}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			s := &WsSession{id: strconv.Itoa(i)}
			s.token.Store(token)
			c.addSession(s, &user)
			if err := c.removeSession(s, false); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	for {
		select {
		case <-done:
			return
		default:
			// No ids, so the fan-out only reads the sessions and writes nothing to a connection.
			c.handleWsNotificationDelete(user, nil)
		}
	}
}

// Run with -race: the update fan-out writes to the connections while the sessions of the user are re-added.
func TestWsNotificationUpdateFanOutWhileSessionsChange(t *testing.T) {
	log.InitForTest()
	c := &Controller{sessions: map[string][]*WsSession{}}
	user := "user-a"
	token := &auth.Token{Sub: user, Expiration: time.Now().Add(time.Hour).Unix()}
	var sessions []*WsSession
	for i := 0; i < 10; i++ {
		// one connection per session: gorilla panics on concurrent writes to a single connection
		s := &WsSession{conn: serverConn(t), id: strconv.Itoa(i)}
		s.token.Store(token)
		c.addSession(s, &user)
		sessions = append(sessions, s)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			for _, s := range sessions {
				if err := c.removeSession(s, false); err != nil {
					t.Error(err)
					return
				}
				c.addSession(s, &user)
			}
		}
	}()
	for i := 0; i < 50; i++ {
		c.handleWsNotificationUpdate(user, model.Notification{Title: strconv.Itoa(i)})
	}
	<-done
}

// A notification fanned out during a re-auth must find the session, so it is in the map at every moment.
func TestWsReauthKeepsSessionInMap(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	exp := time.Now().Add(time.Hour)
	payloads := []string{signToken(t, key, "user-a", exp), signToken(t, key, "user-b", exp), "invalid"}

	c := &Controller{
		config:   configuration.Config{JwtSigningKey: base64.StdEncoding.EncodeToString(pub)},
		sessions: map[string][]*WsSession{},
	}
	session := &WsSession{conn: serverConn(t), id: "test-session"}
	c.addSession(session, &unauthenticatedUserId)

	done := make(chan struct{})
	var absent, checks atomic.Int64
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			c.sessionsMux.Lock()
			n := 0
			for _, list := range c.sessions {
				n += len(list)
			}
			c.sessionsMux.Unlock()
			checks.Add(1)
			if n != 1 {
				absent.Add(1)
			}
		}
	}()
	for i := 0; i < 60; i++ {
		msg := model.EventMessage{Type: model.WsAuthType, Payload: payloads[i%len(payloads)]}
		if err = c.handleWsAuth(session, msg); err != nil {
			t.Fatal(err)
		}
	}
	close(done)
	if checks.Load() == 0 {
		t.Skip("the checker never ran")
	}
	if absent.Load() != 0 {
		t.Errorf("session was not in the map exactly once in %d of %d checks", absent.Load(), checks.Load())
	}
}
