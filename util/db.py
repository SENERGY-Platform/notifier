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
from bson import ObjectId
from pymongo import MongoClient, ReturnDocument, ASCENDING, DESCENDING
import os
import logging
from datetime import datetime, timezone


class DB():
    def __init__(self):
        self.client = MongoClient(os.getenv('MONGO_ADDR', 'localhost'), 27017)
        self.db = self.client.db
        self.notifications = self.db.notifications
        self.logger = logging.getLogger('util.db')

    def create_notification(self, notification, user_id=None):
        if user_id is not None and notification['userId'] != user_id:
            raise ValueError('userIds do not match')
        if "created_at" not in notification:
            notification["created_at"] = datetime.now(tz=timezone.utc).replace(microsecond=0).isoformat()
        notification_id = self.notifications.insert_one(notification).inserted_id
        return self.notifications.find_one({'_id': notification_id})

    def read_notification(self, notification_id, user_id=None):
        if user_id is not None:
            return self.notifications.find_one({'$and': [{'_id': ObjectId(notification_id)}, {'userId': user_id}]})
        return self.notifications.find_one({'$and': [{'_id': ObjectId(notification_id)}]})

    def update_notification(self, notification, notification_id, user_id=None):
        if user_id is not None:
            return self.notifications.find_one_and_update(
                {'$and': [{'_id': ObjectId(notification_id)}, {'userId': user_id}]},
                {'$set': notification, }, return_document=ReturnDocument.AFTER)

        return self.notifications.find_one_and_update({'$and': [{'_id': ObjectId(notification_id)}]},
                                                      {'$set': notification, }, return_document=ReturnDocument.AFTER)

    def delete_notification(self, notification_id, user_id=None):
        if user_id is not None:
            return self.notifications.delete_one({'$and': [{'_id': ObjectId(notification_id)}, {'userId': user_id}]})
        return self.notifications.delete_one({'$and': [{'_id': ObjectId(notification_id)}]})

    def list_notifications(self, limit=0, offset=0, sort=None, user_id=None):
        if sort is None:
            sort = ["_id", "desc"]

        if type(sort) is not list or len(sort) != 2:
            raise ValueError

        if user_id is not None:
            db_notifications = self.notifications.find({'$or': [{'pub': True}, {'userId': user_id}]}) \
                .skip(offset).limit(limit).sort(sort[0], ASCENDING if sort[1] == "asc" else DESCENDING)
        else:
            db_notifications = self.notifications.find() \
                .skip(offset).limit(limit).sort(sort[0], ASCENDING if sort[1] == "asc" else DESCENDING)

        r = []
        for n in db_notifications:
            r.append(n)
        return r

    def delete_all(self):
        d = self.notifications.delete_many({})
        if d.deleted_count > 0:
            self.logger.warning(
                'deleted ' + str(d.deleted_count) + ' notifications from db, because delete_all was called')
        return d
