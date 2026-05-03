package api

import (
	"errors"
	"testing"
)

func TestIsRateLimitError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "binance too many requests",
			err:  errors.New("failed to get account info: <APIError> code=-1003, msg=Too many requests"),
			want: true,
		},
		{
			name: "binance banned until",
			err:  errors.New("Way too many requests; IP banned until 1774588556937"),
			want: true,
		},
		{
			name: "binance code only",
			err:  errors.New("exchange request failed: <APIError> code=-1003"),
			want: true,
		},
		{
			name: "generic error",
			err:  errors.New("database connection failed"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := IsRateLimitError(tt.err)
			if got != tt.want {
				t.Fatalf("IsRateLimitError() = %v, want %v", got, tt.want)
			}
		})
	}
}
