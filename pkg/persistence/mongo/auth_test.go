/*
 * Copyright 2026 InfAI (CC SES)
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

package mongo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	"github.com/SENERGY-Platform/notifier/pkg/tests/mock"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestStartAuthenticates needs a throwaway server with access control; MONGO_AUTH_TEST_USER and
// MONGO_AUTH_TEST_PASSWORD are root credentials, used to create and remove the test users.
func TestStartAuthenticates(t *testing.T) {
	url, rootUser, rootPassword := os.Getenv("MONGO_AUTH_TEST_URL"), os.Getenv("MONGO_AUTH_TEST_USER"), os.Getenv("MONGO_AUTH_TEST_PASSWORD")
	if testing.Short() || url == "" || rootUser == "" || rootPassword == "" {
		t.Skip("needs MONGO_AUTH_TEST_URL, MONGO_AUTH_TEST_USER and MONGO_AUTH_TEST_PASSWORD, not in -short")
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	root, err := mongo.Connect(ctx, options.Client().ApplyURI(url).SetAuth(options.Credential{Username: rootUser, Password: rootPassword, AuthSource: "admin"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Disconnect(context.Background()) })

	suffix := randomHex(t)
	testDB, otherDB := "notifier_auth_test_"+suffix, "notifier_auth_other_"+suffix
	svcUser, svcPassword := "notifier-test-"+suffix, randomHex(t)
	otherUser, otherPassword := "notifier-other-"+suffix, randomHex(t)
	readUser, readPassword := "notifier-read-"+suffix, randomHex(t)
	createUser(t, root, svcUser, svcPassword, "readWrite", testDB)
	createUser(t, root, otherUser, otherPassword, "readWrite", otherDB)
	createUser(t, root, readUser, readPassword, "read", testDB)
	passwords := []string{svcPassword, otherPassword, readPassword, rootPassword}

	base, err := configuration.Load("../../../config.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = mock.MockKeycloak(&base, ctx); err != nil {
		t.Fatal(err)
	}
	if err = mock.MockVault(&base, ctx); err != nil {
		t.Fatal(err)
	}
	config := func(user, password string) configuration.Config {
		c := base
		c.MongoUrl = url
		c.MongoUser = user
		c.MongoPassword = password
		c.MongoAuthSource = "admin"
		c.MongoDatabase = testDB
		return c
	}

	t.Run("New with correct credentials", func(t *testing.T) {
		db, err := New(config(svcUser, svcPassword))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Disconnect()
		n := model.Notification{Id: "000000000000000000000001", UserId: "user", Title: "t", Topic: model.TopicUnknown}
		if err, _ = db.SetNotification(n); err != nil {
			t.Errorf("write as the service user: %v", err)
		}
		if _, total, err, _ := db.ListNotifications("user", persistence.ListOptions{Limit: 10}, []model.Topic{model.TopicUnknown}); err != nil || total != 1 {
			t.Errorf("read as the service user: total = %d, err = %v", total, err)
		}
		assertIndexes(t, root, testDB, base.MongoNotificationCollection, "notificationtitleindex", "notificationcreatedatindex", "notificationuseridindex")
		assertIndexes(t, root, testDB, base.MongoBrokerCollection, "brokeruseridindex")
	})

	cases := []struct {
		name, user, password string
		wantErr              string
	}{
		{"correct credentials", svcUser, svcPassword, ""},
		{"no credentials", "", "", "mongo startup check failed: "},
		{"user of another database", otherUser, otherPassword, "mongo startup check failed: "},
		{"wrong password", svcUser, svcPassword + "-wrong", "mongo startup check failed: "},
		// listCollections passes, index creation does not.
		{"read-only user", readUser, readPassword, "Unauthorized"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conf := config(c.user, c.password)
			if err := validateConfig(conf); err != nil {
				t.Fatal(err)
			}
			pools := &poolCounter{}
			db, err := start(conf, clientOptions(conf).SetPoolMonitor(pools.monitor()), 10*time.Second)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				db.Disconnect()
				pools.assertAllClosed(t)
				return
			}
			if err == nil {
				db.Disconnect()
				t.Fatalf("expected an error containing %q", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("unexpected error: %v", err)
			}
			for _, pw := range passwords {
				if strings.Contains(err.Error(), pw) {
					t.Error("error text contains a password")
				}
			}
			pools.assertAllClosed(t)
		})
	}
}

// createUser registers the cleanup first, so a partly failed creation is removed as well.
func createUser(t *testing.T, root *mongo.Client, user, password, role, db string) {
	t.Helper()
	admin := root.Database("admin")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = admin.RunCommand(ctx, bson.D{{Key: "dropUser", Value: user}}).Err()
		_ = root.Database(db).Drop(ctx)
	})
	cmd := bson.D{
		{Key: "createUser", Value: user},
		{Key: "pwd", Value: password},
		{Key: "roles", Value: bson.A{bson.D{{Key: "role", Value: role}, {Key: "db", Value: db}}}},
	}
	if err := admin.RunCommand(context.Background(), cmd).Err(); err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func assertIndexes(t *testing.T, root *mongo.Client, db, collection string, want ...string) {
	t.Helper()
	specs, err := root.Database(db).Collection(collection).Indexes().ListSpecifications(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, s := range specs {
		names = append(names, s.Name)
	}
	for _, w := range want {
		if !slices.Contains(names, w) {
			t.Errorf("index %q missing in %s.%s, have %v", w, db, collection, names)
		}
	}
}

func randomHex(t *testing.T) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
