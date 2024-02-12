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

package tests

import (
	"context"
	"github.com/SENERGY-Platform/notifier/pkg"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/tests/mock"
	"github.com/golang-jwt/jwt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestInit(t *testing.T) {
	wg, ctx, cancel, conf, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	defer wg.Wait()
	defer cancel()

	err = pkg.Start(ctx, wg, conf)
	if err != nil {
		t.Error(err)
		return
	}

	time.Sleep(time.Second)

	t.Run("empty list", listNotifications(conf, "user1", model.NotificationList{
		Total:  0,
		Limit:  10,
		Offset: 0,
	}))
}

func createToken(userId string) (token string, err error) {
	claims := KeycloakClaims{
		RealmAccess{Roles: []string{}},
		jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Duration(10 * time.Minute)).Unix(),
			Issuer:    "test",
			Subject:   userId,
		},
	}

	jwtoken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	unsignedTokenString, err := jwtoken.SigningString()
	if err != nil {
		return token, err
	}
	tokenString := strings.Join([]string{unsignedTokenString, ""}, ".")
	token = "Bearer " + tokenString
	return token, nil
}

type KeycloakClaims struct {
	RealmAccess RealmAccess `json:"realm_access"`
	jwt.StandardClaims
}

type RealmAccess struct {
	Roles []string `json:"roles"`
}

func setup() (wg *sync.WaitGroup, ctx context.Context, cancel context.CancelFunc, conf configuration.Config, err error) {
	wg = &sync.WaitGroup{}
	ctx, cancel = context.WithCancel(context.Background())

	conf, err = configuration.Load("./../../config.json")
	if err != nil {
		return
	}

	mongoPort, _, err := MongoContainer(ctx, wg)
	if err != nil {
		return
	}
	conf.MongoAddr = "localhost"
	conf.MongoPort = mongoPort

	freePort, err := getFreePort()
	if err != nil {
		return
	}
	conf.ApiPort = strconv.Itoa(freePort)

	err = mock.MockKeycloak(&conf, ctx)
	if err != nil {
		return
	}

	err = mock.MockVault(&conf, ctx)
	if err != nil {
		return
	}

	return
}
