package assert

// Emptyf asserts that the specified object is empty.  I.e. nil, "", false, 0 or either
// a slice or a channel with len == 0.
//
//  assert.Emptyf(t, obj, "error message %s", "formatted")
func Emptyf(t TestingT, object interface{}, msg string, args ...interface{}) bool {
	return Empty(t, object, append([]interface{}{msg}, args...)...)
}

// Equalf asserts that two objects are equal.
//
//    assert.Equalf(t, 123, 123, "error message %s", "formatted")
//
// Pointer variable equality is determined based on the equality of the
// referenced values (as opposed to the memory addresses). Function equality
// cannot be determined and will always fail.
func Equalf(t TestingT, expected interface{}, actual interface{}, msg string, args ...interface{}) bool {
	return Equal(t, expected, actual, append([]interface{}{msg}, args...)...)
}

// EqualErrorf asserts that a function returned an error (i.e. not `nil`)
// and that it is equal to the provided error.
//
//   actualObj, err := SomeFunction()
//   assert.EqualErrorf(t, err,  expectedErrorString, "error message %s", "formatted")
func EqualErrorf(t TestingT, theError error, errString string, msg string, args ...interface{}) bool {
	return EqualError(t, theError, errString, append([]interface{}{msg}, args...)...)
}

// Errorf asserts that a function returned an error (i.e. not `nil`).
//
//   actualObj, err := SomeFunction()
//   if assert.Errorf(t, err, "error message %s", "formatted") {
// 	   assert.Equal(t, expectedErrorf, err)
//   }
func Errorf(t TestingT, err error, msg string, args ...interface{}) bool {
	return Error(t, err, append([]interface{}{msg}, args...)...)
}

// ErrorAsf asserts that at least one of the errors in err's chain matches target, and if so, sets target to that error value.
// This is a wrapper for errors.As.
func ErrorAsf(t TestingT, err error, target interface{}, msg string, args ...interface{}) bool {
	return ErrorAs(t, err, target, append([]interface{}{msg}, args...)...)
}

// ErrorIsf asserts that at least one of the errors in err's chain matches target.
// This is a wrapper for errors.Is.
func ErrorIsf(t TestingT, err error, target error, msg string, args ...interface{}) bool {
	return ErrorIs(t, err, target, append([]interface{}{msg}, args...)...)
}

// Failf reports a failure through
func Failf(t TestingT, failureMessage string, msg string, args ...interface{}) bool {
	return Fail(t, failureMessage, append([]interface{}{msg}, args...)...)
}

// Falsef asserts that the specified value is false.
//
//    assert.Falsef(t, myBool, "error message %s", "formatted")
func Falsef(t TestingT, value bool, msg string, args ...interface{}) bool {
	return False(t, value, append([]interface{}{msg}, args...)...)
}

// Greaterf asserts that the first element is greater than the second
//
//    assert.Greaterf(t, 2, 1, "error message %s", "formatted")
//    assert.Greaterf(t, float64(2), float64(1), "error message %s", "formatted")
//    assert.Greaterf(t, "b", "a", "error message %s", "formatted")
func Greaterf(t TestingT, e1 interface{}, e2 interface{}, msg string, args ...interface{}) bool {
	return Greater(t, e1, e2, append([]interface{}{msg}, args...)...)
}

// GreaterOrEqualf asserts that the first element is greater than or equal to the second
//
//    assert.GreaterOrEqualf(t, 2, 1, "error message %s", "formatted")
//    assert.GreaterOrEqualf(t, 2, 2, "error message %s", "formatted")
//    assert.GreaterOrEqualf(t, "b", "a", "error message %s", "formatted")
//    assert.GreaterOrEqualf(t, "b", "b", "error message %s", "formatted")
func GreaterOrEqualf(t TestingT, e1 interface{}, e2 interface{}, msg string, args ...interface{}) bool {
	return GreaterOrEqual(t, e1, e2, append([]interface{}{msg}, args...)...)
}

// JSONEqf asserts that two JSON strings are equivalent.
//
//  assert.JSONEqf(t, `{"hello": "world", "foo": "bar"}`, `{"foo": "bar", "hello": "world"}`, "error message %s", "formatted")
func JSONEqf(t TestingT, expected string, actual string, msg string, args ...interface{}) bool {
	return JSONEq(t, expected, actual, append([]interface{}{msg}, args...)...)
}

// Lessf asserts that the first element is less than the second
//
//    assert.Lessf(t, 1, 2, "error message %s", "formatted")
//    assert.Lessf(t, float64(1), float64(2), "error message %s", "formatted")
//    assert.Lessf(t, "a", "b", "error message %s", "formatted")
func Lessf(t TestingT, e1 interface{}, e2 interface{}, msg string, args ...interface{}) bool {
	return Less(t, e1, e2, append([]interface{}{msg}, args...)...)
}

// LessOrEqualf asserts that the first element is less than or equal to the second
//
//    assert.LessOrEqualf(t, 1, 2, "error message %s", "formatted")
//    assert.LessOrEqualf(t, 2, 2, "error message %s", "formatted")
//    assert.LessOrEqualf(t, "a", "b", "error message %s", "formatted")
//    assert.LessOrEqualf(t, "b", "b", "error message %s", "formatted")
func LessOrEqualf(t TestingT, e1 interface{}, e2 interface{}, msg string, args ...interface{}) bool {
	return LessOrEqual(t, e1, e2, append([]interface{}{msg}, args...)...)
}

// Negativef asserts that the specified element is negative
//
//    assert.Negativef(t, -1, "error message %s", "formatted")
//    assert.Negativef(t, -1.23, "error message %s", "formatted")
func Negativef(t TestingT, e interface{}, msg string, args ...interface{}) bool {
	return Negative(t, e, append([]interface{}{msg}, args...)...)
}

// Nilf asserts that the specified object is nil.
//
//    assert.Nilf(t, err, "error message %s", "formatted")
func Nilf(t TestingT, object interface{}, msg string, args ...interface{}) bool {
	return Nil(t, object, append([]interface{}{msg}, args...)...)
}

// NoErrorf asserts that a function returned no error (i.e. `nil`).
//
//   actualObj, err := SomeFunction()
//   if assert.NoErrorf(t, err, "error message %s", "formatted") {
// 	   assert.Equal(t, expectedObj, actualObj)
//   }
func NoErrorf(t TestingT, err error, msg string, args ...interface{}) bool {
	return NoError(t, err, append([]interface{}{msg}, args...)...)
}

// NotEmptyf asserts that the specified object is NOT empty.  I.e. not nil, "", false, 0 or either
// a slice or a channel with len == 0.
//
//  if assert.NotEmptyf(t, obj, "error message %s", "formatted") {
//    assert.Equal(t, "two", obj[1])
//  }
func NotEmptyf(t TestingT, object interface{}, msg string, args ...interface{}) bool {
	return NotEmpty(t, object, append([]interface{}{msg}, args...)...)
}

// NotEqualf asserts that the specified values are NOT equal.
//
//    assert.NotEqualf(t, obj1, obj2, "error message %s", "formatted")
//
// Pointer variable equality is determined based on the equality of the
// referenced values (as opposed to the memory addresses).
func NotEqualf(t TestingT, expected interface{}, actual interface{}, msg string, args ...interface{}) bool {
	return NotEqual(t, expected, actual, append([]interface{}{msg}, args...)...)
}

// NotErrorIsf asserts that at none of the errors in err's chain matches target.
// This is a wrapper for errors.Is.
func NotErrorIsf(t TestingT, err error, target error, msg string, args ...interface{}) bool {
	return NotErrorIs(t, err, target, append([]interface{}{msg}, args...)...)
}

// NotNilf asserts that the specified object is not nil.
//
//    assert.NotNilf(t, err, "error message %s", "formatted")
func NotNilf(t TestingT, object interface{}, msg string, args ...interface{}) bool {
	return NotNil(t, object, append([]interface{}{msg}, args...)...)
}

// Panicsf asserts that the code inside the specified f panics.
//
//   assert.Panicsf(t, func(){ GoCrazy() }, "error message %s", "formatted")
func Panicsf(t TestingT, f func(), msg string, args ...interface{}) bool {
	return Panics(t, f, append([]interface{}{msg}, args...)...)
}

// Truef asserts that the specified value is true.
//
//    assert.Truef(t, myBool, "error message %s", "formatted")
func Truef(t TestingT, value bool, msg string, args ...interface{}) bool {
	return True(t, value, append([]interface{}{msg}, args...)...)
}
