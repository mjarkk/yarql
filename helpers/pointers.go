package helpers

// CheckStrPtr returns a pointer to a string or returns a nil pointer if v is empty
func CheckStrPtr(v string) *string {
	if len(v) == 0 {
		return nil
	}
	return StrPtr(v)
}

// StrPtr returns a pointer to a string.
func StrPtr(v string) *string {
	return &v
}

// StrPtr returns a nil string pointer
var PtrToEmptyStr = new(string)

// BoolPtr returns a pointer to a bool.
func BoolPtr(v bool) *bool {
	return &v
}

// IntPtr returns a pointer to an int.
func IntPtr(v int) *int {
	return &v
}

// Int64Ptr returns a pointer to a int64.
func Int64Ptr(v int64) *int64 {
	return &v
}

// Int32Ptr returns a pointer to a int32.
func Int32Ptr(v int32) *int32 {
	return &v
}

// Int16Ptr returns a pointer to a int16.
func Int16Ptr(v int16) *int16 {
	return &v
}

// Int8Ptr returns a pointer to a int8.
func Int8Ptr(v int8) *int8 {
	return &v
}

// UintPtr returns a pointer to a uint.
func UintPtr(v uint) *uint {
	return &v
}

// Uint64Ptr returns a pointer to a uint64.
func Uint64Ptr(v uint64) *uint64 {
	return &v
}

// Uint32Ptr returns a pointer to a uint32.
func Uint32Ptr(v uint32) *uint32 {
	return &v
}

// Uint16Ptr returns a pointer to a uint16.
func Uint16Ptr(v uint16) *uint16 {
	return &v
}

// Uint8Ptr returns a pointer to a uint8.
func Uint8Ptr(v uint8) *uint8 {
	return &v
}
