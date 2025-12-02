package config

var Conf Config

type Config struct {
	Server     Server     `mapstructure:"server"`
	Datasource Datasource `mapstructure:"database"`
}

type Server struct {
	Port string `mapstructure:"port"`
}

type Datasource struct {
	URL      string `mapstructure:"url"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}
