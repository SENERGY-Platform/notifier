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
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/mqtt"
	"github.com/google/uuid"
	"log"
	"net/http"
)

func (this *Controller) handleMqttNotificationUpdate(userId string, notification model.Notification) {
	go this.handlerMqttPlatformBroker(userId, notification)
	brokers, err := this.db.ListEnabledBrokers(userId)
	if err != nil {
		log.Println("ERROR:", err.Error())
		return
	}
	for _, broker := range brokers {
		broker := broker // thread safety
		go func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			publisher, err := mqtt.NewPublisher(ctx, broker.Address, broker.User, broker.Password, this.config.MqttClientPrefix+uuid.NewString(),
				broker.Qos, this.config.Debug)
			if err != nil {
				log.Println("ERROR:", err.Error())
				return
			}
			publishMqtt(publisher, broker.Topic, notification)
		}()
	}
}

func (this *Controller) handlerMqttPlatformBroker(userId string, notification model.Notification) {
	platformBroker, err, errCode := this.db.ReadPlatformBroker(userId)
	if err != nil {
		if errCode == http.StatusNotFound {
			return
		}
		log.Println("Could not publish to platform broker", err)
		return
	}
	if !platformBroker.Enabled {
		return
	}
	publishMqtt(this.platformMqttPublisher, this.config.PlatformMqttBasetopic+"/"+userId, notification)
}

func publishMqtt(publisher *mqtt.Publisher, topic string, notification model.Notification) {
	if publisher == nil {
		log.Println("Could not publish: publisher nil")
		return
	}
	bytes, err := json.Marshal(notification)
	if err != nil {
		log.Println("ERROR:", err.Error())
		return
	}
	err = publisher.Publish(topic, string(bytes))
	if err != nil {
		log.Println("ERROR:", err.Error())
		return
	}
}
