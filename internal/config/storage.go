package config

// StorageConfig holds storage/persistence configuration
type StorageConfig struct {
	Backend   string `env:"STORAGE_BACKEND" yaml:"backend" default:"local"`      // "local" or "s3"
	LocalDir  string `env:"STORAGE_LOCAL_DIR" yaml:"local_dir" default:"./data"` // Base directory for local storage
	S3Bucket  string `env:"STORAGE_S3_BUCKET" yaml:"s3_bucket"`                  // S3 bucket name
	S3Prefix  string `env:"STORAGE_S3_PREFIX" yaml:"s3_prefix"`                  // S3 object key prefix (optional)
	S3Region  string `env:"STORAGE_S3_REGION" yaml:"s3_region"`                  // AWS region
	S3Profile string `env:"STORAGE_S3_PROFILE" yaml:"s3_profile"`                // AWS profile name (optional)
}
