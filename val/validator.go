package val

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
)

var (
	isValidUsername       = regexp.MustCompile(`^[a-z0-9_]+$`).MatchString
	isValidFullName       = regexp.MustCompile(`^[a-zA-Z\s]+$`).MatchString
	isValidAirortIataCode = regexp.MustCompile(`^[A-Za-z]{3}$`).MatchString
	isValidAirortICaoCode = regexp.MustCompile(`^[A-Za-z]{3}$`).MatchString
)

func ValidateString(value string, minLength int, maxLength int) error {
	n := len(value)
	if n < minLength || n > maxLength {
		return fmt.Errorf("must contain from %d-%d characters", minLength, maxLength)
	}
	return nil
}

// ValidatIataCode: Is to check if the this code has met the requirements
// like it must to be only three letters only
func ValidatIataCode(value string) error {

	if err := ValidateString(UpperCase(value), 3, 3); err != nil {
		return err
	}
	if !isValidAirortIataCode(value) {
		return fmt.Errorf("must contain only three letters no more less then")
	}
	return nil
}

// ValidatICoaCode: Is to check if the the airport Icoa  code has met the requirements
// like it must to be only four letters only
func ValidatICoaCode(value string) error {
	if err := ValidateString(UpperCase(value), 4, 4); err != nil {
		return err
	}
	if !isValidAirortICaoCode(value) {
		return fmt.Errorf("must contain only four letters no more less then")
	}
	return nil
}

func ValidateUsername(value string) error {
	if err := ValidateString(value, 3, 100); err != nil {
		return err
	}
	if !isValidUsername(value) {
		return fmt.Errorf("must contain only lowercase letters, digits, or underscore")
	}
	return nil

}

func UpperCase(value string) string {
	return strings.ToUpper(value)
}

func ValidateFullName(value string) error {
	if err := ValidateString(value, 3, 100); err != nil {
		return err
	}
	if !isValidFullName(value) {
		return fmt.Errorf("must contain only letters or spaces")
	}
	return nil
}

func ValidatePassword(value string) error {
	return ValidateString(value, 6, 100)
}

func ValidateEmail(value string) error {
	if err := ValidateString(value, 3, 200); err != nil {
		return err
	}
	if _, err := mail.ParseAddress(value); err != nil {
		return fmt.Errorf("is not a valid email address")
	}
	return nil
}
