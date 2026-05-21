package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Security SecurityConfig `mapstructure:"security"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Kernel   KernelConfig   `mapstructure:"kernel"`
	TI       TIConfig       `mapstructure:"ti"`
	Analysis AnalysisConfig `mapstructure:"analysis"`
	Refresh  RefreshConfig  `mapstructure:"refresh"`
	TinyClaw TinyClawConfig `mapstructure:"tinyclaw"`
}

type TinyClawConfig struct {
	DownloadURL       string `mapstructure:"download_url"`
	RPCServerWinURL   string `mapstructure:"rpcserver_win_url"`
	RPCServerLinuxURL string `mapstructure:"rpcserver_linux_url"`
	TinyClawWinURL    string `mapstructure:"tinyclaw_win_url"`
	TinyClawLinuxURL  string `mapstructure:"tinyclaw_linux_url"`
	SkillDownloadCmd  string `mapstructure:"skill_download_cmd"`
	SkillPrompt       string `mapstructure:"skill_prompt"`
	Showcase1Icon     string `mapstructure:"showcase1_icon"`
	Showcase1Title    string `mapstructure:"showcase1_title"`
	Showcase1Desc     string `mapstructure:"showcase1_desc"`
	Showcase2Icon     string `mapstructure:"showcase2_icon"`
	Showcase2Title    string `mapstructure:"showcase2_title"`
	Showcase2Desc     string `mapstructure:"showcase2_desc"`
	Showcase3Icon     string `mapstructure:"showcase3_icon"`
	Showcase3Title    string `mapstructure:"showcase3_title"`
	Showcase3Desc     string `mapstructure:"showcase3_desc"`
}

type AnalysisConfig struct {
	IDALibPath string `mapstructure:"idalib_path"`
}

type RefreshConfig struct {
	Enabled  bool `mapstructure:"enabled"`
	Interval int  `mapstructure:"interval"` // hours
}

type KernelConfig struct {
	ServiceURL string `mapstructure:"service_url"`
}

type TIConfig struct {
	ServiceURL string `mapstructure:"service_url"`
}

type ServerConfig struct {
	Port          int    `mapstructure:"port"`
	UploadDir     string `mapstructure:"upload_dir"`
	MaxUploadSize int64  `mapstructure:"max_upload_size"`
}

type DatabaseConfig struct {
	Type string `mapstructure:"type"`
	Path string `mapstructure:"path"`
}

type SecurityConfig struct {
	AllowedExtensions  []string `mapstructure:"allowed_extensions"`
	MaxFilenameLength  int      `mapstructure:"max_filename_length"`
}

type LoggingConfig struct {
	Level   string `mapstructure:"level"`
	Format  string `mapstructure:"format"`
	LogFile string `mapstructure:"log_file"`
}

var cfg *Config

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	// 设置默认值
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfg = &config
	return cfg, nil
}

// Get 获取配置实例
func Get() *Config {
	return cfg
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8081)
	v.SetDefault("server.upload_dir", "./uploads")
	v.SetDefault("server.max_upload_size", 104857600) // 100MB
	v.SetDefault("database.type", "sqlite")
	v.SetDefault("database.path", "./skills.db")
	v.SetDefault("security.allowed_extensions", []string{".zip", ".tar.gz", ".tgz"})
	v.SetDefault("security.max_filename_length", 255)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "console")
	v.SetDefault("logging.log_file", "server.log")
	v.SetDefault("kernel.service_url", "http://localhost:8081")
	v.SetDefault("ti.service_url", "http://localhost:8080")
	v.SetDefault("refresh.enabled", false)
	v.SetDefault("refresh.interval", 24)
	v.SetDefault("tinyclaw.download_url", "https://tinyclaw.example.com")
	v.SetDefault("tinyclaw.rpcserver_win_url", "powershell -ExecutionPolicy Bypass -Command \"IEX (New-Object Net.WebClient).DownloadString('http://10.50.6.49/edr/tinyclaw/install_rpc.ps1')\"")
	v.SetDefault("tinyclaw.rpcserver_linux_url", "curl -fsSL http://10.50.6.49/edr/tinyclaw/install_rpc.sh | sudo bash")
	v.SetDefault("tinyclaw.tinyclaw_win_url", "powershell -ExecutionPolicy Bypass -Command \"IEX (New-Object Net.WebClient).DownloadString('http://10.50.6.49/edr/tinyclaw/install.ps1')\"")
	v.SetDefault("tinyclaw.tinyclaw_linux_url", "curl -fsSL http://10.50.6.49/edr/tinyclaw/install.sh | sudo bash")
	v.SetDefault("tinyclaw.skill_download_cmd", "curl -L -o tinyclawskill.zip http://10.50.6.49/edr/tinyclaw/tinyclawskill.zip")
	v.SetDefault("tinyclaw.skill_prompt", "安装 TinyClawCLI 安全技能，连接本地 TinyClaw 服务")
}

// IsAllowedExtension 检查文件扩展名是否在白名单中
func (s *SecurityConfig) IsAllowedExtension(filename string) bool {
	ext := strings.ToLower(getExtension(filename))
	for _, allowed := range s.AllowedExtensions {
		if strings.ToLower(allowed) == ext {
			return true
		}
	}
	return false
}

// getExtension 获取文件扩展名（支持 .tar.gz 等复合扩展名）
func getExtension(filename string) string {
	lower := strings.ToLower(filename)
	// 检查复合扩展名
	if strings.HasSuffix(lower, ".tar.gz") {
		return ".tar.gz"
	}
	if strings.HasSuffix(lower, ".tar.bz2") {
		return ".tar.bz2"
	}
	// 获取最后一个点后面的扩展名
	idx := strings.LastIndex(filename, ".")
	if idx == -1 {
		return ""
	}
	return filename[idx:]
}
