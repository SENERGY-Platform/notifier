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
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/log"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence/vault"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Mongo struct {
	config          configuration.Config
	client          *mongo.Client
	brokerManager   *vault.BrokerManager
	fcmTokenManager *vault.FcmTokenManager
}

var CreateCollections = []func(db *Mongo) error{}

const startupTimeout = 10 * time.Second

var (
	errEmptyDatabase   = errors.New("mongo database name must not be empty")
	errMissingPassword = errors.New("mongo password must not be empty when a mongo user is set")
)

func New(conf configuration.Config) (*Mongo, error) {
	if err := validateConfig(conf); err != nil {
		return nil, err
	}
	return start(conf, clientOptions(conf), startupTimeout)
}

// start disconnects the client on every failure path, so a failed startup leaves nothing connected.
func start(conf configuration.Config, opts *options.ClientOptions, timeout time.Duration) (*Mongo, error) {
	client, err := connect(context.Background(), opts, conf.MongoDatabase, timeout)
	if err != nil {
		return nil, err
	}
	db := &Mongo{config: conf, client: client}
	if err = db.init(); err != nil {
		db.Disconnect()
		return nil, err
	}
	return db, nil
}

func (this *Mongo) init() (err error) {
	this.brokerManager, err = vault.New(this.config)
	if err != nil {
		return err
	}
	this.fcmTokenManager, err = vault.NewFcmTokenManager(this.config)
	if err != nil {
		return err
	}
	initNotifications()
	initBrokers()
	initPlatformBrokers()
	initSettings()
	for _, creators := range CreateCollections {
		err = creators(this)
		if err != nil {
			return err
		}
	}
	if this.config.VaultEnsureMigration {
		err = this.MigrateSecretsToVault()
		if err != nil {
			return err
		}
	}
	err = this.HandlerBrokerMongoVaultConsistency(this.config.VaultCleanupKeys)
	if err != nil {
		return err
	}
	return this.migrateHash()
}

// connect runs listCollections on the service's database because Connect is lazy and ping needs no
// authentication; unreachable servers and wrong or missing credentials then fail at startup.
func connect(ctx context.Context, opts *options.ClientOptions, database string, timeout time.Duration) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}
	listOpts := options.ListCollections().SetNameOnly(true).SetAuthorizedCollections(true)
	if _, err = client.Database(database).ListCollectionNames(ctx, bson.D{}, listOpts); err != nil {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), timeout)
		defer disconnectCancel()
		_ = client.Disconnect(disconnectCtx)
		return nil, fmt.Errorf("mongo startup check failed: %w", err)
	}
	return client, nil
}

func validateConfig(conf configuration.Config) error {
	if conf.MongoDatabase == "" {
		return errEmptyDatabase
	}
	if conf.MongoUser != "" && conf.MongoPassword == "" {
		return errMissingPassword
	}
	return nil
}

// clientOptions applies the credentials after the URI so they replace any given in MONGO_URL.
func clientOptions(conf configuration.Config) *options.ClientOptions {
	opts := options.Client().ApplyURI(conf.MongoUrl)
	if conf.MongoUser != "" {
		opts.SetAuth(options.Credential{
			Username:   conf.MongoUser,
			Password:   conf.MongoPassword,
			AuthSource: conf.MongoAuthSource,
		})
	}
	return opts
}

func (this *Mongo) ensureIndex(collection *mongo.Collection, indexname string, indexKey string, asc bool, unique bool) error {
	ctx, _ := getTimeoutContext()
	var direction int32 = -1
	if asc {
		direction = 1
	}
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{indexKey, direction}},
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
	keys := []bson.E{}
	for _, key := range indexKeys {
		keys = append(keys, bson.E{Key: key, Value: direction})
	}
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D(keys),
		Options: options.Index().SetName(indexname).SetUnique(unique),
	})
	return err
}

func (this *Mongo) ensureTextIndex(collection *mongo.Collection, indexname string, indexKeys ...string) error {
	if len(indexKeys) == 0 {
		return errors.New("expect at least one key")
	}
	keys := bson.D{}
	for _, key := range indexKeys {
		keys = append(keys, bson.E{Key: key, Value: "text"})
	}
	ctx, _ := getTimeoutContext()
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    keys,
		Options: options.Index().SetName(indexname),
	})
	return err
}

func (this *Mongo) Disconnect() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := this.client.Disconnect(ctx); err != nil {
		log.Logger.Debug("mongo disconnect", "error", err.Error())
	}
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
