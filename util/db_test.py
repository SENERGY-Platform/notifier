# Copyright 2020 InfAI (CC SES)
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


import unittest

from bson import ObjectId

import util.db as db


class DBTestCase(unittest.TestCase):
    def setUp(self) -> None:
        db.delete_all()

    def tearDown(self) -> None:
        db.delete_all()

    def test_delete_all(self):
        i = 10
        notifications = get_test_notification_many(i)
        for j in range(0, i):
            db.create_notification(notifications[j])
        self.assertEqual(db.delete_all().deleted_count, i)

    def test_crud_admin(self):
        notification = get_test_notification()
        notification = db.create_notification(notification)
        self.assertEqual(notification, db.read_notification(notification["_id"]), "create-read")
        notification["message"] = "newMessage"
        db.update_notification(notification, notification["_id"])
        self.assertEqual(notification, db.read_notification(notification["_id"]), "update-read")
        db.delete_notification(notification["_id"])
        self.assertIsNone(db.read_notification(notification["_id"], "delete-read"))

    def test_crud_user(self):
        notification = get_test_notification()
        db.create_notification(notification, notification["userId"])
        self.assertEqual(notification, db.read_notification(notification["_id"], notification["userId"]), "create-read")
        notification["message"] = "newMessage"
        db.update_notification(notification, notification["_id"], notification["userId"])
        self.assertEqual(notification, db.read_notification(notification["_id"], notification["userId"]), "update-read")
        db.delete_notification(notification["_id"], notification["userId"])
        self.assertIsNone(db.read_notification(notification["_id"], notification["userId"]), "delete-read")

    def test_read_nonexistent(self):
        self.assertIsNone(db.read_notification(ObjectId()))

    def test_update_nonexistent(self):
        notification = get_test_notification()
        self.assertIsNone(db.update_notification(notification, notification["_id"]))

    def test_delete_nonexistent(self):
        self.assertEqual(db.delete_notification(ObjectId()).deleted_count, 0)

    def test_read_wrong_user_id(self):
        notification = get_test_notification()
        db.create_notification(notification)
        self.assertIsNone(db.read_notification(notification["_id"], "another_user"))

    def test_create_wrong_user_id(self):
        with self.assertRaises(ValueError):
            db.create_notification(get_test_notification(), "another_user")

    def test_update_wrong_user_id(self):
        notification = get_test_notification()
        self.assertIsNone(db.update_notification(notification, notification["_id"], "another_user"))

    def test_delete_wrong_user_id(self):
        notification = get_test_notification()
        db.create_notification(notification)
        self.assertEqual(db.delete_notification(notification["_id"], "another_user").deleted_count, 0)

    def test_list_admin(self):
        i = 10
        notifications = get_test_notification_many(i)
        for j in range(0, i):
            db.create_notification(notifications[j])
        notifications = db.list_notifications()
        self.assertEqual(len(notifications), i)

    def test_list_user(self):
        i = 10
        notifications = get_test_notification_many(i)
        for j in range(0, i):
            db.create_notification(notifications[j])
        different_notification = get_test_notification()
        different_notification["_id"] = "different"
        different_notification["userId"] = "another_user"
        notifications = db.list_notifications(user_id=notifications[0]["userId"])
        self.assertEqual(len(notifications), i)

    def test_limit(self):
        i = 10
        notifications = get_test_notification_many(i)
        for j in range(0, i):
            db.create_notification(notifications[j])
        notifications = db.list_notifications(limit=3)
        self.assertEqual(len(notifications), 3)

    def test_offset(self):
        i = 10
        notifications = get_test_notification_many(i)
        for j in range(0, i):
            db.create_notification(notifications[j])
        notifications = db.list_notifications(offset=3)
        self.assertEqual(len(notifications), 7)

    def test_sort(self):
        i = 10
        notifications = get_test_notification_many(i)
        text = "aaa"
        notifications[0]["message"] = text
        for j in range(0, i):
            db.create_notification(notifications[j])
        notifications = db.list_notifications(sort=["message", "asc"])
        self.assertEqual(notifications[0]["message"], text)

    def test_wrong_sort(self):
        with self.assertRaises(ValueError):
            db.list_notifications(sort="a")
        with self.assertRaises(ValueError):
            db.list_notifications(sort=["a", "b", "c"])


if __name__ == '__main__':
    unittest.main()


def get_test_notification():
    return {"_id": ObjectId(), "message": "test", "userId": "testUser", "isRead": False, "title": "testTitle"}


def get_test_notification_many(i):
    notifications = []
    for _ in range(0, i):
        notification = get_test_notification()
        notifications.append(notification)
    return notifications
