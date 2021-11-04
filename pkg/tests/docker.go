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
	"errors"
	"github.com/SENERGY-Platform/notifier/pkg/mqtt"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

func MongoContainer(ctx context.Context, wg *sync.WaitGroup) (hostPort string, ipAddress string, err error) {
	pool, err := dockertest.NewPool("")
	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "mongo",
		Tag:        "4.1.11",
	}, func(config *docker.HostConfig) {
		config.Tmpfs = map[string]string{"/data/db": "rw"}
	})
	if err != nil {
		return "", "", err
	}
	wg.Add(1)
	go func() {
		<-ctx.Done()
		log.Println("DEBUG: remove container " + container.Container.Name)
		container.Close()
		wg.Done()
	}()
	hostPort = container.GetPort("27017/tcp")
	err = pool.Retry(func() error {
		log.Println("try mongodb connection...")
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:"+hostPort))
		err = client.Ping(ctx, readpref.Primary())
		return err
	})
	return hostPort, container.Container.NetworkSettings.IPAddress, err
}

func MqttContainer(ctx context.Context, wg *sync.WaitGroup) (brokerAddress string, err error) {
	pool, err := dockertest.NewPool("")
	container, err := pool.Run("erlio/docker-vernemq", "latest", []string{
		"DOCKER_VERNEMQ_ACCEPT_EULA=yes",
		"DOCKER_VERNEMQ_ALLOW_ANONYMOUS=on",
		"DOCKER_VERNEMQ_LOG__CONSOLE__LEVEL=error",
		//"DOCKER_VERNEMQ_SHARED_SUBSCRIPTION_POLICY=random",
		//"DOCKER_VERNEMQ_PLUGINS__VMQ_PASSWD=off",
		//"DOCKER_VERNEMQ_PLUGINS__VMQ_ACL=off",
		//"DOCKER_VERNEMQ_PLUGINS__VMQ_WEBHOOKS=on",
	})
	wg.Add(1)
	go func() {
		<-ctx.Done()
		log.Println("DEBUG: remove container " + container.Container.Name)
		container.Close()
		wg.Done()
	}()
	go Dockerlog(pool, ctx, container, "VERNEMQ")
	brokerAddress = "localhost:" + container.GetPort("1883/tcp")
	err = pool.Retry(func() error {
		log.Println("try mqtt connection...")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		pub, err := mqtt.NewPublisher(ctx, brokerAddress, "", "", "connection-check", 0, true)
		if err != nil {
			return err
		}
		if !pub.GetClient().IsConnected() {
			return errors.New("not connected")
		}
		return nil
	})
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

func Dockerlog(pool *dockertest.Pool, ctx context.Context, repo *dockertest.Resource, name string) {
	out := &LogWriter{logger: log.New(os.Stdout, "["+name+"]", 0)}
	err := pool.Client.Logs(docker.LogsOptions{
		Stdout:       true,
		Stderr:       true,
		Context:      ctx,
		Container:    repo.Container.ID,
		Follow:       true,
		OutputStream: out,
		ErrorStream:  out,
	})
	if err != nil && err != context.Canceled {
		log.Println("DEBUG-ERROR: unable to start docker log", name, err)
	}
}

type LogWriter struct {
	logger *log.Logger
}

func (this *LogWriter) Write(p []byte) (n int, err error) {
	this.logger.Print(string(p))
	return len(p), nil
}
