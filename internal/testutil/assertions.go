// Package testutil provides common testing utilities and assertion helpers
package testutil

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

// AssertEqual checks if two values are equal and reports a test failure if not
func AssertEqual(t *testing.T, got, want interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("%sexpected: %v\ngot: %v", msg, want, got)
	}
}

// AssertNotEqual checks if two values are not equal and reports a test failure if they are
func AssertNotEqual(t *testing.T, got, want interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if reflect.DeepEqual(got, want) {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("%sexpected values to be different, but both are: %v", msg, got)
	}
}

// AssertNil checks if value is nil
func AssertNil(t *testing.T, value interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if !isNil(value) {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("%sexpected nil, got: %v", msg, value)
	}
}

// AssertNotNil checks if value is not nil
func AssertNotNil(t *testing.T, value interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if isNil(value) {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("%sexpected non-nil value, got nil", msg)
	}
}

// AssertError checks if error is not nil
func AssertError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if err == nil {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("%sexpected error, got nil", msg)
	}
}

// AssertNoError checks if error is nil
func AssertNoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if err != nil {
		msg := formatMessage(msgAndArgs...)
		t.Fatalf("%sunexpected error: %v", msg, err)
	}
}

// AssertErrorIs checks if error matches expected error
func AssertErrorIs(t *testing.T, got, want error, msgAndArgs ...interface{}) {
	t.Helper()
	if got != want {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("%sexpected error: %v\ngot: %v", msg, want, got)
	}
}

// AssertTrue checks if condition is true
func AssertTrue(t *testing.T, condition bool, msgAndArgs ...interface{}) {
	t.Helper()
	if !condition {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("%sexpected true, got false", msg)
	}
}

// AssertFalse checks if condition is false
func AssertFalse(t *testing.T, condition bool, msgAndArgs ...interface{}) {
	t.Helper()
	if condition {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("%sexpected false, got true", msg)
	}
}

// AssertContains checks if slice contains element
func AssertContains(t *testing.T, slice interface{}, element interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	sliceValue := reflect.ValueOf(slice)
	if sliceValue.Kind() != reflect.Slice {
		t.Fatalf("AssertContains: first argument must be a slice, got %T", slice)
	}

	for i := 0; i < sliceValue.Len(); i++ {
		if reflect.DeepEqual(sliceValue.Index(i).Interface(), element) {
			return
		}
	}

	msg := formatMessage(msgAndArgs...)
	t.Errorf("%sexpected slice to contain: %v\nslice: %v", msg, element, slice)
}

// AssertNotContains checks if slice does not contain element
func AssertNotContains(t *testing.T, slice interface{}, element interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	sliceValue := reflect.ValueOf(slice)
	if sliceValue.Kind() != reflect.Slice {
		t.Fatalf("AssertNotContains: first argument must be a slice, got %T", slice)
	}

	for i := 0; i < sliceValue.Len(); i++ {
		if reflect.DeepEqual(sliceValue.Index(i).Interface(), element) {
			msg := formatMessage(msgAndArgs...)
			t.Errorf("%sexpected slice not to contain: %v\nslice: %v", msg, element, slice)
			return
		}
	}
}

// AssertLen checks if collection has expected length
func AssertLen(t *testing.T, collection interface{}, expectedLen int, msgAndArgs ...interface{}) {
	t.Helper()
	value := reflect.ValueOf(collection)
	actualLen := value.Len()
	if actualLen != expectedLen {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("%sexpected length: %d\ngot: %d", msg, expectedLen, actualLen)
	}
}

// AssertHTTPStatus checks if HTTP response has expected status code
func AssertHTTPStatus(t *testing.T, resp *http.Response, expectedStatus int, msgAndArgs ...interface{}) {
	t.Helper()
	if resp.StatusCode != expectedStatus {
		msg := formatMessage(msgAndArgs...)
		t.Errorf("%sexpected status: %d\ngot: %d", msg, expectedStatus, resp.StatusCode)
	}
}

// AssertJSONResponse decodes JSON response and checks status code
func AssertJSONResponse(t *testing.T, resp *http.Response, expectedStatus int, target interface{}) {
	t.Helper()
	AssertHTTPStatus(t, resp, expectedStatus)

	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
}

// Helper functions

func isNil(value interface{}) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	}

	return false
}

func formatMessage(msgAndArgs ...interface{}) string {
	if len(msgAndArgs) == 0 {
		return ""
	}
	if len(msgAndArgs) == 1 {
		return msgAndArgs[0].(string) + ": "
	}
	return ""
}
