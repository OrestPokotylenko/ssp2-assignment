package domain

import (
	"errors"
	"testing"
)

func TestNewMessage(t *testing.T) {
	tests := []struct {
		name    string
		author  string
		body    string
		want    Message
		wantErr error
	}{
		{
			name:   "valid message",
			author: "  Cartman ",
			body:   " Respect my authoritah!  ",
			want:   Message{Author: "Cartman", Body: "Respect my authoritah!"},
		},
		{
			name:    "missing author",
			author:  "   ",
			body:    "Screw you guys, I'm going home.",
			wantErr: ErrAuthorRequired,
		},
		{
			name:    "missing body",
			author:  "Cartman",
			body:    "\t",
			wantErr: ErrBodyRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMessage(tt.author, tt.body)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewMessage() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("NewMessage() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
