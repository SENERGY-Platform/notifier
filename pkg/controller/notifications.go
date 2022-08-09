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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"time"
)

func (this *Controller) ListNotifications(token auth.Token, options persistence.ListOptions) (result model.NotificationList, err error, errCode int) {
	result.Limit, result.Offset = options.Limit, options.Offset
	result.Notifications, result.Total, err, errCode = this.db.ListNotifications(token.GetUserId(), options)
	return
}

func (this *Controller) ReadNotification(token auth.Token, id string) (result model.Notification, err error, errCode int) {
	result, err, errCode = this.db.ReadNotification(token.GetUserId(), id)
	if err != nil {
		return model.Notification{}, err, errCode
	}
	return result, nil, http.StatusOK
}

func (this *Controller) SetNotification(token auth.Token, notification model.Notification) (result model.Notification, err error, errCode int) {
	_, err, errCode = this.db.ReadNotification(token.GetUserId(), notification.Id) // Check existence before set
	if err != nil {
		return model.Notification{}, err, errCode
	}
	err, errCode = this.db.SetNotification(notification)
	if err == nil {
		go this.handleWsNotificationUpdate(token.GetUserId(), notification)
		go this.handleMqttNotificationUpdate(token.GetUserId(), notification)
		go this.handleFCMNotificationUpdate(token.GetUserId(), notification)
	}
	return notification, err, errCode
}

func (this *Controller) CreateNotification(token auth.Token, notification model.Notification, ignoreDuplicatesWithinSeconds *int64) (result model.Notification, err error, errCode int) {
	if notification.Id != "" {
		return result, errors.New("specifing id is not allowed"), http.StatusBadRequest
	}
	if ignoreDuplicatesWithinSeconds != nil {
		notOlderThan := time.Unix(time.Now().Unix()-*ignoreDuplicatesWithinSeconds, 0)
		existing, err, code := this.db.ReadNotificationByHash(token.GetUserId(), notification.Hash(), notOlderThan)
		if err == nil {
			return existing, err, code
		}
		if code != http.StatusNotFound {
			return existing, err, code
		}
	}

	notification.Id = primitive.NewObjectID().Hex()
	if notification.CreatedAt.Before(time.UnixMilli(0)) {
		notification.CreatedAt = time.Now().Truncate(time.Millisecond)
	}
	err, errCode = this.db.SetNotification(notification)
	if err == nil {
		go this.handleWsNotificationUpdate(token.GetUserId(), notification)
		go this.handleMqttNotificationUpdate(token.GetUserId(), notification)
		go this.handleFCMNotificationUpdate(token.GetUserId(), notification)
	}
	return notification, err, errCode
}

func (this *Controller) DeleteMultipleNotifications(token auth.Token, ids []string) (err error, errCode int) {
	err, errCode = this.db.RemoveNotifications(token.GetUserId(), ids)
	if err == nil {
		go this.handleWsNotificationDelete(token.GetUserId(), ids)
		go this.handleFCMNotificationDelete(token.GetUserId(), ids)
	}
	return
}
