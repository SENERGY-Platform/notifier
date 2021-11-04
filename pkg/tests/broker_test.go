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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/SENERGY-Platform/notifier/pkg"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"io"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestBrokerCRUD(t *testing.T) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := configuration.Load("./../../config.json")
	if err != nil {
		t.Fatal("ERROR: unable to load config", err)
	}

	mongoPort, _, err := MongoContainer(ctx, wg)
	if err != nil {
		t.Error(err)
		return
	}
	config.MongoAddr = "localhost"
	config.MongoPort = mongoPort

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

	t.Run("empty list", listBrokers(config, "user1", model.BrokerList{
		Total:   0,
		Limit:   10,
		Offset:  0,
		Brokers: []model.Broker{},
	}))

	test1, err := createBroker(config, "user1", model.Broker{
		Address:  "test1:1883",
		User:     "test1",
		Password: "testpw",
		Topic:    "iuahsdi/qiwndpin",
	})
	if err != nil {
		t.Error(err)
	}

	err = readBroker(config, "user1", test1.Id, test1)
	if err != nil {
		t.Error(err)
	}

	test2, err := createBroker(config, "user1", model.Broker{
		Address:  "test2:1883",
		User:     "test1",
		Password: "testpw",
		Topic:    "iuahsdi/qiwndpin",
	})
	if err != nil {
		t.Error(err)
	}
	err = readBroker(config, "user1", test2.Id, test2)
	if err != nil {
		t.Error(err)
	}

	test3, err := createBroker(config, "user2", model.Broker{
		Address: "test3",
	})
	if err != nil {
		t.Error(err)
	}
	err = readBroker(config, "user2", test3.Id, test3)
	if err != nil {
		t.Error(err)
	}

	t.Run("list user1", listBrokers(config, "user1", model.BrokerList{
		Total:   2,
		Limit:   10,
		Offset:  0,
		Brokers: []model.Broker{test1, test2},
	}))

	test1.Password = "newpassword"
	test1, err = updateBroker(config, "user1", test1)
	if err != nil {
		t.Error(err)
	}
	if test1.Password != "newpassword" {
		t.Error("update not executed")
	}

	t.Run("list user1 after update", listBrokers(config, "user1", model.BrokerList{
		Total:   2,
		Limit:   10,
		Offset:  0,
		Brokers: []model.Broker{test1, test2},
	}))

	err = deleteBroker(config, "user1", test2.Id)
	if err != nil {
		t.Error(err)
	}

	t.Run("list user1 after delete", listBrokers(config, "user1", model.BrokerList{
		Total:   1,
		Limit:   10,
		Offset:  0,
		Brokers: []model.Broker{test1},
	}))

	// Try disallowed actions

	_, err = createBroker(config, "user2", model.Broker{
		Id: "1234",
	})
	if err == nil {
		t.Error("was allowed to specified ID")
	}

	err = readBroker(config, "user2", test2.Id, test2)
	if err == nil {
		t.Error("was allowed to read from another user")
	}

	test2, err = updateBroker(config, "user2", test2)
	if err == nil {
		t.Error("was allowed to update from another user")
	}

	err = deleteBroker(config, "user2", test2.Id)
	if err == nil {
		t.Error("was allowed to delete from another user")
	}
}

func listBrokers(config configuration.Config, userId string, expected model.BrokerList) func(t *testing.T) {
	return func(t *testing.T) {
		token, err := createToken(userId)
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequest("GET", "http://localhost:"+config.ApiPort+"/brokers?limit=10", nil)
		if err != nil {
			t.Error(err)
			return
		}
		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		req.WithContext(ctx)
		req.Header.Set("Authorization", token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Error(resp.StatusCode, string(b))
			return
		}
		actual := model.BrokerList{}
		err = json.NewDecoder(resp.Body).Decode(&actual)
		if err != nil {
			t.Error(err)
			return
		}

		if !actual.Equal(expected) {
			t.Error(actual, expected)
			return
		}
	}
}

func createBroker(config configuration.Config, userId string, broker model.Broker) (result model.Broker, err error) {
	var token string
	token, err = createToken(userId)
	if err != nil {
		return
	}
	b := new(bytes.Buffer)
	err = json.NewEncoder(b).Encode(broker)
	if err != nil {
		return
	}
	req, err := http.NewRequest("POST", "http://localhost:"+config.ApiPort+"/brokers", b)
	if err != nil {
		return
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	req.WithContext(ctx)
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return result, errors.New(string(b))
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return
}

func updateBroker(config configuration.Config, userId string, broker model.Broker) (result model.Broker, err error) {
	token, err := createToken(userId)
	if err != nil {
		return result, err
	}
	b := new(bytes.Buffer)
	err = json.NewEncoder(b).Encode(broker)
	if err != nil {
		return result, err
	}
	req, err := http.NewRequest("PUT", "http://localhost:"+config.ApiPort+"/brokers/"+broker.Id, b)
	if err != nil {
		return result, err
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	req.WithContext(ctx)
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result, err
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return result, errors.New(string(b))
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return result, err
	}
	return
}

func readBroker(config configuration.Config, userId string, id string, expected model.Broker) error {
	token, err := createToken(userId)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("GET", "http://localhost:"+config.ApiPort+"/brokers/"+id, nil)
	if err != nil {
		return err
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	req.WithContext(ctx)
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return errors.New(string(b))
	}
	actual := model.Broker{}
	err = json.NewDecoder(resp.Body).Decode(&actual)
	if err != nil {
		return err
	}

	if !actual.Equal(expected) {
		return errors.New("actual not expected")
	}
	return nil
}

func deleteBroker(config configuration.Config, userId string, id string) error {
	token, err := createToken(userId)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("DELETE", "http://localhost:"+config.ApiPort+"/brokers/"+id, nil)
	if err != nil {
		return err
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	req.WithContext(ctx)
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return errors.New(string(b))
	}
	return nil
}
