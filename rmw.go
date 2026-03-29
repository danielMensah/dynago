package dynago

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// RMWOption configures a ReadModifyWrite call.
type RMWOption func(*rmwConfig)

type rmwConfig struct {
	versionAttr string
	maxRetries  int
}

// OptimisticLock enables optimistic locking using the given attribute as a
// version counter. The attribute must be a numeric field on the struct.
func OptimisticLock(versionAttr string) RMWOption {
	return func(c *rmwConfig) {
		c.versionAttr = versionAttr
	}
}

// MaxRetries sets the maximum number of retry attempts on condition check
// failure. Default is 3.
func MaxRetries(n int) RMWOption {
	return func(c *rmwConfig) {
		c.maxRetries = n
	}
}

// ReadModifyWrite reads an item, applies fn to modify it, and writes it back.
// With OptimisticLock, it uses a condition expression to detect concurrent
// modifications and retries automatically.
func ReadModifyWrite[T any](ctx context.Context, t *Table, key KeyValue, fn func(*T) error, opts ...RMWOption) error {
	cfg := rmwConfig{maxRetries: 3}
	for _, o := range opts {
		o(&cfg)
	}

	for attempt := 0; attempt <= cfg.maxRetries; attempt++ {
		// Read
		item, err := Get[T](ctx, t, key)
		if err != nil {
			return err
		}

		// Modify
		if err := fn(&item); err != nil {
			return err
		}

		// Write
		var putOpts []PutOption

		if cfg.versionAttr != "" {
			oldVersion, err := getVersionField(&item, cfg.versionAttr)
			if err != nil {
				return err
			}

			// Add condition: version must match
			putOpts = append(putOpts, PutCondition(cfg.versionAttr+" = ?", oldVersion))

			// Increment the version
			if err := setVersionField(&item, cfg.versionAttr, oldVersion+1); err != nil {
				return err
			}
		}

		err = t.Put(ctx, item, putOpts...)
		if err != nil {
			if errors.Is(err, ErrConditionFailed) && cfg.versionAttr != "" && attempt < cfg.maxRetries {
				continue // retry
			}
			return err
		}
		return nil
	}

	return newError(ErrConditionFailed, fmt.Sprintf("dynago.ReadModifyWrite: failed after %d retries", cfg.maxRetries+1))
}

// getVersionField reads a numeric version field from a struct using reflection.
func getVersionField(item any, attr string) (int64, error) {
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return 0, newError(ErrValidation, "dynago.ReadModifyWrite: item must be a struct")
	}

	// Find the field by dynamo tag or field name.
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("dynamo")
		name := tag
		if idx := indexOf(name, ','); idx >= 0 {
			name = name[:idx]
		}
		if name == "" {
			name = f.Name
		}
		if name == attr {
			fv := v.Field(i)
			switch fv.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return fv.Int(), nil
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return int64(fv.Uint()), nil
			case reflect.Float32, reflect.Float64:
				return int64(fv.Float()), nil
			default:
				return 0, newError(ErrValidation, fmt.Sprintf("dynago.ReadModifyWrite: version attribute %q must be numeric, got %s", attr, fv.Kind()))
			}
		}
	}

	return 0, newError(ErrValidation, fmt.Sprintf("dynago.ReadModifyWrite: version attribute %q not found", attr))
}

// setVersionField sets a numeric version field on a struct using reflection.
func setVersionField(item any, attr string, val int64) error {
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("dynamo")
		name := tag
		if idx := indexOf(name, ','); idx >= 0 {
			name = name[:idx]
		}
		if name == "" {
			name = f.Name
		}
		if name == attr {
			fv := v.Field(i)
			switch fv.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				fv.SetInt(val)
				return nil
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				fv.SetUint(uint64(val))
				return nil
			case reflect.Float32, reflect.Float64:
				fv.SetFloat(float64(val))
				return nil
			default:
				return newError(ErrValidation, fmt.Sprintf("dynago.ReadModifyWrite: version attribute %q must be numeric", attr))
			}
		}
	}
	return newError(ErrValidation, fmt.Sprintf("dynago.ReadModifyWrite: version attribute %q not found", attr))
}

// indexOf returns the index of sep in s, or -1.
func indexOf(s string, sep byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return i
		}
	}
	return -1
}

