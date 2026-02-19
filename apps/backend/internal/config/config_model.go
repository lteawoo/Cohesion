package config

var Conf Config

type Config struct {
	Server     Server     `mapstructure:"server" json:"server" yaml:"server"`
	Datasource Datasource `mapstructure:"database" json:"database" yaml:"database"`
}

type Server struct {
	Port          string `mapstructure:"port" json:"port" yaml:"port"`
	HttpEnabled   bool   `mapstructure:"http_enabled" json:"httpEnabled" yaml:"http_enabled"`
	WebdavEnabled bool   `mapstructure:"webdav_enabled" json:"webdavEnabled" yaml:"webdav_enabled"`
	SftpEnabled   bool   `mapstructure:"sftp_enabled" json:"sftpEnabled" yaml:"sftp_enabled"`
	SftpPort      int    `mapstructure:"sftp_port" json:"sftpPort" yaml:"sftp_port"`
}

type Datasource struct {
	URL string `mapstructure:"url" json:"url" yaml:"url"`
}
