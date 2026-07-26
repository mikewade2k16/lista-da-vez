package socialpublishing

import (
	"encoding/hex"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxCaptionRunes   = 2200
	maxAltTextRunes   = 1000
	maxMediaURLBytes  = 4096
	maxSourceRefBytes = 256
)

func (s *Service) normalizeCreate(input CreatePostInput) (CreatePostInput, error) {
	input.Caption = strings.TrimSpace(input.Caption)
	input.MediaURL = strings.TrimSpace(input.MediaURL)
	input.MediaType = strings.ToLower(strings.TrimSpace(input.MediaType))
	input.AltText = strings.TrimSpace(input.AltText)
	input.Timezone = strings.TrimSpace(input.Timezone)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceRef = strings.TrimSpace(input.SourceRef)
	if input.Status == "" {
		input.Status = PostStatusDraft
	}
	if input.Status != PostStatusDraft && input.Status != PostStatusScheduled {
		return CreatePostInput{}, ErrInvalidState
	}
	if input.MediaType == "" {
		input.MediaType = "image"
	}
	if input.MediaType != "image" {
		return CreatePostInput{}, ErrInvalidInput
	}
	if input.Timezone == "" {
		input.Timezone = DefaultTimezone
	}
	if err := validateContent(input.Caption, input.MediaURL, input.AltText); err != nil {
		return CreatePostInput{}, err
	}
	if input.SourceType == "" {
		input.SourceType = "manual"
	}
	if !validSourceType(input.SourceType) || len(input.SourceRef) > maxSourceRefBytes {
		return CreatePostInput{}, ErrInvalidInput
	}
	if input.Status == PostStatusDraft {
		input.ScheduledFor = nil
		if err := validateTimezone(input.Timezone); err != nil {
			return CreatePostInput{}, err
		}
		return input, nil
	}
	if input.ScheduledFor == nil {
		return CreatePostInput{}, ErrInvalidInput
	}
	when, timezone, err := s.validateSchedule(*input.ScheduledFor, input.Timezone)
	if err != nil {
		return CreatePostInput{}, err
	}
	input.ScheduledFor = &when
	input.Timezone = timezone
	return input, nil
}

func (s *Service) validateSchedule(
	when time.Time,
	timezone string,
) (time.Time, string, error) {
	timezone = strings.TrimSpace(timezone)
	if err := validateTimezone(timezone); err != nil {
		return time.Time{}, "", err
	}
	if !when.After(s.now().UTC()) {
		return time.Time{}, "", ErrScheduleInPast
	}
	return when.UTC(), timezone, nil
}

func validateContent(caption, mediaURL, altText string) error {
	if utf8.RuneCountInString(caption) > maxCaptionRunes ||
		utf8.RuneCountInString(altText) > maxAltTextRunes {
		return ErrInvalidInput
	}
	if len(mediaURL) == 0 || len(mediaURL) > maxMediaURLBytes {
		return ErrInvalidMediaURL
	}
	parsed, err := url.Parse(mediaURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.Fragment != "" {
		return ErrInvalidMediaURL
	}
	return nil
}

func validateTimezone(value string) error {
	if strings.TrimSpace(value) == "" {
		return ErrInvalidTimezone
	}
	if _, err := time.LoadLocation(value); err != nil {
		return ErrInvalidTimezone
	}
	return nil
}

func validSourceType(value string) bool {
	switch value {
	case "manual", "calendar", "crow_assistant":
		return true
	default:
		return false
	}
}

func validPostStatus(value PostStatus) bool {
	switch value {
	case PostStatusDraft, PostStatusScheduled, PostStatusPublishing,
		PostStatusPublished, PostStatusFailed, PostStatusCancelled:
		return true
	default:
		return false
	}
}

func normalizePostStatuses(filter ListPostsFilter) ([]PostStatus, error) {
	values := make([]PostStatus, 0, len(filter.Statuses)+1)
	if filter.Status != "" {
		values = append(values, filter.Status)
	}
	values = append(values, filter.Statuses...)
	seen := make(map[PostStatus]struct{}, len(values))
	normalized := make([]PostStatus, 0, len(values))
	for _, value := range values {
		value = PostStatus(strings.TrimSpace(string(value)))
		if !validPostStatus(value) {
			return nil, ErrInvalidInput
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func normalizePostListOrder(value PostListOrder) (PostListOrder, error) {
	value = PostListOrder(strings.TrimSpace(string(value)))
	if value == "" {
		return PostListOrderCreated, nil
	}
	switch value {
	case PostListOrderCreated, PostListOrderScheduled:
		return value, nil
	default:
		return "", ErrInvalidInput
	}
}

func normalizeAnalyticsPostIDs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if !validUUID(value) {
			return nil, ErrInvalidInput
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
		if len(normalized) > 100 {
			return nil, ErrInvalidInput
		}
	}
	return normalized, nil
}

func validUUID(value string) bool {
	if len(value) != 36 ||
		value[8] != '-' ||
		value[13] != '-' ||
		value[18] != '-' ||
		value[23] != '-' {
		return false
	}
	compact := strings.ReplaceAll(value, "-", "")
	_, err := hex.DecodeString(compact)
	return err == nil
}
