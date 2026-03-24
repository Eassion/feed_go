package config

import (
	"log"

	viper "github.com/spf13/viper"
)

var AppConfig *Config

type Config struct {
	ServerConfig   ServerConfig   `mapstructure:"server"`
	DatabaseConfig DatabaseConfig `mapstructure:"database"`
	RedisConfig    RedisConfig    `mapstructure:"redis"`
	RabbitMQConfig RabbitMQConfig `mapstructure:"rabbitmq"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type RabbitMQConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

func InitConfig() {
	AppConfig = &Config{}
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("../../config")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	if err := viper.Unmarshal(AppConfig); err != nil {
		log.Fatalf("Error unmarshalling config struct: %v", err)
	}

	log.Printf("Server port: %d", AppConfig.ServerConfig.Port)
}
