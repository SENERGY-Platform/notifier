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
	"errors"
	"fmt"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	"github.com/SENERGY-Platform/notifier/pkg/persistence/vault"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"time"
)

var brokerUserIdKey = "user_id"
var brokerIdKey = "id"
var brokerEnabledKey = "enabled"

func initBrokers() {
	var err error

	CreateCollections = append(CreateCollections, func(db *Mongo) error {
		collection := db.client.Database(db.config.MongoTable).Collection(db.config.MongoBrokerCollection)
		err = db.ensureIndex(collection, "brokeruseridindex", brokerUserIdKey, true, false)
		if err != nil {
			return err
		}
		err = db.ensureIndex(collection, "brokeridindex", brokerIdKey, true, true)
		if err != nil {
			return err
		}
		return nil
	})
}

func (this *Mongo) brokerCollection() *mongo.Collection {
	return this.client.Database(this.config.MongoTable).Collection(this.config.MongoBrokerCollection)
}

func (this *Mongo) ListBrokers(userId string, o persistence.ListOptions) (result []model.Broker, total int64, err error, errCode int) {
	result = []model.Broker{}
	opt := options.Find()
	opt.SetLimit(int64(o.Limit))
	opt.SetSkip(int64(o.Offset))

	filter := bson.M{}
	if userId != "" {
		filter[brokerUserIdKey] = userId
	}

	ctx, _ := getTimeoutContext()
	collection := this.brokerCollection()

	total, err = collection.CountDocuments(ctx, filter)
	if err != nil {
		return result, total, err, http.StatusInternalServerError
	}
	cursor, err := collection.Find(ctx, filter, opt)
	if err != nil {
		return result, total, err, http.StatusInternalServerError
	}
	for cursor.Next(ctx) {
		element := model.Broker{}
		err = cursor.Decode(&element)
		if err != nil {
			return result, total, err, http.StatusInternalServerError
		}
		result = append(result, element)
	}
	err = cursor.Err()
	if err != nil {
		return nil, total, err, http.StatusInternalServerError
	}
	result, err = this.brokerManager.FillList(result)
	if err != nil {
		return nil, total, err, http.StatusInternalServerError
	}
	return
}

func (this *Mongo) ReadBroker(userId string, id string) (result model.Broker, err error, errCode int) {
	ctx, _ := getTimeoutContext()
	var filter bson.M
	if userId != "" {
		filter = bson.M{
			"$and": []bson.M{
				{brokerIdKey: id},
				{brokerUserIdKey: userId},
			},
		}
	} else {
		filter = bson.M{brokerIdKey: id}
	}
	temp := this.brokerCollection().FindOne(
		ctx,
		filter)
	err = temp.Err()
	if err == mongo.ErrNoDocuments {
		return result, err, http.StatusNotFound
	}
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	err = temp.Decode(&result)
	if err == mongo.ErrNoDocuments {
		return result, err, http.StatusNotFound
	}
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	err = this.brokerManager.Fill(&result)
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	return result, nil, http.StatusOK
}

func (this *Mongo) SetBroker(broker model.Broker) (error, int) {
	err := this.brokerManager.SaveAndStrip(&broker)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	ctx, _ := getTimeoutContext()
	_, err = this.brokerCollection().ReplaceOne(
		ctx,
		bson.M{
			brokerIdKey: broker.Id,
		},
		broker,
		options.Replace().SetUpsert(true))
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, http.StatusOK
}

func (this *Mongo) RemoveBrokers(userId string, ids []string) (error, int) {
	ctx, _ := getTimeoutContext()
	collection := this.brokerCollection()

	filter := bson.M{
		"$and": []bson.M{
			{brokerUserIdKey: userId},
			{brokerIdKey: bson.M{
				"$in": ids,
			}},
		},
	}
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return err, http.StatusInternalServerError
	}

	if total != int64(len(ids)) {
		return errors.New("did not find all ids"), http.StatusNotFound
	}

	_, err = collection.DeleteMany(ctx, filter)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	err = this.brokerManager.Delete(ids)
	if err != nil {
		fmt.Println("ERROR: Could not delete at vault: " + err.Error())
		err = nil
	}
	return nil, http.StatusOK
}

func (this *Mongo) ListEnabledBrokers(userId string) (result []model.Broker, err error) {
	result = []model.Broker{}

	ctx, _ := getTimeoutContext()
	collection := this.brokerCollection()
	filter := bson.M{"$and": []bson.M{
		{brokerUserIdKey: userId},
		{brokerEnabledKey: true},
	}}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return result, err
	}
	for cursor.Next(ctx) {
		element := model.Broker{}
		err = cursor.Decode(&element)
		if err != nil {
			return result, err
		}
		result = append(result, element)
	}
	err = cursor.Err()
	if err != nil {
		return nil, err
	}
	result, err = this.brokerManager.FillList(result)
	if err != nil {
		return nil, err
	}
	return
}

func (this *Mongo) HandlerBrokerMongoVaultConsistency(cleanupVaultKeys bool) (err error) {
	start := time.Now()
	vaultList, err := this.brokerManager.ListKeys()
	if err != nil {
		return err
	}
	deleteKeys := []string{}
	for _, id := range vaultList {
		_, err, _ = this.ReadBroker("", id)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				log.Println("WARN: Vault has data for id " + id + ", but mongo entry missing")
				deleteKeys = append(deleteKeys, id)
			} else {
				return err
			}
		}
	}
	if cleanupVaultKeys {
		log.Println("WARN: Deleting vault keys that have no mongo entries")
		err = this.brokerManager.Delete(deleteKeys)
		if err != nil {
			return err
		}
	}
	log.Println("INFO: Consistency checks took " + time.Since(start).String())
	return nil
}

func (this *Mongo) MigrateSecretsToVault() (err error) {
	start := time.Now()
	var offset int64 = 0
	var batchSize int64 = 100
	ctx, _ := getTimeoutContext()
	collection := this.brokerCollection()
	total, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	opt := options.Find()
	for total > offset {
		ctx, _ := getTimeoutContext()
		opt.SetLimit(batchSize)
		opt.SetSkip(offset)
		cursor, err := collection.Find(ctx, bson.M{}, opt)
		for cursor.Next(ctx) {
			element := model.Broker{}
			err = cursor.Decode(&element)
			if err != nil {
				return err
			}
			if vault.NeedsMigration(&element) {
				log.Println("INFO: Migrating broker " + element.Id)
				err, _ = this.SetBroker(element)
				if err != nil {
					return err
				}
			}
			offset++
		}
	}
	log.Println("INFO: Migration took " + time.Since(start).String())
	return nil
}
