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
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	"sync"
)

type Controller struct {
	config      configuration.Config
	db          Persistence
	sessionsMux sync.Mutex
	sessions    map[string][]*WsSession
}

func New(config configuration.Config, db Persistence) *Controller {
	return &Controller{
		config:      config,
		db:          db,
		sessionsMux: sync.Mutex{},
		sessions:    make(map[string][]*WsSession),
	}
}

type Persistence interface {
	ListNotifications(userId string, options persistence.ListOptions) (result []model.Notification, total int64, err error, errCode int)
	ReadNotification(userId string, id string) (result model.Notification, err error, errCode int)
	SetNotification(notification model.Notification) (err error, errCode int)
	RemoveNotifications(userId string, ids []string) (err error, errCode int)

	ListBrokers(userId string, options persistence.ListOptions) (result []model.Broker, total int64, err error, errCode int)
	ReadBroker(userId string, id string) (result model.Broker, err error, errCode int)
	SetBroker(broker model.Broker) (err error, errCode int)
	RemoveBrokers(userId string, ids []string) (err error, errCode int)

	ReadPlatformBroker(userId string) (platformBroker model.PlatformBroker, err error, errCode int)
	SetPlatformBroker(platformBroker model.PlatformBroker) (err error, errCode int)
	RemovePlatformBroker(userId string) (err error, errCode int)
}
