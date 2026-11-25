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

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/SENERGY-Platform/notifier/pkg"
	"github.com/SENERGY-Platform/notifier/pkg/configuration"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestNotificationCRUD(t *testing.T) {
	wg, ctx, cancel, conf, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	defer wg.Wait()
	defer cancel()

	err = pkg.Start(ctx, wg, conf)
	if err != nil {
		t.Error(err)
		return
	}

	time.Sleep(time.Second)

	t.Run("empty list", listNotifications(conf, "user1", model.NotificationList{
		Total:         0,
		Limit:         10,
		Offset:        0,
		Notifications: []model.Notification{},
	}))

	test1, err := createNotification(conf, "user1", model.Notification{
		UserId:  "user1",
		Title:   "test1",
		Message: "test1",
		IsRead:  false,
		Topic:   model.TopicDeveloper,
	}, nil)
	if err != nil {
		t.Error(err)
	}

	err = readNotification(conf, "user1", test1.Id, test1)
	if err != nil {
		t.Error(err)
	}

	test2, err := createNotificationLegacy(conf, "user1", model.Notification{
		UserId:  "user1",
		Title:   "test2",
		Message: "test2",
		IsRead:  false,
		Topic:   model.TopicMGW,
	})
	if err != nil {
		t.Error(err)
	}

	err = readNotification(conf, "user1", test2.Id, test2)
	if err != nil {
		t.Error(err)
	}

	test3, err := createNotification(conf, "user2", model.Notification{
		UserId:  "user2",
		Title:   "test3",
		Message: "test3",
		IsRead:  false,
		Topic:   model.TopicConnector,
	}, nil)
	if err != nil {
		t.Error(err)
	}
	err = readNotification(conf, "user2", test3.Id, test3)
	if err != nil {
		t.Error(err)
	}

	t.Run("list user1", listNotifications(conf, "user1", model.NotificationList{
		Total:         2,
		Limit:         10,
		Offset:        0,
		Notifications: []model.Notification{test1, test2},
	}))

	test1.IsRead = true
	err = updateNotification(conf, "user1", test1)
	if err != nil {
		t.Error(err)
	}

	t.Run("list user1 after update", listNotifications(conf, "user1", model.NotificationList{
		Total:         2,
		Limit:         10,
		Offset:        0,
		Notifications: []model.Notification{test1, test2},
	}))

	err = deleteNotification(conf, "user1", test2.Id)
	if err != nil {
		t.Error(err)
	}

	t.Run("list user1 after delete", listNotifications(conf, "user1", model.NotificationList{
		Total:         1,
		Limit:         10,
		Offset:        0,
		Notifications: []model.Notification{test1},
	}))

	// Try disallowed actions

	_, err = createNotification(conf, "user2", model.Notification{
		Id:      "1234",
		UserId:  "user2",
		Title:   "test3",
		Message: "test3",
		IsRead:  false,
	}, nil)
	if err == nil {
		t.Error("was allowed to specify ID")
	}

	_, err = createNotification(conf, "user2", model.Notification{
		Id:      "1234",
		UserId:  "user2",
		Title:   "test3",
		Message: "test3",
		IsRead:  false,
		Topic:   model.TopicUnknown,
	}, nil)
	if err == nil {
		t.Error("was allowed to specify topic unknown")
	}

	_, err = createNotification(conf, "user2", model.Notification{
		UserId:  "user1",
		Title:   "test3",
		Message: "test3",
		IsRead:  false,
	}, nil)
	if err == nil {
		t.Error("was allowed to specify different user")
	}

	err = readNotification(conf, "user2", test2.Id, test2)
	if err == nil {
		t.Error("was allowed to read from another user")
	}

	err = updateNotification(conf, "user2", test2)
	if err == nil {
		t.Error("was allowed to update from another user")
	}

	err = deleteNotification(conf, "user2", test2.Id)
	if err == nil {
		t.Error("was allowed to delete from another user")
	}

	t.Run("test deduplication", func(t *testing.T) {
		var sixty int64 = 60

		testNoDupe, err := createNotification(conf, "--", model.Notification{
			UserId:  "--",
			Title:   "dupe",
			Message: "dupe",
			IsRead:  false,
		}, &sixty)
		if err != nil {
			t.Error(err)
		}
		testDupe, err := createNotification(conf, "--", model.Notification{
			UserId:  "--",
			Title:   "dupe",
			Message: "dupe",
			IsRead:  false,
		}, &sixty)
		if err != nil {
			t.Error(err)
		}
		if !testNoDupe.Equal(testDupe) {
			t.Error("duplicate created even though ignoreDuplicatesWithinSeconds specified")
		}

		testNoDupe2, err := createNotification(conf, "--2", model.Notification{
			UserId:  "--2",
			Title:   "dupe",
			Message: "dupe",
			IsRead:  false,
		}, &sixty)
		if err != nil {
			t.Error(err)
		}
		if testNoDupe.Equal(testNoDupe2) {
			t.Error("did not create unique notification when ignoreDuplicatesWithinSeconds specified")
		}
	})
}

func TestNotificationTag(t *testing.T) {
	wg, ctx, cancel, conf, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	defer wg.Wait()
	defer cancel()

	err = pkg.Start(ctx, wg, conf)
	if err != nil {
		t.Error(err)
		return
	}

	time.Sleep(time.Second)

	first, err := createNotification(conf, "tag-user1", model.Notification{
		UserId:  "tag-user1",
		Title:   "Trockner laeuft",
		Message: "2h verbleibend",
		Topic:   model.TopicProcesses,
		Tag:     "dryer",
		Silent:  true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Tag != "dryer" || !first.Silent {
		t.Error("tag or silent not returned", first)
	}
	if err = readNotification(conf, "tag-user1", first.Id, first); err != nil {
		t.Error(err)
	}

	time.Sleep(10 * time.Millisecond)

	second, err := createNotification(conf, "tag-user1", model.Notification{
		UserId:  "tag-user1",
		Title:   "Trockner fertig!",
		Message: "fertig",
		Topic:   model.TopicProcesses,
		Tag:     "dryer",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.Id != first.Id {
		t.Error("same tag did not return same id", first.Id, second.Id)
	}
	if second.Title != "Trockner fertig!" || second.Message != "fertig" || second.Silent {
		t.Error("notification not replaced", second)
	}
	if !second.CreatedAt.After(first.CreatedAt) {
		t.Error("created_at not advanced", first.CreatedAt, second.CreatedAt)
	}
	if err = readNotification(conf, "tag-user1", second.Id, second); err != nil {
		t.Error(err)
	}
	t.Run("list has one", listNotifications(conf, "tag-user1", model.NotificationList{
		Total:         1,
		Limit:         10,
		Offset:        0,
		Notifications: []model.Notification{second},
	}))

	// a client update (mobile app) does not carry tag and silent and must not detach the notification
	clientUpdate := model.Notification{
		Id:      second.Id,
		UserId:  "tag-user1",
		Title:   second.Title,
		Message: second.Message,
		IsRead:  true,
	}
	if err = updateNotification(conf, "tag-user1", clientUpdate); err != nil {
		t.Fatal(err)
	}
	second.IsRead = true
	if err = readNotification(conf, "tag-user1", second.Id, second); err != nil {
		t.Error("tag or silent lost on update:", err)
	}
	third, err := createNotification(conf, "tag-user1", model.Notification{
		UserId:  "tag-user1",
		Title:   "Trockner laeuft wieder",
		Message: "1h verbleibend",
		Topic:   model.TopicProcesses,
		Tag:     "dryer",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if third.Id != first.Id {
		t.Error("tag lost after client update, got new id", third.Id)
	}
	if third.IsRead {
		t.Error("replacement must be unread")
	}

	other, err := createNotification(conf, "tag-user2", model.Notification{
		UserId:  "tag-user2",
		Title:   "Trockner laeuft",
		Message: "2h verbleibend",
		Tag:     "dryer",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if other.Id == first.Id {
		t.Error("same tag of different user returned same id")
	}

	untagged1, err := createNotification(conf, "tag-user1", model.Notification{UserId: "tag-user1", Title: "x", Message: "x"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	untagged2, err := createNotification(conf, "tag-user1", model.Notification{UserId: "tag-user1", Title: "x", Message: "x"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if untagged1.Id == untagged2.Id {
		t.Error("untagged notifications must not be replaced")
	}
}

func listNotifications(config configuration.Config, userId string, expected model.NotificationList) func(t *testing.T) {
	return func(t *testing.T) {
		token, err := createToken(userId)
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequest("GET", "http://localhost:"+config.ApiPort+"/notifications?limit=10", nil)
		if err != nil {
			t.Error(err)
			return
		}
		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		req.WithContext(ctx)
		req.Header.Set("Authorization", token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Error(resp.StatusCode, string(b))
			return
		}
		actual := model.NotificationList{}
		err = json.NewDecoder(resp.Body).Decode(&actual)
		if err != nil {
			t.Error(err)
			return
		}

		if !actual.Equal(expected) {
			t.Error(actual, expected)
			return
		}
	}
}

func createNotification(config configuration.Config, userId string, notification model.Notification, ignoreDuplicatesWithinSeconds *int64) (result model.Notification, err error) {
	var token string
	token, err = createToken(userId)
	if err != nil {
		return
	}
	b := new(bytes.Buffer)
	err = json.NewEncoder(b).Encode(notification)
	if err != nil {
		return
	}
	url := "http://localhost:" + config.ApiPort + "/notifications"
	if ignoreDuplicatesWithinSeconds != nil {
		url += "?ignore_duplicates_within_seconds=" + strconv.FormatInt(*ignoreDuplicatesWithinSeconds, 10)
	}
	req, err := http.NewRequest("POST", url, b)
	if err != nil {
		return
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	req.WithContext(ctx)
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return result, errors.New(string(b))
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return
}

func createNotificationLegacy(config configuration.Config, userId string, notification model.Notification) (result model.Notification, err error) {
	var token string
	token, err = createToken(userId)
	if err != nil {
		return
	}
	b := new(bytes.Buffer)
	err = json.NewEncoder(b).Encode(notification)
	if err != nil {
		return
	}
	req, err := http.NewRequest("PUT", "http://localhost:"+config.ApiPort+"/notifications", b)
	if err != nil {
		return
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	req.WithContext(ctx)
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return result, errors.New(string(b))
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return
}

func updateNotification(config configuration.Config, userId string, notification model.Notification) error {
	token, err := createToken(userId)
	if err != nil {
		return err
	}
	b := new(bytes.Buffer)
	err = json.NewEncoder(b).Encode(notification)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", "http://localhost:"+config.ApiPort+"/notifications/"+notification.Id, b)
	if err != nil {
		return err
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	req.WithContext(ctx)
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return errors.New(string(b))
	}
	return nil
}

func readNotification(config configuration.Config, userId string, id string, expected model.Notification) error {
	token, err := createToken(userId)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("GET", "http://localhost:"+config.ApiPort+"/notifications/"+id, nil)
	if err != nil {
		return err
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	req.WithContext(ctx)
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return errors.New(string(b))
	}
	actual := model.Notification{}
	err = json.NewDecoder(resp.Body).Decode(&actual)
	if err != nil {
		return err
	}

	if !actual.Equal(expected) {
		return errors.New("actual not expected")
	}
	return nil
}

func deleteNotification(config configuration.Config, userId string, id string) error {
	token, err := createToken(userId)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("DELETE", "http://localhost:"+config.ApiPort+"/notifications/"+id, nil)
	if err != nil {
		return err
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	req.WithContext(ctx)
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return errors.New(string(b))
	}
	return nil
}
