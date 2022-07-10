package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"cloud.google.com/go/storage"
)

type configType struct {
	AppID string `json:"appId"`
	Token string `json:"token"`
}

func loadConfig() (configType, error) {
	configBucket := os.Getenv("CONFIG_BUCKET")
	configObject := os.Getenv("CONFIG_OBJECT")
	if configBucket == "" || configObject == "" {
		return configType{}, errors.New("no CONFIG_BUCKET and/or CONFIG_OBJECT provided")
	}

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return configType{}, err
	}

	r, err := client.Bucket(configBucket).Object(configObject).NewReader(ctx)
	if err != nil {
		return configType{}, err
	}

	f, err := ioutil.ReadAll(r)
	r.Close()
	if err != nil {
		return configType{}, err
	}

	config := configType{}
	err = json.Unmarshal(f, &config)
	return config, err
}
