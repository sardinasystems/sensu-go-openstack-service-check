package main

import (
	"encoding/json"
	"time"

	"github.com/gophercloud/gophercloud"
	"go.uber.org/multierr"
)

type AnyTime time.Time

const RFC3339MilliNoTNoZ = "2006-01-02 15:04:05.999999"
const RFC3339MilliNoT = "2006-01-02 15:04:05.999999-07:00"

func (jt *AnyTime) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	if s == "" {
		return nil
	}

	parsed := false
	for _, layout := range []string{
		RFC3339MilliNoT,
		RFC3339MilliNoTNoZ,
		gophercloud.RFC3339Milli,
		gophercloud.RFC3339MilliNoZ,
		gophercloud.RFC3339ZNoTNoZ,
		gophercloud.RFC3339ZNoT,
	} {
		t, err2 := time.Parse(layout, s)
		if err2 != nil {
			multierr.AppendInto(&err, err2)
			continue
		}

		parsed = true
		*jt = AnyTime(t)
		break
	}

	if parsed {
		return nil
	}
	return err
}

func (jt *AnyTime) As() time.Time {
	if jt == nil {
		return time.Time{}
	}
	return time.Time(*jt)
}
