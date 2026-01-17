package lock

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsArrowMarkerSuffix(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantMatch  bool
		wantRemove int
	}{
		{
			name:       "empty slice",
			data:       []byte{},
			wantMatch:  false,
			wantRemove: 1,
		},
		{
			name:       "single byte",
			data:       []byte{0x61}, // 'a'
			wantMatch:  false,
			wantRemove: 1,
		},
		{
			name:       "ends with ArrowUpMarker",
			data:       []byte{'a', 'b', 0x00, 0x01},
			wantMatch:  true,
			wantRemove: 2,
		},
		{
			name:       "ends with ArrowDownMarker",
			data:       []byte{'a', 'b', 0x00, 0x02},
			wantMatch:  true,
			wantRemove: 2,
		},
		{
			name:       "ends with ArrowLeftMarker",
			data:       []byte{'a', 'b', 0x00, 0x03},
			wantMatch:  true,
			wantRemove: 2,
		},
		{
			name:       "ends with ArrowRightMarker",
			data:       []byte{'a', 'b', 0x00, 0x04},
			wantMatch:  true,
			wantRemove: 2,
		},
		{
			name:       "does not end with marker",
			data:       []byte{'a', 'b', 'c'},
			wantMatch:  false,
			wantRemove: 1,
		},
		{
			name:       "marker in middle but not at end",
			data:       []byte{0x00, 0x01, 'x'},
			wantMatch:  false,
			wantRemove: 1,
		},
		{
			name:       "just the marker",
			data:       []byte{0x00, 0x01},
			wantMatch:  true,
			wantRemove: 2,
		},
		{
			name:       "null byte but wrong second byte",
			data:       []byte{'a', 0x00, 0x05},
			wantMatch:  false,
			wantRemove: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch, gotRemove := IsArrowMarkerSuffix(tt.data)
			assert.Equal(t, tt.wantMatch, gotMatch, "match result")
			assert.Equal(t, tt.wantRemove, gotRemove, "remove length")
		})
	}
}

func TestSecureBuffer_Backspace(t *testing.T) {
	t.Run("backspace on empty buffer", func(t *testing.T) {
		sb := NewSecureBuffer()
		assert.False(t, sb.Backspace(), "Backspace() on empty buffer should return false")
		assert.Equal(t, 0, sb.Len())
	})

	t.Run("backspace after regular char", func(t *testing.T) {
		sb := NewSecureBuffer()
		sb.AppendRune('a')
		sb.AppendRune('b')

		require.True(t, sb.Backspace(), "Backspace() should return true")
		assert.Equal(t, 1, sb.Len())
		assert.Equal(t, "a", string(sb.Bytes()))
	})

	t.Run("backspace after arrow marker", func(t *testing.T) {
		sb := NewSecureBuffer()
		sb.AppendRune('x')
		sb.AppendString(ArrowUpMarker)

		require.True(t, sb.Backspace(), "Backspace() should return true")
		// Should remove 2 bytes (the arrow marker)
		assert.Equal(t, 1, sb.Len())
		assert.Equal(t, "x", string(sb.Bytes()))
	})

	t.Run("mixed sequence backspace", func(t *testing.T) {
		sb := NewSecureBuffer()
		sb.AppendRune('a')             // 1 byte
		sb.AppendString(ArrowUpMarker) // 2 bytes
		sb.AppendRune('b')             // 1 byte

		// Backspace removes 'b' (1 byte)
		require.True(t, sb.Backspace(), "Backspace() should return true")
		assert.Equal(t, 3, sb.Len(), "after 1st backspace: should be 'a' + ArrowUpMarker")

		// Backspace removes ArrowUpMarker (2 bytes)
		require.True(t, sb.Backspace(), "Backspace() should return true")
		assert.Equal(t, 1, sb.Len(), "after 2nd backspace: should be 'a'")

		// Backspace removes 'a' (1 byte)
		require.True(t, sb.Backspace(), "Backspace() should return true")
		assert.Equal(t, 0, sb.Len(), "after 3rd backspace: should be empty")

		// Backspace on empty
		assert.False(t, sb.Backspace(), "Backspace() on empty should return false")
	})

	t.Run("backspace with multiple arrow markers", func(t *testing.T) {
		sb := NewSecureBuffer()
		sb.AppendString(ArrowDownMarker)  // 2 bytes
		sb.AppendString(ArrowLeftMarker)  // 2 bytes
		sb.AppendString(ArrowRightMarker) // 2 bytes

		// Remove ArrowRightMarker
		sb.Backspace()
		assert.Equal(t, 4, sb.Len(), "after 1st backspace")

		// Remove ArrowLeftMarker
		sb.Backspace()
		assert.Equal(t, 2, sb.Len(), "after 2nd backspace")

		// Remove ArrowDownMarker
		sb.Backspace()
		assert.Equal(t, 0, sb.Len(), "after 3rd backspace")
	})
}

func TestSecureBuffer_VisualLen(t *testing.T) {
	t.Run("empty buffer", func(t *testing.T) {
		sb := NewSecureBuffer()
		assert.Equal(t, 0, sb.VisualLen())
	})

	t.Run("regular chars only", func(t *testing.T) {
		sb := NewSecureBuffer()
		sb.AppendRune('a')
		sb.AppendRune('b')
		sb.AppendRune('c')
		assert.Equal(t, 3, sb.VisualLen())
	})

	t.Run("arrows count as one each", func(t *testing.T) {
		sb := NewSecureBuffer()
		sb.AppendString(ArrowUpMarker)
		sb.AppendString(ArrowDownMarker)
		assert.Equal(t, 2, sb.VisualLen())
	})

	t.Run("mixed chars and arrows", func(t *testing.T) {
		sb := NewSecureBuffer()
		sb.AppendRune('a')
		sb.AppendString(ArrowUpMarker)
		sb.AppendRune('b')
		sb.AppendString(ArrowLeftMarker)
		sb.AppendString(ArrowRightMarker)
		// 'a' + arrow + 'b' + arrow + arrow = 5 visual chars
		assert.Equal(t, 5, sb.VisualLen())
	})

	t.Run("all arrow types", func(t *testing.T) {
		sb := NewSecureBuffer()
		sb.AppendString(ArrowUpMarker)
		sb.AppendString(ArrowDownMarker)
		sb.AppendString(ArrowLeftMarker)
		sb.AppendString(ArrowRightMarker)
		assert.Equal(t, 4, sb.VisualLen())
	})
}
