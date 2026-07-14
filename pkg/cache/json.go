package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"golang.org/x/sync/singleflight"
)

var rememberJSONGroup singleflight.Group

func SetJSON[T any](ctx context.Context, store Store, key string, value T, ttl time.Duration) error {
	if isNilStore(store) {
		return fmt.Errorf("cache store is required")
	}
	ctx = normalizeContext(ctx)
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode cache JSON: %w", err)
	}
	if err := store.Set(ctx, key, encoded, ttl); err != nil {
		return fmt.Errorf("set cache JSON: %w", err)
	}
	return nil
}

func GetJSON[T any](ctx context.Context, store Store, key string) (T, bool, error) {
	var zero T
	if isNilStore(store) {
		return zero, false, fmt.Errorf("cache store is required")
	}
	ctx = normalizeContext(ctx)
	encoded, found, err := store.Get(ctx, key)
	if err != nil {
		return zero, false, fmt.Errorf("get cache JSON: %w", err)
	}
	if !found {
		return zero, false, nil
	}
	var value T
	if err := json.Unmarshal(encoded, &value); err != nil {
		return zero, false, fmt.Errorf("decode cache JSON: %w", err)
	}
	return value, true, nil
}

func RememberJSON[T any](ctx context.Context, store Store, key string, ttl time.Duration, loader func(context.Context) (T, error)) (T, error) {
	var zero T
	if isNilStore(store) {
		return zero, fmt.Errorf("cache store is required")
	}
	if loader == nil {
		return zero, fmt.Errorf("cache loader is required")
	}
	ctx = normalizeContext(ctx)
	value, found, err := GetJSON[T](ctx, store, key)
	if err != nil {
		return zero, err
	}
	if found {
		return value, nil
	}

	resultCh := rememberJSONGroup.DoChan(rememberJSONKey[T](store, key), func() (any, error) {
		value, found, err := GetJSON[T](ctx, store, key)
		if err != nil {
			return nil, err
		}
		if found {
			return value, nil
		}
		value, err = loader(ctx)
		if err != nil {
			return nil, err
		}
		if err := SetJSON(ctx, store, key, value, ttl); err != nil {
			return nil, err
		}
		return value, nil
	})

	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return zero, result.Err
		}
		value, ok := result.Val.(T)
		if !ok {
			return zero, fmt.Errorf("cache singleflight returned unexpected type %T", result.Val)
		}
		return value, nil
	}
}

func rememberJSONKey[T any](store Store, key string) string {
	storeValue := reflect.ValueOf(store)
	identity := fmt.Sprintf("%T", store)
	if storeValue.IsValid() && (storeValue.Kind() == reflect.Pointer || storeValue.Kind() == reflect.Map || storeValue.Kind() == reflect.Slice || storeValue.Kind() == reflect.Func || storeValue.Kind() == reflect.Chan) {
		identity += ":" + strconv.FormatUint(uint64(storeValue.Pointer()), 16)
	}
	valueType := reflect.TypeOf((*T)(nil)).Elem()
	return identity + ":" + valueType.String() + ":" + key
}

func isNilStore(store Store) bool {
	if store == nil {
		return true
	}
	value := reflect.ValueOf(store)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
