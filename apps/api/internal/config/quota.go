package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	quotaTimezoneEnv     = "LABKIT_QUOTA_TIMEZONE"
	defaultQuotaTimezone = "Asia/Shanghai"
)

func ResolveQuotaLocationFromEnv() (*time.Location, error) {
	name := strings.TrimSpace(os.Getenv(quotaTimezoneEnv))
	if name == "" {
		name = defaultQuotaTimezone
	}

	location, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", quotaTimezoneEnv, err)
	}
	return location, nil
}
