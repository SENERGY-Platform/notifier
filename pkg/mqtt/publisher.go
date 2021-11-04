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
	paho "github.com/eclipse/paho.mqtt.golang"
	"log"
	"strings"
)

type Publisher struct {
	client paho.Client
	qos    byte
	debug  bool
}

func NewPublisher(ctx context.Context, broker string, user string, pw string, client string, qos uint8, debug bool) (mqtt *Publisher, err error) {
	if strings.Index(broker, ":") == -1 {
		broker += ":1883"
	}
	mqtt = &Publisher{debug: debug, qos: qos}
	options := paho.NewClientOptions().
		SetPassword(pw).
		SetUsername(user).
		SetAutoReconnect(true).
		SetCleanSession(true).
		SetClientID(client).
		AddBroker(broker)

	mqtt.client = paho.NewClient(options)
	if token := mqtt.client.Connect(); token.Wait() && token.Error() != nil {
		log.Println("Error on Publisher.Connect(): ", broker, user, pw, client, token.Error())
		return mqtt, token.Error()
	}
	log.Println("MQTT publisher up and running...")
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
	if this.debug {
		log.Printf("Publish Mqtt on topic %v: %v", topic, msg)
	}
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return err
}

func (this *Publisher) GetClient() paho.Client {
	return this.client
}
