package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`
	Channels []struct {
		Name      string `yaml:"name"`
		Ratelimit string `yaml:"ratelimit"`
	} `yaml:"channels"`
}

func LoadConfig(fp string) (*Config, error) {
	// create config file if not exists
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		if err := createConfigFile(fp); err != nil {
			return nil, fmt.Errorf("failed to create config file: %w", err)
		}
	}

	// open config file
	file, err := os.Open(fp)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	// decode config file
	var config Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	return &config, nil
}

func createConfigFile(fp string) error {
	config := Config{
		Redis: struct {
			Addr     string `yaml:"addr"`
			Password string `yaml:"password"`
			DB       int    `yaml:"db"`
		}{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		},
		Channels: []struct {
			Name      string `yaml:"name"`
			Ratelimit string `yaml:"ratelimit"`
		}{
			{
				Name:      "default",
				Ratelimit: "2/s",
			},
		},
	}

	// create directory if not exists
	if err := os.MkdirAll(filepath.Dir(fp), os.ModePerm); err != nil {
		return err
	}

	// create file
	file, err := os.Create(fp)
	if err != nil {
		return err
	}
	defer file.Close()

	// write config to file
	encoder := yaml.NewEncoder(file)
	if err := encoder.Encode(config); err != nil {
		return err
	}

	return nil
}
