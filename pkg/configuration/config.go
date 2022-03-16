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

package configuration

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	ApiPort                       string `json:"api_port"`
	MongoAddr                     string `json:"mongo_addr"`
	MongoPort                     string `json:"mongo_port"`
	MongoTable                    string `json:"mongo_table"`
	MongoNotificationCollection   string `json:"mongo_notification_collection"`
	MongoBrokerCollection         string `json:"mongo_broker_collection"`
	MongoPlatformBrokerCollection string `json:"mongo_platformbroker_collection"`
	Debug                         bool   `json:"debug"`
	JwtSigningKey                 string `json:"jwt_signing_key"` //without -----BEGIN PUBLIC KEY-----
	WsPingPeriod                  string `json:"ws_ping_period"`
	PlatformMqttAddress           string `json:"platform_mqtt_address"`
	PlatformMqttUser              string `json:"platform_mqtt_user"`
	PlatformMqttPw                string `json:"platform_mqtt_pw"`
	PlatformMqttQos               uint8  `json:"platform_mqtt_qos"`
	PlatformMqttBasetopic         string `json:"platform_mqtt_basetopic"`
	MqttClientPrefix              string `json:"mqtt_client_prefix"`

	KeycloakUrl          string `json:"keycloak_url"`
	KeycloakRealm        string `json:"keycloak_realm"`
	KeycloakClientId     string `json:"keycloak_client_id"`
	KeycloakClientSecret string `json:"keycloak_client_secret"`
	VaultUrl             string `json:"vault_url"`
	VaultRole            string `json:"vault_role"`
	VaultEngineBroker    string `json:"vault_engine_broker"`
	VaultEngineFcm       string `json:"vault_engine_fcm"`
	VaultCleanupKeys     bool   `json:"vault_cleanup_keys"`
	VaultEnsureMigration bool   `json:"vault_ensure_migration"`

	FcmProjectId string `json:"fcm_project_id"`
	FcmIamId     string `json:"fcm_iam_id"`
}

//loads config from json in location and used environment variables (e.g ZookeeperUrl --> ZOOKEEPER_URL)
func Load(location string) (config Config, err error) {
	file, err := os.Open(location)
	if err != nil {
		log.Println("error on config load: ", err)
		return config, err
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		log.Println("invalid config json: ", err)
		return config, err
	}
	handleEnvironmentVars(&config)
	return config, nil
}

var camel = regexp.MustCompile("(^[^A-Z]*|[A-Z]*)([A-Z][^A-Z]+|$)")

func fieldNameToEnvName(s string) string {
	var a []string
	for _, sub := range camel.FindAllStringSubmatch(s, -1) {
		if sub[1] != "" {
			a = append(a, sub[1])
		}
		if sub[2] != "" {
			a = append(a, sub[2])
		}
	}
	return strings.ToUpper(strings.Join(a, "_"))
}

// preparations for docker
func handleEnvironmentVars(config *Config) {
	configValue := reflect.Indirect(reflect.ValueOf(config))
	configType := configValue.Type()
	for index := 0; index < configType.NumField(); index++ {
		fieldName := configType.Field(index).Name
		envName := fieldNameToEnvName(fieldName)
		envValue := os.Getenv(envName)
		if envValue != "" {
			fmt.Println("use environment variable: ", envName, " = ", envValue)
			if configValue.FieldByName(fieldName).Kind() == reflect.Int64 {
				i, _ := strconv.ParseInt(envValue, 10, 64)
				configValue.FieldByName(fieldName).SetInt(i)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Uint8 {
				i, _ := strconv.ParseUint(envValue, 10, 8)
				configValue.FieldByName(fieldName).SetUint(i)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.String {
				configValue.FieldByName(fieldName).SetString(envValue)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Bool {
				b, _ := strconv.ParseBool(envValue)
				configValue.FieldByName(fieldName).SetBool(b)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Float64 {
				f, _ := strconv.ParseFloat(envValue, 64)
				configValue.FieldByName(fieldName).SetFloat(f)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Slice {
				val := []string{}
				for _, element := range strings.Split(envValue, ",") {
					val = append(val, strings.TrimSpace(element))
				}
				configValue.FieldByName(fieldName).Set(reflect.ValueOf(val))
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Map {
				value := map[string]string{}
				for _, element := range strings.Split(envValue, ",") {
					keyVal := strings.Split(element, ":")
					key := strings.TrimSpace(keyVal[0])
					val := strings.TrimSpace(keyVal[1])
					value[key] = val
				}
				configValue.FieldByName(fieldName).Set(reflect.ValueOf(value))
			}
		}
	}
}
