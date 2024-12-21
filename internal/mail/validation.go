package mail

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	minEmailLength  = 3   // a@b
	maxEmailLength  = 254 // RFC 5321
	maxLocalLength  = 64  // RFC 5321
	maxDomainLength = 255 // RFC 5321
)

type ValidationError string

func (e ValidationError) Error() string {
	return string(e)
}

const (
	ErrEmailEmpty    = ValidationError("email cannot be empty")
	ErrEmailTooShort = ValidationError("email is too short")
	ErrEmailTooLong  = ValidationError("email exceeds maximum length")
	ErrLocalTooLong  = ValidationError("local part exceeds maximum length")
	ErrDomainTooLong = ValidationError("domain part exceeds maximum length")
	ErrInvalidFormat = ValidationError("email format is invalid")
	ErrMultipleAt    = ValidationError("email contains multiple @ symbols")
	ErrInvalidDomain = ValidationError("domain is invalid")
)

func ValidateEmail(email string) error {
	// Trim spaces
	email = strings.TrimSpace(email)

	// Check if empty
	if email == "" {
		return ErrEmailEmpty
	}

	// Check total length
	emailLength := utf8.RuneCountInString(email)
	if emailLength < minEmailLength {
		return ErrEmailTooShort
	}
	if emailLength > maxEmailLength {
		return ErrEmailTooLong
	}

	// Split into local and domain parts
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ErrMultipleAt
	}

	localPart := parts[0]
	domainPart := parts[1]

	// Check local part length
	if utf8.RuneCountInString(localPart) > maxLocalLength {
		return ErrLocalTooLong
	}

	// Check domain length
	if utf8.RuneCountInString(domainPart) > maxDomainLength {
		return ErrDomainTooLong
	}

	// More comprehensive regex pattern
	pattern := `^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`

	matched, err := regexp.MatchString(pattern, email)
	if err != nil {
		return err
	}
	if !matched {
		return ErrInvalidFormat
	}

	// Additional domain validations
	if !validateDomain(domainPart) {
		return ErrInvalidDomain
	}

	return nil
}

func validateDomain(domain string) bool {
	// Domain specific validations
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}

	// Check for valid TLD (at least one dot and last part >= 2 chars)
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}

	tld := parts[len(parts)-1]
	if len(tld) < 2 {
		return false
	}

	// Check for consecutive dots
	if strings.Contains(domain, "..") {
		return false
	}

	// Check each part of the domain
	for _, part := range parts {
		if len(part) == 0 {
			return false
		}

		// Check if part starts or ends with hyphen
		if strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return false
		}
	}

	return true
}
