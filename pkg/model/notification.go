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

package model

import (
	"crypto/sha256"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Topic = string

const TopicProcesses = "processes"
const TopicSmartService = "smart_service"
const TopicDeviceOffline = "device_offline"
const TopicDeveloper = "developer"
const TopicConnector = "connector"
const TopicMGW = "mgw"
const TopicIncident = "incident"
const TopicAnalytics = "analytics"
const TopicUnknown = "unknown"

func AllTopics() []Topic {
	return []Topic{
		TopicProcesses,
		TopicSmartService,
		TopicDeviceOffline,
		TopicDeveloper,
		TopicConnector,
		TopicMGW,
		TopicIncident,
		TopicAnalytics,
		//TopicUnknown, excluded on purpose. TopicUnknown should never be set but serve for notifications without a topic set
	}
}

type Notification struct {
	Id        string    `json:"_id" bson:"_id"`
	UserId    string    `json:"userId" bson:"userId"`
	Title     string    `json:"title" bson:"title"`
	Message   string    `json:"message" bson:"message"`
	IsRead    bool      `json:"isRead" bson:"isRead"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	Topic     `json:"topic" bson:"topic"`
	hash      [32]byte
}

func (n *Notification) ToDB() (db NotificationDB, err error) {
	db.Id, err = primitive.ObjectIDFromHex(n.Id)
	if err != nil {
		return db, err
	}
	db.UserId, db.Title, db.Message, db.IsRead, db.CreatedAt, db.Topic, db.Hash = n.UserId, n.Title, n.Message, n.IsRead, n.CreatedAt, n.Topic, n.Hash()
	return
}

func (n *Notification) Equal(other interface{}) bool {
	otherN, ok := other.(Notification)
	if !ok {
		return false
	}
	return n.Id == otherN.Id &&
		n.IsRead == otherN.IsRead &&
		n.CreatedAt.Equal(otherN.CreatedAt) &&
		n.Title == otherN.Title &&
		n.Message == otherN.Message &&
		n.UserId == otherN.UserId &&
		n.Topic == otherN.Topic
}

func (n *Notification) Hash() [32]byte {
	for _, v := range n.hash {
		if v != 0 {
			return n.hash
		}
	}
	n.hash = sha256.Sum256([]byte(n.Title + "___" + n.Message + "____" + n.Topic))
	return n.hash
}

type NotificationDB struct {
	Id        primitive.ObjectID `bson:"_id"`
	UserId    string             `bson:"userId"`
	Title     string             `bson:"title"`
	Message   string             `bson:"message"`
	IsRead    bool               `bson:"isRead"`
	CreatedAt time.Time          `bson:"created_at"`
	Topic     `bson:"topic"`
	Hash      [32]byte `bson:"hash"`
}

type NotificationList struct {
	Total         int64          `json:"total"`
	Limit         int            `json:"limit"`
	Offset        int            `json:"offset"`
	Notifications []Notification `json:"notifications"`
}

func (n *NotificationList) Equal(other interface{}) bool {
	otherL, ok := other.(NotificationList)
	if !ok {
		return false
	}
	if n.Offset != otherL.Offset || n.Limit != otherL.Limit || n.Total != otherL.Total {
		return false
	}
	for i := range n.Notifications {
		if !n.Notifications[i].Equal(otherL.Notifications[i]) {
			return false
		}
	}
	return true
}

type EventMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

const WsAuthType = "authentication"
const WsAuthRequestType = "please reauthenticate"
const WsAuthOkType = "authentication confirmed"
const WsUpdateSetType = "put notification"
const WsRefreshType = "refresh"
const WsListType = "notification list"
const WsUpdateDeleteType = "delete notification"
const WsUpdateDeleteManyType = "delete notifications"
