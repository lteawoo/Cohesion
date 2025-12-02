package config

import (
	"log"

	"github.com/spf13/viper"
)

func SetConfig(goEnv string) {
	log.Printf("Loading configuration for environment: %s", goEnv)

	viper.AddConfigPath("config")
	viper.SetConfigType("yaml")

	if goEnv == "production" {
		viper.SetConfigName("config.prod")
	} else {
		viper.SetConfigName("config.dev")
	}

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	err = viper.Unmarshal(&Conf)
	if err != nil {
		log.Fatalf("Failed to unmarshal config: %v", err)
	}
}
