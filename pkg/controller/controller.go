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
	"log"
	"sync"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/mqtt"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	"github.com/SENERGY-Platform/vault-jwt-go/vault/vaultjwt"
	"github.com/google/uuid"
)

type Controller struct {
	config                configuration.Config
	db                    Persistence
	sessionsMux           sync.Mutex
	sessions              map[string][]*WsSession
	platformMqttPublisher *mqtt.Publisher
	firebaseClient        *messaging.Client
	clientToken           *vaultjwt.OpenidToken
}

func New(config configuration.Config, db Persistence, ctx context.Context) (*Controller, error) {
	var publisher *mqtt.Publisher
	if config.PlatformMqttAddress != "" && config.PlatformMqttAddress != "-" {
		var err error
		publisher, err = mqtt.NewPublisher(context.Background(), config.PlatformMqttAddress, config.PlatformMqttUser,
			config.PlatformMqttPw, config.MqttClientPrefix+uuid.NewString(), config.PlatformMqttQos, config.Debug)
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	var firebaseClient *messaging.Client

	if config.FcmProjectId != "" && config.FcmIamId != "" {
		firebaseApp, err := firebase.NewApp(context.Background(), &firebase.Config{
			ProjectID:        config.FcmProjectId,
			ServiceAccountID: config.FcmIamId,
		})
		if err != nil {
			log.Fatal(err.Error())
		}
		firebaseClient, err = firebaseApp.Messaging(context.Background())
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	c := &Controller{
		config:                config,
		db:                    db,
		sessionsMux:           sync.Mutex{},
		sessions:              make(map[string][]*WsSession),
		platformMqttPublisher: publisher,
		firebaseClient:        firebaseClient,
		clientToken:           &vaultjwt.OpenidToken{},
	}

	return c, nil
}

type Persistence interface {
	ListNotifications(userId string, options persistence.ListOptions) (result []model.Notification, total int64, err error, errCode int)
	ReadNotification(userId string, id string) (result model.Notification, err error, errCode int)
	ReadNotificationByHash(userId string, hash [32]byte, notOlderThan time.Time) (result model.Notification, err error, errCode int)
	SetNotification(notification model.Notification) (err error, errCode int)
	RemoveNotifications(userId string, ids []string) (err error, errCode int)

	ListBrokers(userId string, options persistence.ListOptions) (result []model.Broker, total int64, err error, errCode int)
	ListEnabledBrokers(userId string) (result []model.Broker, err error)
	ReadBroker(userId string, id string) (result model.Broker, err error, errCode int)
	SetBroker(broker model.Broker) (err error, errCode int)
	RemoveBrokers(userId string, ids []string) (err error, errCode int)

	ReadPlatformBroker(userId string) (platformBroker model.PlatformBroker, err error, errCode int)
	SetPlatformBroker(platformBroker model.PlatformBroker) (err error, errCode int)
	RemovePlatformBroker(userId string) (err error, errCode int)

	UpsertFcmToken(token model.FcmToken) (err error, errCode int)
	DeleteFcmToken(token model.FcmToken) (err error, errCode int)
	GetFcmTokens(userId string) (tokens []model.FcmToken, err error)

	ReadSettings(userId string) (result model.Settings, err error, errCode int)
	SetSettings(settings model.Settings) (error, int)
}
