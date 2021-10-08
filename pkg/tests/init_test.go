package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/SENERGY-Platform/notifier/pkg"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/golang-jwt/jwt"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
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

func TestInit(t *testing.T) {
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

	t.Run("empty list", listNotifications(config, "user1", model.NotificationList{
		Total:  0,
		Limit:  10,
		Offset: 0,
	}))
}

func listNotifications(config configuration.Config, userId string, expected model.NotificationList) func(t *testing.T) {
	return func(t *testing.T) {
		token, err := createToken(userId)
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequest("GET", "http://localhost:"+config.ApiPort+"?limit=10", nil)
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
		actual := model.NotificationList{}
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

func createNotification(config configuration.Config, userId string, notification model.Notification) (result model.Notification, err error) {
	var token string
	token, err = createToken(userId)
	if err != nil {
		return
	}
	b := new(bytes.Buffer)
	err = json.NewEncoder(b).Encode(notification)
	if err != nil {
		return
	}
	req, err := http.NewRequest("POST", "http://localhost:"+config.ApiPort, b)
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

func createNotificationLegacy(config configuration.Config, userId string, notification model.Notification) (result model.Notification, err error) {
	var token string
	token, err = createToken(userId)
	if err != nil {
		return
	}
	b := new(bytes.Buffer)
	err = json.NewEncoder(b).Encode(notification)
	if err != nil {
		return
	}
	req, err := http.NewRequest("PUT", "http://localhost:"+config.ApiPort, b)
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

func updateNotification(config configuration.Config, userId string, notification model.Notification) error {
	token, err := createToken(userId)
	if err != nil {
		return err
	}
	b := new(bytes.Buffer)
	err = json.NewEncoder(b).Encode(notification)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", "http://localhost:"+config.ApiPort+"/"+notification.Id, b)
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

func readNotification(config configuration.Config, userId string, id string, expected model.Notification) error {
	token, err := createToken(userId)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("GET", "http://localhost:"+config.ApiPort+"/"+id, nil)
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
	actual := model.Notification{}
	err = json.NewDecoder(resp.Body).Decode(&actual)
	if err != nil {
		return err
	}

	if !actual.Equal(expected) {
		return errors.New("actual not expected")
	}
	return nil
}

func deleteNotification(config configuration.Config, userId string, id string) error {
	token, err := createToken(userId)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("DELETE", "http://localhost:"+config.ApiPort+"/"+id, nil)
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
