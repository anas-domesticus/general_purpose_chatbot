# Config

Type-safe configuration loading from YAML files and environment variables with validation.

## Features

- Load configuration from YAML files and/or environment variables
- Struct tags for `env`, `yaml`, `default`, and `required` fields
- Environment variable interpolation in YAML values (`${VAR_NAME}`)
- Automatic validation via the `Validator` interface
- Precedence: defaults → YAML → environment variables

## Usage

```go
type AppConfig struct {
    config.CommonConfig `yaml:",inline"`
    APIKey string `env:"API_KEY" yaml:"api_key" required:"true"`
    Debug  bool   `env:"DEBUG" yaml:"debug" default:"false"`
}

func (c AppConfig) Validate() error {
    return c.CommonConfig.Validate()
}

var cfg AppConfig
err := config.GetConfig(&cfg, "config.yaml", true)
```

## Struct Tags

| Tag | Purpose |
|-----|---------|
| `env` | Environment variable name |
| `yaml` | YAML field name |
| `default` | Fallback value if not set |
| `required` | Must be provided (error if missing and no default) |