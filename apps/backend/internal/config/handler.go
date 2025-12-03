package config

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func SetConfig(goEnv string) {
	log.Info().Msgf("Loading configuration for environment: %s", goEnv)

	viper.AddConfigPath("config")
	viper.SetConfigType("yaml")

	if goEnv == "production" {
		viper.SetConfigName("config.prod")
	} else {
		viper.SetConfigName("config.dev")
	}

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read config file")
	}

	err = viper.Unmarshal(&Conf)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to unmarshal config")
	}
}
