package xdr

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// iso8601Time is a timestamp which supports parsing dates which have a year outside the 0000..9999 range
type iso8601Time struct {
	time.Time
}

// reISO8601 is the regular expression used to parse date strings in the
// ISO 8601 extended format, with or without an expanded year representation.
var reISO8601 = regexp.MustCompile(`^([-+]?\d{4,})-(\d{2})-(\d{2})`)

// MarshalJSON serializes the timestamp to a string
func (t iso8601Time) MarshalJSON() ([]byte, error) {
	ts := t.Format(time.RFC3339)
	if t.Year() > 9999 {
		ts = "+" + ts
	}

	return json.Marshal(ts)
}

// UnmarshalJSON parses a JSON string into a iso8601Time instance.
func (t *iso8601Time) UnmarshalJSON(b []byte) error {
	var s *string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == nil {
		return nil
	}

	text := *s
	m := reISO8601.FindStringSubmatch(text)

	if len(m) != 4 {
		return fmt.Errorf("UnmarshalJSON: cannot parse %s", text)
	}
	// No need to check for errors since the regexp guarantees the matches
	// are valid integers
	year, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	day, _ := strconv.Atoi(m[3])

	ts, err := time.Parse(time.RFC3339, "2006-01-02"+text[len(m[0]):])
	if err != nil {
		return errors.Wrap(err, "Could not extract time")
	}

	t.Time = time.Date(year, time.Month(month), day, ts.Hour(), ts.Minute(), ts.Second(), ts.Nanosecond(), ts.Location())
	return nil
}

func newiso8601Time(epoch int64) *iso8601Time {
	return &iso8601Time{time.Unix(epoch, 0).UTC()}
}

type claimPredicateJSON struct {
	And            *[]claimPredicateJSON `json:"and,omitempty"`
	Or             *[]claimPredicateJSON `json:"or,omitempty"`
	Not            *claimPredicateJSON   `json:"not,omitempty"`
	Unconditional  bool                  `json:"unconditional,omitempty"`
	AbsBefore      *iso8601Time          `json:"abs_before,omitempty"`
	AbsBeforeEpoch *int64                `json:"abs_before_epoch,string,omitempty"`
	RelBefore      *int64                `json:"rel_before,string,omitempty"`
}

func convertPredicatesToXDR(input []claimPredicateJSON) ([]ClaimPredicate, error) {
	parts := make([]ClaimPredicate, len(input))
	for i, pred := range input {
		converted, err := pred.toXDR()
		if err != nil {
			return parts, err
		}
		parts[i] = converted
	}
	return parts, nil
}

func (c claimPredicateJSON) toXDR() (ClaimPredicate, error) {
	var result ClaimPredicate
	var err error

	switch {
	case c.Unconditional:
		result.Type = ClaimPredicateTypeClaimPredicateUnconditional
	case c.RelBefore != nil:
		relBefore := Int64(*c.RelBefore)
		result.Type = ClaimPredicateTypeClaimPredicateBeforeRelativeTime
		result.RelBefore = &relBefore
	case c.AbsBefore != nil:
		absBefore := Int64(c.AbsBefore.UTC().Unix())
		result.Type = ClaimPredicateTypeClaimPredicateBeforeAbsoluteTime
		result.AbsBefore = &absBefore
	case c.Not != nil:
		if inner, innerErr := c.Not.toXDR(); innerErr != nil {
			err = innerErr
		} else {
			result.Type = ClaimPredicateTypeClaimPredicateNot
			result.NotPredicate = new(*ClaimPredicate)
			*result.NotPredicate = &inner
		}
	case c.And != nil:
		if inner, innerErr := convertPredicatesToXDR(*c.And); innerErr != nil {
			err = innerErr
		} else {
			result.Type = ClaimPredicateTypeClaimPredicateAnd
			result.AndPredicates = &inner
		}
	case c.Or != nil:
		if inner, innerErr := convertPredicatesToXDR(*c.Or); innerErr != nil {
			err = innerErr
		} else {
			result.Type = ClaimPredicateTypeClaimPredicateOr
			result.OrPredicates = &inner
		}
	}

	return result, err
}

func convertPredicatesToJSON(input []ClaimPredicate) ([]claimPredicateJSON, error) {
	parts := make([]claimPredicateJSON, len(input))
	for i, pred := range input {
		converted, err := pred.toJSON()
		if err != nil {
			return parts, err
		}
		parts[i] = converted
	}
	return parts, nil
}

func (c ClaimPredicate) toJSON() (claimPredicateJSON, error) {
	var payload claimPredicateJSON
	var err error

	switch c.Type {
	case ClaimPredicateTypeClaimPredicateAnd:
		payload.And = new([]claimPredicateJSON)
		*payload.And, err = convertPredicatesToJSON(c.MustAndPredicates())
	case ClaimPredicateTypeClaimPredicateOr:
		payload.Or = new([]claimPredicateJSON)
		*payload.Or, err = convertPredicatesToJSON(c.MustOrPredicates())
	case ClaimPredicateTypeClaimPredicateUnconditional:
		payload.Unconditional = true
	case ClaimPredicateTypeClaimPredicateNot:
		payload.Not = new(claimPredicateJSON)
		*payload.Not, err = c.MustNotPredicate().toJSON()
	case ClaimPredicateTypeClaimPredicateBeforeAbsoluteTime:
		absBeforeEpoch := int64(c.MustAbsBefore())
		payload.AbsBefore = newiso8601Time(absBeforeEpoch)
		payload.AbsBeforeEpoch = &absBeforeEpoch
	case ClaimPredicateTypeClaimPredicateBeforeRelativeTime:
		relBefore := int64(c.MustRelBefore())
		payload.RelBefore = &relBefore
	default:
		err = fmt.Errorf("invalid predicate type: %s", c.Type.String())
	}
	return payload, err
}

func (c ClaimPredicate) MarshalJSON() ([]byte, error) {
	payload, err := c.toJSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(payload)
}

func (c *ClaimPredicate) UnmarshalJSON(b []byte) error {
	var payload claimPredicateJSON
	if err := json.Unmarshal(b, &payload); err != nil {
		return err
	}

	parsed, err := payload.toXDR()
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}
