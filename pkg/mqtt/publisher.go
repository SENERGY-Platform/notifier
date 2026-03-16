/*
 * Copyright 2020 InfAI (CC SES)
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

package mqtt

import (
	"context"
	"errors"
	"strings"

	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/SENERGY-Platform/notifier/pkg/log"
	paho "github.com/eclipse/paho.mqtt.golang"
)

type Publisher struct {
	client paho.Client
	qos    byte
}

func NewPublisher(ctx context.Context, broker string, user string, pw string, client string, qos uint8) (mqtt *Publisher, err error) {
	if strings.Index(broker, ":") == -1 {
		broker += ":1883"
	}
	mqtt = &Publisher{qos: qos}
	options := paho.NewClientOptions().
		SetPassword(pw).
		SetUsername(user).
		SetAutoReconnect(true).
		SetCleanSession(true).
		SetClientID(client).
		AddBroker(broker)

	mqtt.client = paho.NewClient(options)
	if token := mqtt.client.Connect(); token.Wait() && token.Error() != nil {
		log.Logger.Error("publisher connect failed", "broker", broker, "user", user, "client", client, attributes.ErrorKey, token.Error())
		return mqtt, token.Error()
	}
	log.Logger.Info("mqtt publisher up and running")
	go func() {
		<-ctx.Done()
		mqtt.client.Disconnect(0)
	}()

	return mqtt, nil
}

func (this *Publisher) Publish(topic string, msg string) (err error) {
	if !this.client.IsConnected() {
		return errors.New("mqtt client not connected")
	}

	token := this.client.Publish(topic, this.qos, false, msg)
		log.Logger.Debug("publish mqtt", "topic", topic, "message", msg)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return err
}

func (this *Publisher) GetClient() paho.Client {
	return this.client
}
