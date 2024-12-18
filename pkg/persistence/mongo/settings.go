/*
 * Copyright 2024 InfAI (CC SES)
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
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/http"
)

var settingsUserIdKey = "user_id"

func initSettings() {
	CreateCollections = append(CreateCollections, func(db *Mongo) error {
		collection := db.client.Database(db.config.MongoTable).Collection(db.config.MongoSettingsCollection)
		return db.ensureIndex(collection, "settingsuseridindex", settingsUserIdKey, true, true)
	})
}

func (this *Mongo) settingsCollection() *mongo.Collection {
	return this.client.Database(this.config.MongoTable).Collection(this.config.MongoSettingsCollection)
}

func (this *Mongo) ReadSettings(userId string) (result model.Settings, err error, errCode int) {
	ctx, _ := getTimeoutContext()
	temp := this.settingsCollection().FindOne(
		ctx,
		bson.M{
			settingsUserIdKey: userId,
		})
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
	return result, nil, http.StatusOK
}

func (this *Mongo) SetSettings(settings model.Settings) (error, int) {
	ctx, _ := getTimeoutContext()
	_, err := this.settingsCollection().ReplaceOne(
		ctx,
		bson.M{
			settingsUserIdKey: settings.UserId,
		},
		settings,
		options.Replace().SetUpsert(true))
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, http.StatusOK
}
