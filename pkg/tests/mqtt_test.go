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
	"encoding/json"
	"github.com/SENERGY-Platform/notifier/pkg"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/mqtt"
	paho "github.com/eclipse/paho.mqtt.golang"
	uuid "github.com/satori/go.uuid"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestMQTT(t *testing.T) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := configuration.Load("./../../config.json")
	if err != nil {
		t.Fatal("ERROR: unable to load config", err)
	}

	config.Debug = true

	mongoPort, _, err := MongoContainer(ctx, wg)
	if err != nil {
		t.Error(err)
		return
	}
	config.MongoAddr = "localhost"
	config.MongoPort = mongoPort

	config.PlatformMqttAddress, err = MqttContainer(ctx, wg)
	if err != nil {
		t.Error(err)
		return
	}

	freePort, err := getFreePort()
	if err != nil {
		t.Error(err)
		return
	}
	config.ApiPort = strconv.Itoa(freePort)

	err = pkg.Start(ctx, wg, config)
	if err != nil {
		t.Error(err)
		return
	}

	_, err = setPlatformBroker(config, "user1", model.PlatformBroker{
		Enabled: true,
	})
	if err != nil {
		t.Error(err)
	}
	topic1, topic2, topic3 := "topic1", "topic2", "topic3"

	_, err = createBroker(config, "user1", model.Broker{
		Address: config.PlatformMqttAddress,
		Topic:   topic1,
		Qos:     2,
		Enabled: true,
	})
	if err != nil {
		t.Error(err)
	}

	_, err = createBroker(config, "user1", model.Broker{
		Address: config.PlatformMqttAddress,
		Topic:   topic2,
	})
	if err != nil {
		t.Error(err)
	}

	_, err = createBroker(config, "user2", model.Broker{
		Address: config.PlatformMqttAddress,
		Topic:   topic3,
		Qos:     2,
		Enabled: true,
	})
	if err != nil {
		t.Error(err)
	}

	msgsU1, msgsU2, msgs1, msgs2, msgs3 := []string{}, []string{}, []string{}, []string{}, []string{}
	publisher, err := mqtt.NewPublisher(ctx, config.PlatformMqttAddress, config.PlatformMqttUser,
		config.PlatformMqttPw, "notifier-test-"+uuid.NewV4().String(), config.PlatformMqttQos, config.Debug)
	if err != nil {
		t.Error(err)
	}
	mqttClient := publisher.GetClient()

	mqttClient.Subscribe(config.PlatformMqttBasetopic+"/user1", 1, func(_ paho.Client, message paho.Message) {
		msgsU1 = append(msgsU1, string(message.Payload()))
	})
	mqttClient.Subscribe(config.PlatformMqttBasetopic+"/user2", 1, func(_ paho.Client, message paho.Message) {
		msgsU2 = append(msgsU2, string(message.Payload()))
	})
	mqttClient.Subscribe(topic1, 1, func(_ paho.Client, message paho.Message) {
		msgs1 = append(msgs1, string(message.Payload()))
	})
	mqttClient.Subscribe(topic2, 1, func(_ paho.Client, message paho.Message) {
		msgs2 = append(msgs2, string(message.Payload()))
	})
	mqttClient.Subscribe(topic3, 1, func(_ paho.Client, message paho.Message) {
		msgs3 = append(msgs3, string(message.Payload()))
	})

	test1, err := createNotification(config, "user1", model.Notification{
		Title: "test1",
	})
	if err != nil {
		t.Error(err)
	}
	test1S, err := json.Marshal(test1)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(time.Second)
	if len(msgsU1) != 1 || msgsU1[0] != string(test1S) {
		t.Error("user1 did not receive mqtt notification on platform broker")
	}
	if len(msgsU2) != 0 {
		t.Error("user2 received mqtt notification of user1")
	}
	if len(msgs1) != 1 || msgsU1[0] != string(test1S) {
		t.Error("user1 did not receive mqtt notification on custom broker")
	}
	if len(msgs2) != 0 {
		t.Error("user1 received mqtt notification on disabled custom broker")
	}
	if len(msgs3) != 0 {
		t.Error("user2 received mqtt notification of user1")
	}

	test2, err := createNotification(config, "user2", model.Notification{
		Title: "test1",
	})
	if err != nil {
		t.Error(err)
	}
	test2S, err := json.Marshal(test2)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(time.Second)
	if len(msgsU1) != 1 {
		t.Error("user1 received message of user2 on platform broker")
	}
	if len(msgsU2) != 0 {
		t.Error("user2 received mqtt notification on platform broker, but should be disabled")
	}
	if len(msgs1) != 1 {
		t.Error("user1 received message of user2 on custom broker")
	}
	if len(msgs2) != 0 {
		t.Error("user1 received message of user2 on custom broker")
	}
	if len(msgs3) != 1 || msgs3[0] != string(test2S) {
		t.Error("user2 did not receive mqtt notification on custom broker")
	}
}
