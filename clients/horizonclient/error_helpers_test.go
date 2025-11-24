package horizonclient

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/support/render/problem"
	"github.com/stretchr/testify/assert"
)

func TestIsNotFoundError(t *testing.T) {
	testCases := []struct {
		desc string
		err  error
		is   bool
	}{
		{
			desc: "nil error",
			err:  nil,
			is:   false,
		},
		{
			desc: "another Go type of error",
			err:  errors.New("error"),
			is:   false,
		},
		{
			desc: "not found problem (pointer)",
			err: &Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/not_found",
					Title:  "Resource Missing",
					Status: 404,
				},
			},
			is: true,
		},
		{
			desc: "wrapped not found problem (pointer)",
			err: errors.Wrap(&Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/not_found",
					Title:  "Resource Missing",
					Status: 404,
				},
			}, "wrap message"),
			is: true,
		},
		{
			desc: "not found problem (not a pointer)",
			err: Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/not_found",
					Title:  "Resource Missing",
					Status: 404,
				},
			},
			is: true,
		},
		{
			desc: "wrapped not found problem (not a pointer)",
			err: errors.Wrap(Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/not_found",
					Title:  "Resource Missing",
					Status: 404,
				},
			}, "wrap message"),
			is: true,
		},
		{
			desc: "some other problem (pointer)",
			err: &Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/server_error",
					Title:  "Server Error",
					Status: 500,
				},
			},
			is: false,
		},
		{
			desc: "wrapped some other problem (pointer)",
			err: errors.Wrap(&Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/server_error",
					Title:  "Server Error",
					Status: 500,
				},
			}, "wrap message"),
			is: false,
		},
		{
			desc: "some other problem (not a pointer)",
			err: Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/server_error",
					Title:  "Server Error",
					Status: 500,
				},
			},
			is: false,
		},
		{
			desc: "wrapped some other problem (not a pointer)",
			err: errors.Wrap(Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/server_error",
					Title:  "Server Error",
					Status: 500,
				},
			}, "wrap message"),
			is: false,
		},
		{
			desc: "a nil *horizonclient.Error",
			err:  (*Error)(nil),
			is:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			is := IsNotFoundError(tc.err)
			assert.Equal(t, tc.is, is)
		})
	}
}

func TestGetError(t *testing.T) {
	testCases := []struct {
		desc    string
		err     error
		wantErr error
	}{
		{
			desc:    "nil error",
			err:     nil,
			wantErr: nil,
		},
		{
			desc:    "another Go type of error",
			err:     errors.New("error"),
			wantErr: nil,
		},
		{
			desc: "not found problem (pointer)",
			err: &Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/not_found",
					Title:  "Resource Missing",
					Status: 404,
				},
			},
			wantErr: &Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/not_found",
					Title:  "Resource Missing",
					Status: 404,
				},
			},
		},
		{
			desc: "wrapped not found problem (pointer)",
			err: errors.Wrap(&Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/not_found",
					Title:  "Resource Missing",
					Status: 404,
				},
			}, "wrap message"),
			wantErr: &Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/not_found",
					Title:  "Resource Missing",
					Status: 404,
				},
			},
		},
		{
			desc: "not found problem (not a pointer)",
			err: Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/not_found",
					Title:  "Resource Missing",
					Status: 404,
				},
			},
			wantErr: &Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/not_found",
					Title:  "Resource Missing",
					Status: 404,
				},
			},
		},
		{
			desc: "wrapped not found problem (not a pointer)",
			err: errors.Wrap(Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/not_found",
					Title:  "Resource Missing",
					Status: 404,
				},
			}, "wrap message"),
			wantErr: &Error{
				Problem: problem.P{
					Type:   "https://stellar.org/horizon-errors/not_found",
					Title:  "Resource Missing",
					Status: 404,
				},
			},
		},
		{
			desc:    "a nil *horizonclient.Error",
			err:     (*Error)(nil),
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			gotErr := GetError(tc.err)
			if tc.wantErr == nil {
				assert.Nil(t, gotErr)
			} else {
				assert.Equal(t, tc.wantErr, gotErr)
			}
		})
	}
}
