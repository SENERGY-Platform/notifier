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
	"net"
	"sync"
	"time"

	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/SENERGY-Platform/notifier/pkg/log"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func MongoContainer(ctx context.Context, wg *sync.WaitGroup) (hostport string, containerip string, err error) {
	log.Logger.Info("start mongo")
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mongo:8.0",
			ExposedPorts: []string{"27017/tcp"},
			WaitingFor: wait.ForAll(
				wait.ForLog(`"msg":"Waiting for connections"`),
				wait.ForListeningPort("27017/tcp"),
			),
			Tmpfs: map[string]string{"/data/db": "rw"},
		},
		Started: true,
	})
	if err != nil {
		return "", "", err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		err := c.Terminate(context.Background())
		if err != nil {
			log.Logger.Debug("remove container mongo", attributes.ErrorKey, err)
		}
	}()

	containerip, err = c.ContainerIP(ctx)
	if err != nil {
		return "", "", err
	}
	temp, err := c.MappedPort(ctx, "27017/tcp")
	if err != nil {
		return "", "", err
	}
	hostport = temp.Port()

	return hostport, containerip, err
}

func MqttContainer(ctx context.Context, wg *sync.WaitGroup) (brokerAddress string, err error) {
	log.Logger.Info("start mqtt broker")
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:           "eclipse-mosquitto:1.6.12",
			ExposedPorts:    []string{"1883/tcp"},
			WaitingFor:      wait.ForListeningPort("1883/tcp"),
			AlwaysPullImage: true,
		},
		Started: true,
	})
	if err != nil {
		return "", err
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		timeout, _ := context.WithTimeout(context.Background(), 10*time.Second)
		err := c.Terminate(timeout)
		if err != nil {
			log.Logger.Debug("remove container mqtt", attributes.ErrorKey, err)
		}
	}()

	port, err := c.MappedPort(ctx, "1883/tcp")
	if err != nil {
		return "", err
	}
	hostPort := port.Port()

	brokerAddress = "tcp://localhost:" + hostPort

	return brokerAddress, err
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
