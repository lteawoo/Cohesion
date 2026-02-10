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
	FtpEnabled    bool   `mapstructure:"ftp_enabled" json:"ftpEnabled" yaml:"ftp_enabled"`
	FtpPort       int    `mapstructure:"ftp_port" json:"ftpPort" yaml:"ftp_port"`
	SftpEnabled   bool   `mapstructure:"sftp_enabled" json:"sftpEnabled" yaml:"sftp_enabled"`
	SftpPort      int    `mapstructure:"sftp_port" json:"sftpPort" yaml:"sftp_port"`
}

type Datasource struct {
	URL      string `mapstructure:"url" json:"url" yaml:"url"`
	User     string `mapstructure:"user" json:"user" yaml:"user"`
	Password string `mapstructure:"password" json:"password" yaml:"password"`
	DBName   string `mapstructure:"dbname" json:"dbname" yaml:"dbname"`
}
