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

import "time"

type Broker struct {
	Id        string    `json:"id"`
	Address   string    `json:"address"`
	User      string    `json:"user"`
	Password  string    `json:"password"`
	Topic     string    `json:"topic"`
	Qos       uint8     `json:"qos"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
	UserId    string    `json:"-" bson:"user_id"`
}

func (b *Broker) Equal(other interface{}) bool {
	otherB, ok := other.(Broker)
	if !ok {
		return false
	}
	return b.Id == otherB.Id &&
		b.Address == otherB.Address &&
		b.User == otherB.User &&
		b.Password == otherB.Password &&
		b.Topic == otherB.Topic &&
		b.CreatedAt.Equal(otherB.CreatedAt) &&
		b.UpdatedAt.Equal(otherB.UpdatedAt) &&
		b.UserId == otherB.UserId
}

type BrokerList struct {
	Total   int64    `json:"total"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
	Brokers []Broker `json:"brokers"`
}

func (l *BrokerList) Equal(other interface{}) bool {
	otherL, ok := other.(BrokerList)
	if !ok {
		return false
	}
	if l.Offset != otherL.Offset || l.Limit != otherL.Limit || l.Total != otherL.Total {
		return false
	}
	for i := range l.Brokers {
		if !l.Brokers[i].Equal(otherL.Brokers[i]) {
			return false
		}
	}
	return true
}
