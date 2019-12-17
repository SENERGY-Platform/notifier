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
from flask import Flask, request
from flask_restplus import Api, Resource, fields, reqparse
from flask_cors import CORS
import json
from pymongo import MongoClient, ReturnDocument, ASCENDING, DESCENDING
from dotenv import load_dotenv
load_dotenv()

app = Flask("notification-service")
app.config.SWAGGER_UI_DOC_EXPANSION = 'list'
CORS(app)
api = Api(app, version='0.1', title='Notification Service API',
          description='Notification Service API')


@api.route('/doc')
class Docs(Resource):
    def get(self):
        return api.__schema__


client = MongoClient(os.getenv('MONGO_ADDR', 'localhost'), 27017)

db = client.db

notifications = db.notifications

ns = api.namespace('notifications', description='Operations related to notifications')

notification_model = api.model('Notification', {
    'userId': fields.String(required=True, description='User ID'),
    'title': fields.String(required=True, description='Title'),
    'message': fields.String(required=True, description='Message'),
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
    def put(self):
        """Creates a notification."""
        req = request.get_json()
        operator_id = notifications.insert_one(req).inserted_id
        o = notifications.find_one({'_id': operator_id})
        print("Added notification: " + json.dumps({"_id": str(operator_id)}) + " for user " + getUserId(request))
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
        user_id = getUserId(request)

        nots = notifications.find({'$or': [{'pub': True}, {'userId': user_id}]}) \
            .skip(offset).limit(limit).sort(sort[0], ASCENDING if sort[1] == "asc" else DESCENDING)

        notifications_list = []
        for o in nots:
            notifications_list.append(o)
        print("User " + user_id + " read " + len(notifications_list) + " notifications")
        return {"notifications": notifications_list}


@ns.route('/<string:notification_id>', strict_slashes=False)
@api.response(404, 'Operator not found.')
class OperatorUpdate(Resource):
    @api.marshal_with(notification_return)
    def get(self, notification_id):
        """Get a single notification."""
        o = notifications.find_one({'_id': ObjectId(notification_id)})
        print(o)
        return o, 200

    @api.expect(notification_model)
    @api.marshal_with(notification_return)
    def post(self, notification_id):
        """Updates a notification."""
        user_id = getUserId(request)
        req = request.get_json()
        operator = notifications.find_one_and_update({'$and': [{'_id': ObjectId(notification_id)}, {'userId': user_id}]}, {
            '$set': req,
        },
                                                     return_document=ReturnDocument.AFTER)
        if operator is not None:
            return operator, 200
        return "Operator not found", 404

    @api.response(204, "Deleted")
    def delete(self, notification_id):
        """Deletes a notification."""
        user_id = getUserId(request)
        o = notifications.find_one({'$and': [{'_id': ObjectId(notification_id)}, {'userId': user_id}]})
        if o is not None:
            notifications.delete_one({'_id': ObjectId(notification_id)})
            return "Deleted", 204
        return "Notification not found", 404


def getUserId(req):
    user_id = req.headers.get('X-UserID')
    if user_id is None:
        user_id = os.getenv('DUMMY_USER', 'test')
    return user_id


if __name__ == "__main__":
    app.run("0.0.0.0", os.getenv('PORT', 5000), debug=False)