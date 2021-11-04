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

package controller

import (
	"errors"
	"github.com/SENERGY-Platform/notifier/pkg/auth"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	uuid "github.com/satori/go.uuid"
	"net/http"
	"time"
)

func (this *Controller) ListBrokers(token auth.Token, options persistence.ListOptions) (result model.BrokerList, err error, errCode int) {
	result.Limit, result.Offset = options.Limit, options.Offset
	result.Brokers, result.Total, err, errCode = this.db.ListBrokers(token.GetUserId(), options)
	return
}

func (this *Controller) ReadBroker(token auth.Token, id string) (result model.Broker, err error, errCode int) {
	result, err, errCode = this.db.ReadBroker(token.GetUserId(), id)
	if err != nil {
		return model.Broker{}, err, errCode
	}
	return result, nil, http.StatusOK
}

func (this *Controller) CreateBroker(token auth.Token, broker model.Broker) (result model.Broker, err error, errCode int) {
	if broker.Id != "" {
		return result, errors.New("specifing id is not allowed"), http.StatusBadRequest
	}
	if broker.Address == "" {
		return result, errors.New("empty address not allowed"), http.StatusBadRequest
	}
	broker.Id = uuid.NewV4().String()
	broker.UserId = token.GetUserId()
	broker.CreatedAt = time.Now().Truncate(time.Millisecond)
	broker.UpdatedAt = broker.CreatedAt
	err, errCode = this.db.SetBroker(broker)
	return broker, err, errCode
}

func (this *Controller) SetBroker(token auth.Token, broker model.Broker) (result model.Broker, err error, errCode int) {
	if broker.Address == "" {
		return result, errors.New("empty address not allowed"), http.StatusBadRequest
	}
	_, err, errCode = this.db.ReadBroker(token.GetUserId(), broker.Id) // Check existence before set
	if err != nil {
		return model.Broker{}, err, errCode
	}
	broker.UpdatedAt = time.Now().Truncate(time.Millisecond)
	err, errCode = this.db.SetBroker(broker)
	return broker, err, errCode
}

func (this *Controller) DeleteMultipleBrokers(token auth.Token, ids []string) (err error, errCode int) {
	return this.db.RemoveBrokers(token.GetUserId(), ids)
}
