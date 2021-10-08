package tests

import (
	"context"
	"github.com/SENERGY-Platform/notifier/pkg"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"strconv"
	"sync"
	"testing"
)

func TestCRUD(t *testing.T) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := configuration.Load("./../../config.json")
	if err != nil {
		t.Fatal("ERROR: unable to load config", err)
	}

	mongoPort, _, err := MongoContainer(ctx, wg)
	if err != nil {
		t.Error(err)
		return
	}
	config.MongoAddr = "localhost"
	config.MongoPort = mongoPort

	freePort, err := getFreePort()
	if err != nil {
		t.Error(err)
		return
	}
	config.ApiPort = strconv.Itoa(freePort)

	err = pkg.Start(ctx, wg, config)
	if err != nil {
		t.Error(err)
		return
	}

	t.Run("empty list", listNotifications(config, "user1", model.NotificationList{
		Total:         0,
		Limit:         10,
		Offset:        0,
		Notifications: []model.Notification{},
	}))

	test1, err := createNotification(config, "user1", model.Notification{
		UserId:  "user1",
		Title:   "test1",
		Message: "test1",
		IsRead:  false,
	})
	if err != nil {
		t.Error(err)
	}

	err = readNotification(config, "user1", test1.Id, test1)
	if err != nil {
		t.Error(err)
	}

	test2, err := createNotificationLegacy(config, "user1", model.Notification{
		UserId:  "user1",
		Title:   "test2",
		Message: "test2",
		IsRead:  false,
	})
	if err != nil {
		t.Error(err)
	}

	err = readNotification(config, "user1", test2.Id, test2)
	if err != nil {
		t.Error(err)
	}

	test3, err := createNotification(config, "user2", model.Notification{
		UserId:  "user2",
		Title:   "test3",
		Message: "test3",
		IsRead:  false,
	})
	if err != nil {
		t.Error(err)
	}
	err = readNotification(config, "user2", test3.Id, test3)
	if err != nil {
		t.Error(err)
	}

	t.Run("list user1", listNotifications(config, "user1", model.NotificationList{
		Total:         2,
		Limit:         10,
		Offset:        0,
		Notifications: []model.Notification{test1, test2},
	}))

	test1.IsRead = true
	err = updateNotification(config, "user1", test1)
	if err != nil {
		t.Error(err)
	}

	t.Run("list user1 after update", listNotifications(config, "user1", model.NotificationList{
		Total:         2,
		Limit:         10,
		Offset:        0,
		Notifications: []model.Notification{test1, test2},
	}))

	err = deleteNotification(config, "user1", test2.Id)
	if err != nil {
		t.Error(err)
	}

	t.Run("list user1 after delete", listNotifications(config, "user1", model.NotificationList{
		Total:         1,
		Limit:         10,
		Offset:        0,
		Notifications: []model.Notification{test1},
	}))

	// Try disallowed actions

	_, err = createNotification(config, "user2", model.Notification{
		Id:      "1234",
		UserId:  "user2",
		Title:   "test3",
		Message: "test3",
		IsRead:  false,
	})
	if err == nil {
		t.Error("was allowed to specified ID")
	}

	_, err = createNotification(config, "user2", model.Notification{
		UserId:  "user1",
		Title:   "test3",
		Message: "test3",
		IsRead:  false,
	})
	if err == nil {
		t.Error("was allowed to specify different user")
	}

	err = readNotification(config, "user2", test2.Id, test2)
	if err == nil {
		t.Error("was allowed to read from another user")
	}

	err = updateNotification(config, "user2", test2)
	if err == nil {
		t.Error("was allowed to update from another user")
	}

	err = deleteNotification(config, "user2", test2.Id)
	if err == nil {
		t.Error("was allowed to delete from another user")
	}
}
