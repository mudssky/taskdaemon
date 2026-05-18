package scheduler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
)

const (
	// WarningSecondLevelCron 表示 cron 表达式启用了秒字段。
	WarningSecondLevelCron = "second_level_cron"
	// WarningHighFrequencyCron 表示 cron 表达式频率较高，需要用户显式确认。
	WarningHighFrequencyCron = "high_frequency_cron"
)

var (
	// ErrCronWarningsNeedConfirmation 表示 cron 校验只有软警告，但调用方尚未确认。
	ErrCronWarningsNeedConfirmation = errors.New("cron warnings need confirmation")
)

// CronValidationInput 是 cron 校验请求。
type CronValidationInput struct {
	Expression      string
	Timezone        string
	ConfirmWarnings bool
}

// CronValidationResult 是 cron 校验结果。
type CronValidationResult struct {
	Warnings    []string
	WithSeconds bool
}

// ValidateCron 校验 cron 表达式和 timezone，并返回软警告。
//
// 参数:
//   - input: cron 表达式、timezone 和软警告确认标记。
//
// 返回值:
//   - CronValidationResult: 是否包含秒字段和软警告列表。
//   - error: cron/timezone 硬错误，或软警告未确认时返回错误。
func ValidateCron(input CronValidationInput) (CronValidationResult, error) {
	expression := strings.TrimSpace(input.Expression)
	if expression == "" {
		return CronValidationResult{}, fmt.Errorf("cron expression is required")
	}
	location, err := time.LoadLocation(normalizeTimezone(input.Timezone))
	if err != nil {
		return CronValidationResult{}, fmt.Errorf("load cron timezone: %w", err)
	}
	fields := strings.Fields(expression)
	withSeconds := len(fields) == 6
	if len(fields) != 5 && len(fields) != 6 {
		return CronValidationResult{}, fmt.Errorf("cron field count must be 5 or 6")
	}

	cron := gocron.NewDefaultCron(withSeconds)
	if err := cron.IsValid(expression, location, time.Now().In(location)); err != nil {
		return CronValidationResult{}, fmt.Errorf("validate cron: %w", err)
	}

	result := CronValidationResult{WithSeconds: withSeconds}
	if withSeconds {
		result.Warnings = append(result.Warnings, WarningSecondLevelCron)
	}
	if isHighFrequency(fields, withSeconds) {
		result.Warnings = append(result.Warnings, WarningHighFrequencyCron)
	}
	if len(result.Warnings) > 0 && !input.ConfirmWarnings {
		return result, ErrCronWarningsNeedConfirmation
	}
	return result, nil
}

// normalizeTimezone 返回默认 timezone。
//
// 参数:
//   - timezone: 调用方提供的 timezone。
//
// 返回值:
//   - string: 标准 timezone 字符串。
func normalizeTimezone(timezone string) string {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		return "Local"
	}
	return timezone
}

// isHighFrequency 判断 cron 是否可能高频执行。
//
// 参数:
//   - fields: cron 字段。
//   - withSeconds: 是否包含秒字段。
//
// 返回值:
//   - bool: true 表示需要软警告。
func isHighFrequency(fields []string, withSeconds bool) bool {
	if withSeconds && strings.HasPrefix(fields[0], "*/") {
		n, err := strconv.Atoi(strings.TrimPrefix(fields[0], "*/"))
		return err == nil && n > 0 && n < 60
	}
	return false
}
