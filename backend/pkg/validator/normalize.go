package validator

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
)

var courseCodeRegex = regexp.MustCompile(`^[0-9]{6}$`)

// NormalizeOptionalString trims leading/trailing whitespace and returns nil if the resulting string is empty.
func NormalizeOptionalString(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// NormalizeRequiredString trims leading/trailing whitespace from a value string.
func NormalizeRequiredString(s string) string {
	return strings.TrimSpace(s)
}

// ValidateCourseInput validates course fields according to docs/api-contract.md.
func ValidateCourseInput(courseCode, courseName *string) map[string]string {
	details := make(map[string]string)

	if courseCode == nil && courseName == nil {
		details["courseName"] = "กรุณาระบุรหัสวิชาหรือชื่อวิชาอย่างน้อย 1 อย่าง"
	}

	if courseCode != nil && !courseCodeRegex.MatchString(*courseCode) {
		details["courseCode"] = "รหัสวิชาต้องเป็นตัวเลข 6 หลักเท่านั้น (เช่น 001101)"
	}

	if len(details) == 0 {
		return nil
	}
	return details
}

// ParseUintParam parses a positive uint64 path parameter.
func ParseUintParam(c *fiber.Ctx, name string) (uint64, error) {
	raw := strings.TrimSpace(c.Params(name))
	val, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || val == 0 {
		return 0, errors.New("invalid path parameter: " + name)
	}
	return val, nil
}

// ParsePagination extracts page and limit query parameters with safe defaults and bounds.
func ParsePagination(c *fiber.Ctx, defaultLimit, maxLimit int) (page int, limit int, offset int) {
	page = c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}

	limit = c.QueryInt("limit", defaultLimit)
	if limit < 1 {
		limit = defaultLimit
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}

	offset = (page - 1) * limit
	return page, limit, offset
}

// IsUniqueViolation checks if a database error is a PostgreSQL 23505 unique_violation.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return strings.Contains(err.Error(), "SQLSTATE 23505") || strings.Contains(err.Error(), "duplicate key value")
}

// IsForeignKeyRestriction checks if a database error is a PostgreSQL 23001 / 23503 foreign key restriction.
func IsForeignKeyRestriction(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23001" || pgErr.Code == "23503"
	}
	return strings.Contains(err.Error(), "SQLSTATE 23001") || strings.Contains(err.Error(), "SQLSTATE 23503")
}

