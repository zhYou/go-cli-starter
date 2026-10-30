// Package diagnostics 管理每次运行的私有诊断文件、轮转和保留策略。
package diagnostics

// Config 描述日志和本地 Trace 共用的文件策略，0 禁用对应删除限制。
type Config struct {
	MaxSizeMiB      int `mapstructure:"max_size_mib" validate:"gt=0,lte=1048576"`
	MaxBackups      int `mapstructure:"max_backups" validate:"gte=0"`
	RetentionDays   int `mapstructure:"retention_days" validate:"gte=0,lte=36500"`
	MaxTotalSizeMiB int `mapstructure:"max_total_size_mib" validate:"gte=0,lte=1048576"`
}

// Defaults 返回独立的默认文件策略。
func Defaults() Config {
	return Config{MaxSizeMiB: 10, MaxBackups: 5, RetentionDays: 14, MaxTotalSizeMiB: 200}
}
