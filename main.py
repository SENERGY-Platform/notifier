# Copyright 2018 InfAI (CC SES)
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

import os

from bson.objectid import ObjectId
from flask import Flask, request, abort
from flask_restx import Api, Resource, fields, reqparse
from flask_cors import CORS
import json
from pymongo import MongoClient, ReturnDocument, ASCENDING, DESCENDING
from dotenv import load_dotenv
load_dotenv()

application = Flask("notification-service")
application.config.SWAGGER_UI_DOC_EXPANSION = 'list'
CORS(application)
api = Api(application, version='0.1', title='Notification Service API',
          description='Notification Service API')


@api.route('/doc')
class Docs(Resource):
    def get(self):
        return api.__schema__


client = MongoClient(os.getenv('MONGO_ADDR', 'localhost'), 27017)

db = client.db

notifications = db.notifications

ns = api.namespace('notifications', description='Operations related to notifications')
admin = api.namespace('admin', description='Admin operations related to notifications. Will not perform X-UserID checks.'
                                           ' API will not be accessible from outside the platform.')

notification_model = api.model('Notification', {
    'userId': fields.String(required=True, description='User ID'),
    'title': fields.String(required=True, description='Title'),
    'message': fields.String(required=True, description='Message'),
    'isRead': fields.Boolean(required=True, description='If the message has been read')
})

notification_return = notification_model.clone('Notification', {
    '_id': fields.String(required=True, description='Notification id'),
    'userId': fields.String
})

notification_list = api.model('NotificationList', {
    "notifications": fields.List(fields.Nested(notification_return))
})


@ns.route('/', strict_slashes=False)
class Operator(Resource):
    @api.expect(notification_model)
    @api.marshal_with(notification_return, code=201)
    @api.response(403, 'Forbidden')
    def put(self):
        """Creates a notification."""
        req = request.get_json()
        user_id = getUserId(request)
        if (req['userId'] == user_id):
            operator_id = notifications.insert_one(req).inserted_id
            o = notifications.find_one({'_id': operator_id})
            print("Added notification: " + json.dumps({"_id": str(operator_id)}) + " for user " + req['userId'])
            return o, 201
        else:
            abort(403, 'You may only send messages to yourself')

    @api.marshal_with(notification_list, code=200)
    def get(self):
        """Returns a list of notifications."""
        parser = reqparse.RequestParser()
        parser.add_argument('limit', type=int, help='Limit', location='args')
        parser.add_argument('offset', type=int, help='Offset', location='args')
        parser.add_argument('sort', type=str, help='Sort', location='args')
        args = parser.parse_args()
        limit = 0
        if not (args["limit"] is None):
            limit = args["limit"]
        offset = 0
        if not (args["offset"] is None):
            offset = args["offset"]
        if not (args["sort"] is None):
            sort = args["sort"].split(":")
        else:
            sort = ["_id", "desc"]
        user_id = getUserId(request)

        nots = notifications.find({'$or': [{'pub': True}, {'userId': user_id}]}) \
            .skip(offset).limit(limit).sort(sort[0], ASCENDING if sort[1] == "asc" else DESCENDING)

        notifications_list = []
        for o in nots:
            notifications_list.append(o)
        print("User " + user_id + " read " + str(len(notifications_list)) + " notifications")
        return {"notifications": notifications_list}


@ns.route('/<string:notification_id>', strict_slashes=False)
@api.response(404, 'Notification not found.')
@api.response(400, 'Bad request')
class OperatorUpdate(Resource):
    @api.marshal_with(notification_return)
    def get(self, notification_id):
        """Get a single notification. This will perform userId checks and returns 404, even if this messages exists, but the userId isn't matching """
        user_id = getUserId(request)
        try:
            o = notifications.find_one({'$and': [{'_id': ObjectId(notification_id)}, {'userId': user_id}]})
        except Exception as e:
            abort(400, str(e))
        print(o)
        if o is not None:
            return o, 200
        abort(404, "Notification not found")

    @api.expect(notification_model)
    @api.marshal_with(notification_return)
    @api.response(403, 'Forbidden')
    def post(self, notification_id):
        """Updates a notification."""
        user_id = getUserId(request)
        req = request.get_json()
        if (req['userId'] == user_id):
            try:
                operator = notifications.find_one_and_update({'$and': [{'_id': ObjectId(notification_id)}, {'userId': user_id}]}, {
                    '$set': req,
                },
                                                             return_document=ReturnDocument.AFTER)
            except Exception as e:
                abort(400, str(e))
            if operator is not None:
                return operator, 200
            abort(404, "Notification not found")
        else:
            abort(403, 'You may only update your own messages')

    @api.response(204, "Deleted")
    def delete(self, notification_id):
        """Deletes a notification."""
        user_id = getUserId(request)
        try:
            o = notifications.find_one({'$and': [{'_id': ObjectId(notification_id)}, {'userId': user_id}]})
        except Exception as e:
            abort(400, str(e))
        if o is not None:
            notifications.delete_one({'_id': ObjectId(notification_id)})
            return "Deleted", 204
        abort(404, "Notification not found")


@admin.route('/', strict_slashes=False)
class Operator(Resource):
    @api.expect(notification_model)
    @api.marshal_with(notification_return, code=201)
    def put(self):
        """Creates a notification."""
        req = request.get_json()
        operator_id = notifications.insert_one(req).inserted_id
        o = notifications.find_one({'_id': operator_id})
        print("Added notification: " + json.dumps({"_id": str(operator_id)}) + " for user " + req['userId'])
        return o, 201

    @api.marshal_with(notification_list, code=200)
    def get(self):
        """Returns a list of notifications."""
        parser = reqparse.RequestParser()
        parser.add_argument('limit', type=int, help='Limit', location='args')
        parser.add_argument('offset', type=int, help='Offset', location='args')
        parser.add_argument('sort', type=str, help='Sort', location='args')
        args = parser.parse_args()
        limit = 0
        if not (args["limit"] is None):
            limit = args["limit"]
        offset = 0
        if not (args["offset"] is None):
            offset = args["offset"]
        if not (args["sort"] is None):
            sort = args["sort"].split(":")
        else:
            sort = ["_id", "desc"]

        nots = notifications.find() \
            .skip(offset).limit(limit).sort(sort[0], ASCENDING if sort[1] == "asc" else DESCENDING)

        notifications_list = []
        for o in nots:
            notifications_list.append(o)
        print("Admin API delivered " + str(len(notifications_list)) + " notifications")
        return {"notifications": notifications_list}


@admin.route('/<string:notification_id>', strict_slashes=False)
@api.response(404, 'Notification not found.')
@api.response(400, 'Bad request')
class OperatorUpdate(Resource):
    @api.marshal_with(notification_return)
    def get(self, notification_id):
        """Get a single notification."""
        try:
            o = notifications.find_one({'_id': ObjectId(notification_id)})
        except Exception as e:
            abort(400, str(e))
        print(o)
        return o, 200

    @api.expect(notification_model)
    @api.marshal_with(notification_return)
    def post(self, notification_id):
        """Updates a notification."""
        req = request.get_json()
        try:
            operator = notifications.find_one_and_update({'$and': [{'_id': ObjectId(notification_id)}]}, {
                '$set': req,
            },
                                                         return_document=ReturnDocument.AFTER)
        except Exception as e:
            abort(400, str(e))
        if operator is not None:
            return operator, 200
        abort(404, "Notification not found")

    @api.response(204, "Deleted")
    def delete(self, notification_id):
        """Deletes a notification."""
        try:
            o = notifications.find_one({'$and': [{'_id': ObjectId(notification_id)}]})
        except Exception as e:
            abort(400, str(e))
        if o is not None:
            notifications.delete_one({'_id': ObjectId(notification_id)})
            return "Deleted", 204
        abort(404, "Notification not found")


def getUserId(req):
    user_id = req.headers.get('X-UserID')
    if user_id is None:
        user_id = os.getenv('DUMMY_USER', 'test')
    return user_id


if __name__ == "__main__":
    application.run("0.0.0.0", os.getenv('PORT', 5000), debug=False)