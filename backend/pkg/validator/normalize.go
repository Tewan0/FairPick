package validator

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
)

var courseCodeRegex = regexp.MustCompile(`^[A-Za-z0-9]{1,10}$`)

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

// FormatSearchLike normalizes a search query, caps its length to prevent DoS,
// and escapes SQL LIKE/ILIKE wildcard characters (%, _, \) per security best practices.
func FormatSearchLike(search string) string {
	trimmed := strings.TrimSpace(search)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) > 100 {
		trimmed = string(runes[:100])
	}
	escaped := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	).Replace(trimmed)
	return "%" + escaped + "%"
}

// ValidateCourseInput validates course fields according to docs/api-contract.md.
func ValidateCourseInput(courseCode, courseName *string) map[string]string {
	return ValidateCourseFields(courseCode, courseName, nil, nil)
}

// ValidateCourseFields validates courseCode (alphanumeric max 10 chars) and length bounds on course fields.
func ValidateCourseFields(courseCode, courseName, section, semester *string) map[string]string {
	details := make(map[string]string)

	if courseCode == nil && courseName == nil {
		details["courseName"] = "กรุณาระบุรหัสวิชาหรือชื่อวิชาอย่างน้อย 1 อย่าง"
	}

	if courseCode != nil && !courseCodeRegex.MatchString(*courseCode) {
		details["courseCode"] = "รหัสวิชาต้องเป็นตัวอักษรภาษาอังกฤษหรือตัวเลข ไม่เกิน 10 ตัวอักษร (เช่น CS101, 001101)"
	}

	if courseName != nil && len([]rune(*courseName)) > 255 {
		details["courseName"] = "ชื่อวิชาต้องมีความยาวไม่เกิน 255 ตัวอักษร"
	}

	if section != nil && len([]rune(*section)) > 50 {
		details["section"] = "กลุ่มเรียน (Section) ต้องมีความยาวไม่เกิน 50 ตัวอักษร"
	}

	if semester != nil && len([]rune(*semester)) > 50 {
		details["semester"] = "ภาคการศึกษา (Semester) ต้องมีความยาวไม่เกิน 50 ตัวอักษร"
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

