package controller

import (
	"context"
	"errors"
	"github.com/SENERGY-Platform/notifier/pkg/auth"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
	"sync"
	"time"
)

type Controller struct {
	config      configuration.Config
	db          Persistence
	sessionsMux sync.Mutex
	sessions    map[string][]WsSession
}

func New(config configuration.Config, db Persistence) *Controller {
	return &Controller{
		config:      config,
		db:          db,
		sessionsMux: sync.Mutex{},
		sessions:    make(map[string][]WsSession),
	}
}

type Persistence interface {
	ListNotifications(userId string, options persistence.ListOptions) (result []model.Notification, total int64, err error, errCode int)
	ReadNotification(userId string, id string) (result model.Notification, err error, errCode int)
	SetNotification(notification model.Notification) (err error, errCode int)
	RemoveNotifications(userId string, ids []string) (err error, errCode int)
}

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
		this.handleWsNotificationUpdate(token.GetUserId(), notification)
	}
	return notification, err, errCode
}

func (this *Controller) CreateNotification(token auth.Token, notification model.Notification) (result model.Notification, err error, errCode int) {
	if notification.Id != "" {
		return result, errors.New("specifing id is not allowed"), http.StatusBadRequest
	}
	notification.Id = primitive.NewObjectID().Hex()
	if notification.CreatedAt.Before(time.UnixMilli(0)) {
		notification.CreatedAt = time.Now().Truncate(time.Millisecond)
	}
	err, errCode = this.db.SetNotification(notification)
	if err == nil {
		this.handleWsNotificationUpdate(token.GetUserId(), notification)
	}
	return notification, err, errCode
}

func (this *Controller) DeleteMultipleNotifications(token auth.Token, ids []string) (err error, errCode int) {
	return this.db.RemoveNotifications(token.GetUserId(), ids)
}

func (this *Controller) startPing(ctx context.Context, conn *websocket.Conn) (err error) {
	pingPeriod, err := time.ParseDuration(this.config.WsPingPeriod)
	if err != nil {
		return err
	}
	go func() {
		ticker := time.NewTicker(pingPeriod)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := conn.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					log.Println("ERROR: sending ws ping:", err)
					return
				}
			}
		}
	}()
	return nil
}
