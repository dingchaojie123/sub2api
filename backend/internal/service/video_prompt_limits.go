package service

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func validateVideoPromptFieldLength(provider, field, value string, maxRunes int) error {
	if utf8.RuneCountInString(strings.TrimSpace(value)) > maxRunes {
		return fmt.Errorf("%s %s must be at most %d characters", provider, field, maxRunes)
	}
	return nil
}
