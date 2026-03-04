package config

var Conf Config

type Config struct {
	Server     Server     `mapstructure:"server" json:"server" yaml:"server"`
	Datasource Datasource `mapstructure:"database" json:"database" yaml:"database"`
}

type Server struct {
	Port            string `mapstructure:"port" json:"port" yaml:"port"`
	WebdavEnabled   bool   `mapstructure:"webdav_enabled" json:"webdavEnabled" yaml:"webdav_enabled"`
	FtpEnabled      bool   `mapstructure:"ftp_enabled" json:"ftpEnabled" yaml:"ftp_enabled"`
	FtpPort         int    `mapstructure:"ftp_port" json:"ftpPort" yaml:"ftp_port"`
	SftpEnabled     bool   `mapstructure:"sftp_enabled" json:"sftpEnabled" yaml:"sftp_enabled"`
	SftpPort        int    `mapstructure:"sftp_port" json:"sftpPort" yaml:"sftp_port"`
	SmbEnabled      bool   `mapstructure:"smb_enabled" json:"smbEnabled" yaml:"smb_enabled"`
	SmbPort         int    `mapstructure:"smb_port" json:"smbPort" yaml:"smb_port"`
	SmbRolloutPhase string `mapstructure:"smb_rollout_phase" json:"smbRolloutPhase" yaml:"smb_rollout_phase"`
}

const (
	SMBEndpointModeDirect = "direct"

	SMBVersion21  = "2.1"
	SMBVersion300 = "3.0"
	SMBVersion302 = "3.0.2"
	SMBVersion311 = "3.1.1"

	DefaultSMBPort       = 445
	DefaultSMBMinVersion = SMBVersion21
	DefaultSMBMaxVersion = SMBVersion311

	SMBRolloutPhaseReadOnly  = "readonly"
	SMBRolloutPhaseWriteSafe = "write-safe"
	SMBRolloutPhaseWriteFull = "write-full"
	DefaultSMBRolloutPhase   = SMBRolloutPhaseReadOnly
)

type Datasource struct {
	URL string `mapstructure:"url" json:"url" yaml:"url"`
}
