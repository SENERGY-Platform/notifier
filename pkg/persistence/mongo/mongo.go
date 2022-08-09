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

package mongo

import (
	"context"
	"errors"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence/vault"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type Mongo struct {
	config          configuration.Config
	client          *mongo.Client
	brokerManager   *vault.BrokerManager
	fcmTokenManager *vault.FcmTokenManager
}

var CreateCollections = []func(db *Mongo) error{}

func New(conf configuration.Config) (*Mongo, error) {
	//ctx, _ := getTimeoutContext()
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://"+conf.MongoAddr+":"+conf.MongoPort))
	if err != nil {
		return nil, err
	}
	manager, err := vault.New(conf)
	if err != nil {
		return nil, err
	}

	fcmTokenManager, err := vault.NewFcmTokenManager(conf)
	if err != nil {
		return nil, err
	}

	db := &Mongo{config: conf, client: client, brokerManager: manager, fcmTokenManager: fcmTokenManager}
	initNotifications()
	initBrokers()
	initPlatformBrokers()
	for _, creators := range CreateCollections {
		err = creators(db)
		if err != nil {
			client.Disconnect(context.Background())
			return nil, err
		}
	}
	if conf.VaultEnsureMigration {
		err = db.MigrateSecretsToVault()
		if err != nil {
			return nil, err
		}
	}
	err = db.HandlerBrokerMongoVaultConsistency(conf.VaultCleanupKeys)
	if err != nil {
		return nil, err
	}
	err = db.migrateHash()
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (this *Mongo) ensureIndex(collection *mongo.Collection, indexname string, indexKey string, asc bool, unique bool) error {
	ctx, _ := getTimeoutContext()
	var direction int32 = -1
	if asc {
		direction = 1
	}
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bsonx.Doc{{indexKey, bsonx.Int32(direction)}},
		Options: options.Index().SetName(indexname).SetUnique(unique),
	})
	return err
}

func (this *Mongo) ensureCompoundIndex(collection *mongo.Collection, indexname string, asc bool, unique bool, indexKeys ...string) error {
	ctx, _ := getTimeoutContext()
	var direction int32 = -1
	if asc {
		direction = 1
	}
	keys := []bsonx.Elem{}
	for _, key := range indexKeys {
		keys = append(keys, bsonx.Elem{Key: key, Value: bsonx.Int32(direction)})
	}
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bsonx.Doc(keys),
		Options: options.Index().SetName(indexname).SetUnique(unique),
	})
	return err
}

func (this *Mongo) ensureTextIndex(collection *mongo.Collection, indexname string, indexKeys ...string) error {
	if len(indexKeys) == 0 {
		return errors.New("expect at least one key")
	}
	keys := bsonx.Doc{}
	for _, key := range indexKeys {
		keys = append(keys, bsonx.Elem{Key: key, Value: bsonx.String("text")})
	}
	ctx, _ := getTimeoutContext()
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    keys,
		Options: options.Index().SetName(indexname),
	})
	return err
}

func (this *Mongo) Disconnect() {
	timeout, _ := context.WithTimeout(context.Background(), 10*time.Second)
	log.Println(this.client.Disconnect(timeout))
}

func (this *Mongo) UpsertFcmToken(token model.FcmToken) (err error, errCode int) {
	err = this.fcmTokenManager.Save(&token)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, http.StatusOK
}

func (this *Mongo) DeleteFcmToken(token model.FcmToken) (err error, errCode int) {
	return this.fcmTokenManager.Delete(&token)
}

func (this *Mongo) GetFcmTokens(userId string) (tokens []model.FcmToken, err error) {
	tokensWithId, err := this.fcmTokenManager.GetFcmTokens(userId)
	if err != nil {
		return nil, err
	}
	tokens = []model.FcmToken{}
	for _, token := range tokensWithId {
		tokens = append(tokens, token.FcmToken)
	}
	return
}

func getBsonFieldName(obj interface{}, fieldName string) (bsonName string, err error) {
	field, found := reflect.TypeOf(obj).FieldByName(fieldName)
	if !found {
		return "", errors.New("field '" + fieldName + "' not found")
	}
	tags, err := bsoncodec.DefaultStructTagParser.ParseStructTags(field)
	return tags.Name, err
}

func getBsonFieldPath(obj interface{}, path string) (bsonPath string, err error) {
	t := reflect.TypeOf(obj)
	pathParts := strings.Split(path, ".")
	bsonPathParts := []string{}
	for _, name := range pathParts {
		field, found := t.FieldByName(name)
		if !found {
			return "", errors.New("field path '" + path + "' not found at '" + name + "'")
		}
		tags, err := bsoncodec.DefaultStructTagParser.ParseStructTags(field)
		if err != nil {
			return bsonPath, err
		}
		bsonPathParts = append(bsonPathParts, tags.Name)
		t = field.Type
	}
	bsonPath = strings.Join(bsonPathParts, ".")
	return
}

func getTimeoutContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
