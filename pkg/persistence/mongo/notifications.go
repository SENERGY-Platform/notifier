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
	"log"
	"net/http"
	"time"

	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const notificationTitleFieldName = "Title"
const deviceCreatedAtFieldName = "CreatedAt"

var notificationTitleKey string
var notificationUserIdKey = "userId"
var notificationCreatedAtKey string
var notificationIdKey = "_id"
var notificationHashKey = "hash"
var topicKey = "topic"

func initNotifications() {
	var err error
	notificationTitleKey, err = getBsonFieldPath(model.Notification{}, notificationTitleFieldName)
	if err != nil {
		log.Fatal(err)
	}

	notificationCreatedAtKey, err = getBsonFieldName(model.Notification{}, deviceCreatedAtFieldName)
	if err != nil {
		log.Fatal(err)
	}

	CreateCollections = append(CreateCollections, func(db *Mongo) error {
		collection := db.client.Database(db.config.MongoTable).Collection(db.config.MongoNotificationCollection)
		err = db.ensureIndex(collection, "notificationtitleindex", notificationTitleKey, true, false)
		if err != nil {
			return err
		}
		err = db.ensureIndex(collection, "notificationcreatedatindex", notificationCreatedAtKey, true, false)
		if err != nil {
			return err
		}
		err = db.ensureIndex(collection, "notificationuseridindex", notificationUserIdKey, true, false)
		if err != nil {
			return err
		}
		return nil
	})
}

func (this *Mongo) notificationCollection() *mongo.Collection {
	return this.client.Database(this.config.MongoTable).Collection(this.config.MongoNotificationCollection)
}

func (this *Mongo) ListNotifications(userId string, o persistence.ListOptions, topics []model.Topic) (result []model.Notification, total int64, err error, errCode int) {
	result = []model.Notification{}
	opt := options.Find()
	opt.SetLimit(int64(o.Limit))
	opt.SetSkip(int64(o.Offset))

	filter := bson.M{notificationUserIdKey: userId, topicKey: bson.M{"$in": topics}}

	ctx, _ := getTimeoutContext()
	collection := this.notificationCollection()

	total, err = collection.CountDocuments(ctx, filter)
	if err != nil {
		return result, total, err, http.StatusInternalServerError
	}
	cursor, err := collection.Find(ctx, filter, opt)
	if err != nil {
		return result, total, err, http.StatusInternalServerError
	}
	for cursor.Next(ctx) {
		element := model.Notification{}
		err = cursor.Decode(&element)
		if err != nil {
			return result, total, err, http.StatusInternalServerError
		}
		result = append(result, element)
	}
	err = cursor.Err()
	return
}

func (this *Mongo) ReadNotification(userId string, id string) (result model.Notification, err error, errCode int) {
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return result, err, http.StatusBadRequest // id requested by user
	}
	ctx, _ := getTimeoutContext()
	temp := this.notificationCollection().FindOne(
		ctx,
		bson.M{
			"$and": []bson.M{
				{notificationIdKey: objectId},
				{notificationUserIdKey: userId},
			},
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

func (this *Mongo) ReadNotificationByHash(userId string, hash [32]byte, notOlderThan time.Time) (result model.Notification, err error, errCode int) {
	ctx, _ := getTimeoutContext()
	temp := this.notificationCollection().FindOne(
		ctx,
		bson.M{
			"$and": []bson.M{
				{notificationHashKey: hash},
				{notificationUserIdKey: userId},
				{notificationCreatedAtKey: bson.M{"$gte": notOlderThan}},
			},
		}, &options.FindOneOptions{Sort: bson.D{{"created_at", -1}}})
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

func (this *Mongo) SetNotification(notification model.Notification) (error, int) {
	notificationDb, err := notification.ToDB()
	if err != nil {
		return err, http.StatusInternalServerError // Id set by app
	}
	ctx, _ := getTimeoutContext()
	_, err = this.notificationCollection().ReplaceOne(
		ctx,
		bson.M{
			notificationIdKey: notificationDb.Id,
		},
		notificationDb,
		options.Replace().SetUpsert(true))
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, http.StatusOK
}

func (this *Mongo) RemoveNotifications(userId string, ids []string) (error, int) {
	objectIds := make([]primitive.ObjectID, len(ids))

	for i := range ids {
		objectId, err := primitive.ObjectIDFromHex(ids[i])
		if err != nil {
			return err, http.StatusInternalServerError // Id set by app
		}
		objectIds[i] = objectId
	}

	ctx, _ := getTimeoutContext()
	collection := this.notificationCollection()

	filter := bson.M{
		"$and": []bson.M{
			{notificationUserIdKey: userId},
			{notificationIdKey: bson.M{
				"$in": objectIds,
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
	return nil, http.StatusOK
}

func (this *Mongo) migrateHash() error {
	collection := this.notificationCollection()
	ctx, _ := getTimeoutContext()
	cursor, err := collection.Find(ctx, bson.M{
		notificationHashKey: nil,
	})
	if err != nil {
		return err
	}
	for cursor.Next(ctx) {
		element := model.Notification{}
		err = cursor.Decode(&element)
		if err != nil {
			return err
		}
		element.Hash()
		err, _ = this.SetNotification(element)
		if err != nil {
			return err
		}
	}
	return err
}
