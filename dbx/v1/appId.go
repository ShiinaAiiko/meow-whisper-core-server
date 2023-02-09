package dbxV1

import (
	"context"

	"github.com/ShiinaAiiko/meow-whisper-core/models"
	"go.mongodb.org/mongo-driver/bson"
)

type AppIdDbx struct {
}

func (d *AppIdDbx) CreateApp(name string, avatar string) (*models.AppId, error) {
	app := models.AppId{
		Avatar: avatar,
		Name:   name,
	}
	if err := app.Default(); err != nil {
		return nil, err
	}
	_, err := app.GetCollection().InsertOne(context.TODO(), app)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (cr *AppIdDbx) GetAppByName(name string) *models.AppId {
	// 后续启用redis
	app := new(models.AppId)

	filter := bson.M{
		"name": name,
	}
	err := app.GetCollection().FindOne(context.TODO(), filter).Decode(app)
	if err != nil {
		return nil
	}
	return app
}
