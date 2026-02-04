package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"gopkg.in/yaml.v3"
)

var durationType = reflect.TypeOf(time.Duration(0))

// Validator interface allows config structs to implement custom validation logic.
// If a config struct implements this interface, validation will be automatically
// called after loading configuration from files and environment variables.
type Validator interface {
	Validate() error
}

func processFields(val reflect.Value, typeOfT reflect.Type) (map[string]bool, error) {
	setFields := make(map[string]bool)

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typeOfT.Field(i)

		// Handle all nested structs recursively
		if field.Kind() == reflect.Struct {
			embeddedSetFields, err := processFields(field, fieldType.Type)
			if err != nil {
				return nil, err
			}
			// Merge embedded set fields
			for k, v := range embeddedSetFields {
				setFields[k] = v
			}
			continue
		}

		// Process regular fields with env tags
		{
			tag := fieldType.Tag.Get("env")
			if tag != "" {
				envVal := os.Getenv(tag)
				if envVal == "" {
					continue
				}

				// Mark this field as set from environment using struct type + field name to avoid collisions
				fieldKey := typeOfT.Name() + "." + fieldType.Name
				setFields[fieldKey] = true

				// Set the value to the field based on its type
				// Check for time.Duration first (it's an int64 underneath)
				if field.Type() == durationType {
					duration, err := time.ParseDuration(envVal)
					if err != nil {
						return nil, fmt.Errorf("failed to convert %s to duration: %v", envVal, err)
					}
					field.SetInt(int64(duration))
					continue
				}

				switch field.Kind() {
				case reflect.String:
					field.SetString(envVal)
				case reflect.Int, reflect.Int64:
					intVal, err := strconv.ParseInt(envVal, 10, 64)
					if err != nil {
						return nil, fmt.Errorf("failed to convert %s to int: %v", envVal, err)
					}
					field.SetInt(intVal)
				case reflect.Float64:
					floatVal, err := strconv.ParseFloat(envVal, 64)
					if err != nil {
						return nil, fmt.Errorf("failed to convert %s to float64: %v", envVal, err)
					}
					field.SetFloat(floatVal)
				case reflect.Float32:
					floatVal, err := strconv.ParseFloat(envVal, 32)
					if err != nil {
						return nil, fmt.Errorf("failed to convert %s to float32: %v", envVal, err)
					}
					field.SetFloat(floatVal)
				case reflect.Bool:
					boolVal, err := strconv.ParseBool(envVal)
					if err != nil {
						return nil, fmt.Errorf("failed to convert %s to bool: %v", envVal, err)
					}
					field.SetBool(boolVal)
				case reflect.Slice:
					// Handle string slices (comma-separated values)
					if field.Type().Elem().Kind() == reflect.String {
						values := strings.Split(envVal, ",")
						slice := reflect.MakeSlice(field.Type(), len(values), len(values))
						for i, v := range values {
							slice.Index(i).SetString(strings.TrimSpace(v))
						}
						field.Set(slice)
					} else {
						return nil, fmt.Errorf("unsupported slice type %s", field.Type())
					}
				default:
					return nil, fmt.Errorf("unsupported kind %s", field.Kind())
				}
			}
		}
	}
	return setFields, nil
}

func checkRequiredAndDefaults(val reflect.Value, typeOfT reflect.Type, setFields map[string]bool) error {
	var result error
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typeOfT.Field(i)

		// Handle all nested structs recursively
		if field.Kind() == reflect.Struct {
			if err := checkRequiredAndDefaults(field, fieldType.Type, setFields); err != nil {
				result = multierror.Append(result, err)
			}
			continue
		}

		// Process regular fields
		{
			fieldRequired := false
			requiredTag := fieldType.Tag.Get("required")
			if strings.ToLower(requiredTag) == "true" || strings.ToLower(requiredTag) == "1" {
				fieldRequired = true
			}
			defaultTag := fieldType.Tag.Get("default")
			if fieldRequired && defaultTag != "" { // ignoring required tag if default is set
				fieldRequired = false
			}

			if field.IsZero() && fieldRequired {
				envTag := fieldType.Tag.Get("env")
				yamlTag := fieldType.Tag.Get("yaml")
				result = multierror.Append(result, fmt.Errorf("required field env:%s / yaml:%s is missing", envTag, yamlTag))
				continue
			}

			// Only apply defaults if the field wasn't explicitly set from environment
			fieldKey := typeOfT.Name() + "." + fieldType.Name
			if field.IsZero() && defaultTag != "" && !setFields[fieldKey] {
				// Check for time.Duration first (it's an int64 underneath)
				if field.Type() == durationType {
					duration, err := time.ParseDuration(defaultTag)
					if err != nil {
						result = multierror.Append(result, fmt.Errorf("failed to convert %s to duration: %v", defaultTag, err))
					} else {
						field.SetInt(int64(duration))
					}
					continue
				}

				switch field.Kind() {
				case reflect.String:
					field.SetString(defaultTag)
				case reflect.Int, reflect.Int64:
					intVal, err := strconv.ParseInt(defaultTag, 10, 64)
					if err != nil {
						result = multierror.Append(result, fmt.Errorf("failed to convert %s to int: %v", defaultTag, err))
					}
					field.SetInt(intVal)
				case reflect.Float64:
					floatVal, err := strconv.ParseFloat(defaultTag, 64)
					if err != nil {
						result = multierror.Append(result, fmt.Errorf("failed to convert %s to float64: %v", defaultTag, err))
					}
					field.SetFloat(floatVal)
				case reflect.Float32:
					floatVal, err := strconv.ParseFloat(defaultTag, 32)
					if err != nil {
						result = multierror.Append(result, fmt.Errorf("failed to convert %s to float32: %v", defaultTag, err))
					}
					field.SetFloat(floatVal)
				case reflect.Bool:
					boolVal, err := strconv.ParseBool(defaultTag)
					if err != nil {
						result = multierror.Append(result, fmt.Errorf("failed to convert %s to bool: %v", defaultTag, err))
					}
					field.SetBool(boolVal)
				case reflect.Slice:
					// Handle string slices (comma-separated values)
					if field.Type().Elem().Kind() == reflect.String {
						values := strings.Split(defaultTag, ",")
						slice := reflect.MakeSlice(field.Type(), len(values), len(values))
						for i, v := range values {
							slice.Index(i).SetString(strings.TrimSpace(v))
						}
						field.Set(slice)
					} else {
						result = multierror.Append(result, fmt.Errorf("unsupported slice type %s for default", field.Type()))
					}
				default:
					result = multierror.Append(result, fmt.Errorf("unsupported kind %s", field.Kind()))
				}
			}
		}
	}
	return result
}

// GetConfigFromEnvVars loads configuration from environment variables only.
// It processes struct tags: env, default, required.
// Example usage:
//
//	var cfg MyConfig
//	err := GetConfigFromEnvVars(&cfg)
func GetConfigFromEnvVars[T any](dest *T) error {
	val := reflect.ValueOf(dest).Elem()
	typeOfT := val.Type()
	setFields, err := processFields(val, typeOfT)
	if err != nil {
		return err
	}
	err = checkRequiredAndDefaults(val, typeOfT, setFields)
	if err != nil {
		*dest = reflect.New(reflect.TypeOf(dest).Elem()).Elem().Interface().(T) // resets config to empty
		return err
	}

	// Run custom validation if the type implements Validator
	if validator, ok := any(*dest).(Validator); ok {
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	return nil
}

// GetConfig loads configuration from YAML file first, then overlays environment variables.
// If filepath is empty, only environment variables are used.
// If allowFileErrors is true, file read/parse errors fallback to env vars only.
// Example usage:
//
//	var cfg MyConfig
//	err := GetConfig(&cfg, "config.yaml", true)
func GetConfig[T any](dest *T, filepath string, allowFileErrors bool) error {
	if filepath == "" {
		return GetConfigFromEnvVars(dest)
	}
	data, err := os.ReadFile(filepath)
	if err != nil {
		if allowFileErrors {
			return GetConfigFromEnvVars(dest)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}
	if err := yaml.Unmarshal(data, dest); err != nil {
		if allowFileErrors {
			return GetConfigFromEnvVars(dest)
		}
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}
	err = GetConfigFromEnvVars(dest)
	if err != nil {
		return err
	}

	// Run custom validation if the type implements Validator
	if validator, ok := any(*dest).(Validator); ok {
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	return nil
}
