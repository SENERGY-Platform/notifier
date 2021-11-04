package mongo

import (
	"errors"
	"github.com/SENERGY-Platform/notifier/pkg/model"
	"github.com/SENERGY-Platform/notifier/pkg/persistence"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/http"
)

var brokerUserIdKey = "user_id"
var brokerIdKey = "id"

func initBrokers() {
	var err error

	CreateCollections = append(CreateCollections, func(db *Mongo) error {
		collection := db.client.Database(db.config.MongoTable).Collection(db.config.MongoBrokerCollection)
		err = db.ensureIndex(collection, "brokeruseridindex", brokerUserIdKey, true, false)
		if err != nil {
			return err
		}
		err = db.ensureIndex(collection, "brokeridindex", brokerIdKey, true, true)
		if err != nil {
			return err
		}
		return nil
	})
}

func (this *Mongo) brokerCollection() *mongo.Collection {
	return this.client.Database(this.config.MongoTable).Collection(this.config.MongoBrokerCollection)
}

func (this *Mongo) ListBrokers(userId string, o persistence.ListOptions) (result []model.Broker, total int64, err error, errCode int) {
	result = []model.Broker{}
	opt := options.Find()
	opt.SetLimit(int64(o.Limit))
	opt.SetSkip(int64(o.Offset))

	filter := bson.M{brokerUserIdKey: userId}

	ctx, _ := getTimeoutContext()
	collection := this.brokerCollection()

	total, err = collection.CountDocuments(ctx, filter)
	if err != nil {
		return result, total, err, http.StatusInternalServerError
	}
	cursor, err := collection.Find(ctx, filter, opt)
	if err != nil {
		return result, total, err, http.StatusInternalServerError
	}
	for cursor.Next(ctx) {
		element := model.Broker{}
		err = cursor.Decode(&element)
		if err != nil {
			return result, total, err, http.StatusInternalServerError
		}
		result = append(result, element)
	}
	err = cursor.Err()
	return
}

func (this *Mongo) ReadBroker(userId string, id string) (result model.Broker, err error, errCode int) {
	ctx, _ := getTimeoutContext()
	temp := this.brokerCollection().FindOne(
		ctx,
		bson.M{
			"$and": []bson.M{
				{brokerIdKey: id},
				{brokerUserIdKey: userId},
			},
		})
	err = temp.Err()
	if err == mongo.ErrNoDocuments {
		return result, err, http.StatusNotFound
	}
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	err = temp.Decode(&result)
	if err == mongo.ErrNoDocuments {
		return result, err, http.StatusNotFound
	}
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	return result, nil, http.StatusOK
}

func (this *Mongo) SetBroker(broker model.Broker) (error, int) {
	ctx, _ := getTimeoutContext()
	_, err := this.brokerCollection().ReplaceOne(
		ctx,
		bson.M{
			brokerIdKey: broker.Id,
		},
		broker,
		options.Replace().SetUpsert(true))
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, http.StatusOK
}

func (this *Mongo) RemoveBrokers(userId string, ids []string) (error, int) {
	ctx, _ := getTimeoutContext()
	collection := this.brokerCollection()

	filter := bson.M{
		"$and": []bson.M{
			{brokerUserIdKey: userId},
			{brokerIdKey: bson.M{
				"$in": ids,
			}},
		},
	}
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return err, http.StatusInternalServerError
	}

	if total != int64(len(ids)) {
		return errors.New("did not find all ids"), http.StatusNotFound
	}

	_, err = collection.DeleteMany(ctx, filter)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, http.StatusOK
}
