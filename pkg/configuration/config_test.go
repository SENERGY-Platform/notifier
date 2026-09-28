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

package configuration

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const repoConfig = "../../config.json"

type mongoFields struct {
	Url, User, Password, AuthSource, Database         string
	Notifications, Brokers, PlatformBrokers, Settings string
}

func mongoOf(c Config) mongoFields {
	return mongoFields{
		c.MongoUrl, c.MongoUser, c.MongoPassword, c.MongoAuthSource, c.MongoDatabase,
		c.MongoNotificationCollection, c.MongoBrokerCollection, c.MongoPlatformBrokerCollection, c.MongoSettingsCollection,
	}
}

// clearMongoEnv empties the variables for this test; the loader ignores empty values.
func clearMongoEnv(t *testing.T) {
	for _, k := range []string{
		"MONGO_URL", "MONGO_USER", "MONGO_PASSWORD", "MONGO_AUTH_SOURCE", "MONGO_DATABASE",
		"MONGO_NOTIFICATION_COLLECTION", "MONGO_BROKER_COLLECTION", "MONGO_PLATFORM_BROKER_COLLECTION", "MONGO_SETTINGS_COLLECTION",
		"MONGO_ADDR", "MONGO_PORT", "MONGO_TABLE",
	} {
		t.Setenv(k, "")
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_RepoConfigMongoDefaults(t *testing.T) {
	clearMongoEnv(t)
	cfg, err := Load(repoConfig)
	if err != nil {
		t.Fatal(err)
	}
	want := mongoFields{
		Url: "mongodb://localhost:27017", AuthSource: "admin", Database: "notifier",
		Notifications: "notifications", Brokers: "brokers", PlatformBrokers: "platformbroker", Settings: "settings",
	}
	if got := mongoOf(cfg); got != want {
		t.Errorf("mongo = %+v, want %+v", got, want)
	}
}

func TestLoad_MongoDefaultsWhenFileOmitsThem(t *testing.T) {
	clearMongoEnv(t)
	cfg, err := Load(writeConfig(t, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	want := mongoFields{Url: "mongodb://localhost:27017", AuthSource: "admin", Database: "notifier"}
	if got := mongoOf(cfg); got != want {
		t.Errorf("mongo = %+v, want %+v", got, want)
	}
}

func TestLoad_MongoEnvNames(t *testing.T) {
	clearMongoEnv(t)
	t.Setenv("MONGO_URL", "mongodb://mongo-0.mongo:27017,mongo-1.mongo:27017/?replicaSet=rs0")
	t.Setenv("MONGO_USER", "notifier")
	t.Setenv("MONGO_PASSWORD", "p@ss:w/rd")
	t.Setenv("MONGO_AUTH_SOURCE", "users")
	t.Setenv("MONGO_DATABASE", "notifier_test")
	var cfg Config
	captureStdout(t, func() {
		var err error
		if cfg, err = Load(repoConfig); err != nil {
			t.Error(err)
		}
	})
	want := mongoFields{
		Url:             "mongodb://mongo-0.mongo:27017,mongo-1.mongo:27017/?replicaSet=rs0",
		User:            "notifier",
		Password:        "p@ss:w/rd",
		AuthSource:      "users",
		Database:        "notifier_test",
		Notifications:   "notifications",
		Brokers:         "brokers",
		PlatformBrokers: "platformbroker",
		Settings:        "settings",
	}
	if got := mongoOf(cfg); got != want {
		t.Errorf("mongo = %+v, want %+v", got, want)
	}
}

func TestLoad_RemovedMongoSettingsIgnored(t *testing.T) {
	clearMongoEnv(t)
	t.Setenv("MONGO_TABLE", "other")
	t.Setenv("MONGO_ADDR", "otherhost")
	t.Setenv("MONGO_PORT", "1234")
	cfg, err := Load(writeConfig(t, `{"mongo_table": "other", "mongo_addr": "otherhost", "mongo_port": "1234"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MongoDatabase != "notifier" || cfg.MongoUrl != "mongodb://localhost:27017" {
		t.Errorf("database = %q, url = %q, want notifier and mongodb://localhost:27017", cfg.MongoDatabase, cfg.MongoUrl)
	}
}

func TestLoad_MongoConfigFile(t *testing.T) {
	clearMongoEnv(t)
	cfg, err := Load(writeConfig(t, `{"mongo_url": "mongodb://file:27017", "mongo_user": "u", "mongo_password": "s3cr3t", "mongo_auth_source": "a", "mongo_database": "d"}`))
	if err != nil {
		t.Fatal(err)
	}
	want := mongoFields{Url: "mongodb://file:27017", User: "u", Password: "s3cr3t", AuthSource: "a", Database: "d"}
	if got := mongoOf(cfg); got != want {
		t.Errorf("mongo = %+v, want %+v", got, want)
	}
}

// The loader prints every environment variable it applies.
func TestLoad_EnvPrintMasksMongoPassword(t *testing.T) {
	clearMongoEnv(t)
	t.Setenv("MONGO_USER", "notifier")
	t.Setenv("MONGO_PASSWORD", "s3cr3t-pw")
	out := captureStdout(t, func() {
		if _, err := Load(repoConfig); err != nil {
			t.Error(err)
		}
	})
	if strings.Contains(out, "s3cr3t-pw") {
		t.Errorf("printed environment leaks the password: %s", out)
	}
	if !strings.Contains(out, "MONGO_USER  =  notifier") {
		t.Errorf("expected the applied variables to be printed, got: %s", out)
	}
}

func TestConfigFormattingMasksMongoPassword(t *testing.T) {
	clearMongoEnv(t)
	t.Setenv("MONGO_PASSWORD", "s3cr3t-pw")
	var cfg Config
	captureStdout(t, func() {
		var err error
		if cfg, err = Load(repoConfig); err != nil {
			t.Error(err)
		}
	})
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	bp, err := json.Marshal(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	outputs := map[string]string{
		"json":         string(b),
		"json pointer": string(bp),
		"%v":           fmt.Sprintf("%v", cfg),
		"%+v":          fmt.Sprintf("%+v", cfg),
		"%#v":          fmt.Sprintf("%#v", cfg),
		"%s":           fmt.Sprintf("%s", cfg),
		"%+v pointer":  fmt.Sprintf("%+v", &cfg),
		"%#v pointer":  fmt.Sprintf("%#v", &cfg),
		"Sprint":       fmt.Sprint(cfg),
	}
	for name, s := range outputs {
		if strings.Contains(s, "s3cr3t-pw") {
			t.Errorf("%s leaks the password: %s", name, s)
		}
		if !strings.Contains(s, "platformbroker") {
			t.Errorf("%s lost the other fields: %s", name, s)
		}
	}
	if !strings.Contains(string(b), `"mongo_password":"***"`) {
		t.Errorf("json does not show the password as masked: %s", b)
	}
	if cfg.MongoPassword != "s3cr3t-pw" {
		t.Errorf("masking changed the loaded password to %q", cfg.MongoPassword)
	}
}

func TestConfigFormattingMasksKeycloakSecretAndMqttPw(t *testing.T) {
	t.Setenv("KEYCLOAK_CLIENT_SECRET", "s3cr3t-client")
	t.Setenv("PLATFORM_MQTT_PW", "s3cr3t-mqtt")
	var cfg Config
	captureStdout(t, func() {
		var err error
		if cfg, err = Load(repoConfig); err != nil {
			t.Error(err)
		}
	})
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	outputs := map[string]string{
		"json": string(b),
		"%v":   fmt.Sprintf("%v", cfg),
		"%+v":  fmt.Sprintf("%+v", cfg),
		"%#v":  fmt.Sprintf("%#v", cfg),
	}
	for name, s := range outputs {
		if strings.Contains(s, "s3cr3t-client") {
			t.Errorf("%s leaks the keycloak client secret: %s", name, s)
		}
		if strings.Contains(s, "s3cr3t-mqtt") {
			t.Errorf("%s leaks the platform mqtt password: %s", name, s)
		}
	}
	if !strings.Contains(string(b), `"keycloak_client_secret":"***"`) {
		t.Errorf("json does not show the keycloak client secret as masked: %s", b)
	}
	if !strings.Contains(string(b), `"platform_mqtt_pw":"***"`) {
		t.Errorf("json does not show the platform mqtt password as masked: %s", b)
	}
	if cfg.KeycloakClientSecret != "s3cr3t-client" || cfg.PlatformMqttPw != "s3cr3t-mqtt" {
		t.Errorf("masking changed the loaded values: keycloak=%q mqtt=%q", cfg.KeycloakClientSecret, cfg.PlatformMqttPw)
	}
}

func TestConfigFormattingKeepsEmptyPasswordEmpty(t *testing.T) {
	b, err := json.Marshal(Config{MongoUser: "notifier"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"mongo_password":""`) {
		t.Errorf("an unset password should stay visibly unset: %s", b)
	}
}

func TestFieldNameToEnvName_MongoFields(t *testing.T) {
	for field, want := range map[string]string{
		"MongoUrl":        "MONGO_URL",
		"MongoUser":       "MONGO_USER",
		"MongoPassword":   "MONGO_PASSWORD",
		"MongoAuthSource": "MONGO_AUTH_SOURCE",
		"MongoDatabase":   "MONGO_DATABASE",
	} {
		if got := fieldNameToEnvName(field); got != want {
			t.Errorf("%s -> %s, want %s", field, got, want)
		}
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	defer func() { os.Stdout = orig }()
	f()
	_ = w.Close()
	return <-done
}
