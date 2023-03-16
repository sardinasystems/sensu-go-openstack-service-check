package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/stretchr/testify/assert"
)

func TestAnyTime(t *testing.T) {

	// value got from Neutron Zed
	testCases := []struct {
		name   string
		value  string
		layout string
	}{
		{"NoTNoZ", "2023-03-16 18:35:47", gophercloud.RFC3339ZNoTNoZ},
		{"MilliNoZ", "2023-03-16 18:35:47.845000", RFC3339MilliNoTNoZ},
		{"Micros", "2023-03-16 18:35:47.845000+00:00", RFC3339MilliNoT},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			expected, err := time.Parse(tc.layout, tc.value)
			assert.NoError(err)

			js := &struct {
				T string `json:"t"`
			}{
				T: tc.value,
			}

			encoded, err := json.Marshal(js)
			assert.NoError(err)

			jt := &struct {
				T AnyTime `json:"t"`
			}{}

			err = json.Unmarshal(encoded, jt)
			assert.NoError(err)
			assert.Equal(expected, time.Time(jt.T))
		})
	}
}
