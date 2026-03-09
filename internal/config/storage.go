package config

import "time"

// StorageConfig holds storage/persistence configuration
type StorageConfig struct {
	Backend   string `env:"STORAGE_BACKEND" yaml:"backend" default:"local"`      // "local", "s3", or "git"
	LocalDir  string `env:"STORAGE_LOCAL_DIR" yaml:"local_dir" default:"./data"` // Base directory for local storage
	S3Bucket  string `env:"STORAGE_S3_BUCKET" yaml:"s3_bucket"`                  // S3 bucket name
	S3Prefix  string `env:"STORAGE_S3_PREFIX" yaml:"s3_prefix"`                  // S3 object key prefix (optional)
	S3Region  string `env:"STORAGE_S3_REGION" yaml:"s3_region"`                  // AWS region
	S3Profile string `env:"STORAGE_S3_PROFILE" yaml:"s3_profile"`                // AWS profile name (optional)

	// Git backend configuration
	GitPath           string        `env:"STORAGE_GIT_PATH" yaml:"git_path"`                         // Path to git repository
	GitRemoteURL      string        `env:"STORAGE_GIT_REMOTE_URL" yaml:"git_remote_url"`             // Remote repository URL
	GitBranch         string        `env:"STORAGE_GIT_BRANCH" yaml:"git_branch" default:"main"`      // Branch name
	GitAuthorName     string        `env:"STORAGE_GIT_AUTHOR_NAME" yaml:"git_author_name"`           // Commit author name
	GitAuthorEmail    string        `env:"STORAGE_GIT_AUTHOR_EMAIL" yaml:"git_author_email"`         // Commit author email
	GitPushDebounce   time.Duration `env:"STORAGE_GIT_PUSH_DEBOUNCE" yaml:"git_push_debounce"`       // Push debounce delay (default: 5s)
	GitAuthUsername   string        `env:"STORAGE_GIT_AUTH_USERNAME" yaml:"git_auth_username"`       // HTTPS username
	GitAuthPassword   string        `env:"STORAGE_GIT_AUTH_PASSWORD" yaml:"git_auth_password"`       // HTTPS password/token
	GitSSHKeyPath     string        `env:"STORAGE_GIT_SSH_KEY_PATH" yaml:"git_ssh_key_path"`         // SSH private key path
	GitSSHKeyPassword string        `env:"STORAGE_GIT_SSH_KEY_PASSWORD" yaml:"git_ssh_key_password"` // SSH key passphrase
}
