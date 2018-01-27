package deis

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	// formatErrUnknown is used to create an dynamic error if no error matches
	formatErrUnknown = "Unknown Error (%d): %s"
	jsonParsingError = "error decoding json response (%s): %s"

	// fieldReqMsg is API error stating a field is required.
	fieldReqMsg           = "This field may not be blank."
	invalidUserMsg        = "Enter a valid username. This value may contain only letters, numbers and @/./+/-/_ characters."
	failedLoginMsg        = "Unable to log in with provided credentials."
	invalidAppNameMsg     = "App name can only contain a-z (lowercase), 0-9 and hyphens"
	invalidNameMsg        = "Can only contain a-z (lowercase), 0-9 and hyphens"
	invalidCertMsg        = "Could not load certificate"
	invalidPodMsg         = "does not exist in application"
	invalidDomainMsg      = "Hostname does not look valid."
	invalidVersionMsg     = "version cannot be below 0"
	invalidKeyMsg         = "Key contains invalid base64 chars"
	duplicateUserMsg      = "A user with that username already exists."
	invalidEmailMsg       = "Enter a valid email address."
	invalidTagMsg         = "No nodes matched the provided labels"
	duplicateIDMsg        = "Application with this id already exists."
	cancellationFailedMsg = "still has applications assigned. Delete or transfer ownership"
	duplicateDomainMsg    = "Domain is already in use by another application"
	duplicateKeyMsg       = "Public Key is already in use"
)

var (
	// ErrServerError is returned when the server returns a 500.
	ErrServerError = errors.New("Internal Server Error")
	// ErrMethodNotAllowed is thrown when using a unsupposrted method.
	// This should not come up unless there in an bug in the SDK.
	ErrMethodNotAllowed = errors.New("Method Not Allowed")
	// ErrInvalidUsername is returned when the user specifies an invalid or missing username.
	ErrInvalidUsername = errors.New(invalidUserMsg)
	// ErrDuplicateUsername is returned when trying to register a user that already exists.
	ErrDuplicateUsername = errors.New(duplicateUserMsg)
	// ErrMissingPassword is returned when a password is not sent with the request.
	ErrMissingPassword = errors.New("A Password is required")
	// ErrLogin is returned when the api cannot login fails with provided username and password
	ErrLogin = errors.New(failedLoginMsg)
	// ErrUnauthorized is given when the API returns a 401.
	ErrUnauthorized = errors.New("Unauthorized: Missing or Invalid Token")
	// ErrInvalidAppName is returned when the user specifies an invalid app name.
	ErrInvalidAppName = errors.New(invalidAppNameMsg)
	// ErrConflict is returned when the API returns a 409.
	ErrConflict = errors.New("This action could not be completed due to a conflict.")
	// ErrForbidden is returned when the API returns a 403.
	ErrForbidden = errors.New("You do not have permission to perform this action.")
	// ErrMissingKey is returned when a key is not sent with the request.
	ErrMissingKey = errors.New("A key is required")
	// ErrDuplicateKey is returned when adding a key that already exists.
	ErrDuplicateKey = errors.New(duplicateKeyMsg)
	// ErrInvalidName is returned when a name is invalid or missing.
	ErrInvalidName = errors.New(fmt.Sprintf("Name %s", strings.ToLower(invalidNameMsg)))
	// ErrInvalidCertificate is returned when a certififate is missing or invalid
	ErrInvalidCertificate = errors.New(invalidCertMsg)
	// ErrPodNotFound is returned when a pod type is not Found
	ErrPodNotFound = errors.New("Pod not found in application")
	// ErrInvalidDomain is returned when a domain is missing or invalid
	ErrInvalidDomain = errors.New(invalidDomainMsg)
	// ErrDuplicateDomain is returned adding domain that is already in use
	ErrDuplicateDomain = errors.New(duplicateDomainMsg)
	// ErrInvalidImage is returned when a image is missing or invalid
	ErrInvalidImage = errors.New("The given image is invalid")
	// ErrInvalidVersion is returned when a version is invalid
	ErrInvalidVersion = errors.New("The given version is invalid")
	// ErrMissingID is returned when a ID is missing
	ErrMissingID = errors.New("An id is required")
	// ErrInvalidEmail is returned when a user gives an invalid email.
	ErrInvalidEmail = errors.New(invalidEmailMsg)
	// ErrTagNotFound is returned when no node can be found that matches the tag
	ErrTagNotFound = errors.New(invalidTagMsg)
	// ErrDuplicateApp is returned when create an app with an ID that already exists
	ErrDuplicateApp = errors.New(duplicateIDMsg)
	// ErrCancellationFailed is returned when cancelling a user fails.
	ErrCancellationFailed = errors.New("Failed to delete user because the user still has applications assigned. Delete or transfer ownership.")
)

// ErrUnprocessable is returned when the controller throws a 422.
type ErrUnprocessable struct {
	errorMsg string
}

// ErrNotFound is returned when the controller throws a 404.
type ErrNotFound struct {
	errorMsg string
}

func (e ErrUnprocessable) Error() string {
	return fmt.Sprintf("Unable to process your request: %s", e.errorMsg)
}

func (e ErrNotFound) Error() string {
	return e.errorMsg
}

// checkForErrors tries to match up an API error with an predefined error in the SDK.
func checkForErrors(res *http.Response) error {
	if res.StatusCode >= 200 && res.StatusCode < 400 {
		return nil
	}
	defer res.Body.Close()

	out, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return unknownServerError(res.StatusCode, err.Error())
	}

	switch res.StatusCode {
	case 400:
		bodyMap := make(map[string]interface{})
		if err := json.Unmarshal(out, &bodyMap); err != nil {
			return unknownServerError(res.StatusCode, fmt.Sprintf(jsonParsingError, err, string(out)))
		}

		if scanResponse(bodyMap, "username", []string{fieldReqMsg, invalidUserMsg}, true) {
			return ErrInvalidUsername
		}

		if scanResponse(bodyMap, "username", []string{duplicateUserMsg}, true) {
			return ErrDuplicateUsername
		}

		if scanResponse(bodyMap, "password", []string{fieldReqMsg}, true) {
			return ErrMissingPassword
		}

		if scanResponse(bodyMap, "non_field_errors", []string{failedLoginMsg}, true) {
			return ErrLogin
		}

		if scanResponse(bodyMap, "id", []string{invalidAppNameMsg}, true) {
			return ErrInvalidAppName
		}

		if scanResponse(bodyMap, "id", []string{duplicateIDMsg}, true) {
			return ErrDuplicateApp
		}

		if scanResponse(bodyMap, "key", []string{fieldReqMsg}, true) {
			return ErrMissingKey
		}

		if scanResponse(bodyMap, "key", []string{duplicateKeyMsg}, true) {
			return ErrDuplicateKey
		}

		if scanResponse(bodyMap, "public", []string{fieldReqMsg, invalidKeyMsg}, true) {
			return ErrMissingKey
		}

		if scanResponse(bodyMap, "certificate", []string{fieldReqMsg, invalidCertMsg}, false) {
			return ErrInvalidCertificate
		}

		if scanResponse(bodyMap, "name", []string{fieldReqMsg, invalidNameMsg}, true) {
			return ErrInvalidName
		}

		if scanResponse(bodyMap, "domain", []string{invalidDomainMsg}, true) {
			return ErrInvalidDomain
		}

		if scanResponse(bodyMap, "domain", []string{duplicateDomainMsg}, true) {
			return ErrDuplicateDomain
		}

		if scanResponse(bodyMap, "image", []string{fieldReqMsg}, true) {
			return ErrInvalidImage
		}

		if scanResponse(bodyMap, "id", []string{fieldReqMsg}, true) {
			return ErrMissingID
		}

		if scanResponse(bodyMap, "email", []string{invalidEmailMsg}, true) {
			return ErrInvalidEmail
		}

		if v, ok := bodyMap["detail"].(string); ok {
			if strings.Contains(v, invalidPodMsg) {
				return ErrPodNotFound
			}
			if strings.Contains(v, invalidVersionMsg) {
				return ErrInvalidVersion
			}
			if strings.Contains(v, invalidTagMsg) {
				return ErrTagNotFound
			}
		}
		return unknownServerError(res.StatusCode, string(out))
	case 401:
		return ErrUnauthorized
	case 403:
		return ErrForbidden
	case 404:
		if string(out) != "" {
			return ErrNotFound{string(out)}
		}
		return ErrNotFound{"Not Found"}
	case 405:
		return ErrMethodNotAllowed
	case 409:
		bodyMap := make(map[string]interface{})
		if err := json.Unmarshal(out, &bodyMap); err != nil {
			return unknownServerError(res.StatusCode, fmt.Sprintf(jsonParsingError, err, string(out)))
		}
		if v, ok := bodyMap["detail"].(string); ok {
			if strings.Contains(v, cancellationFailedMsg) {
				return ErrCancellationFailed
			}
		}
		return unknownServerError(res.StatusCode, string(out))
	case 422:
		bodyMap := make(map[string]interface{})
		if err := json.Unmarshal(out, &bodyMap); err != nil {
			return unknownServerError(res.StatusCode, fmt.Sprintf(jsonParsingError, err, string(out)))
		}
		if v, ok := bodyMap["detail"].(string); ok {
			return ErrUnprocessable{v}
		}
		return unknownServerError(res.StatusCode, string(out))
	case 500:
		return ErrServerError
	default:
		return unknownServerError(res.StatusCode, string(out))
	}
}

func arrayContents(m map[string]interface{}, field string) []string {
	if v, ok := m[field]; ok {
		if a, ok := v.([]interface{}); ok {
			sa := []string{}

			for _, i := range a {
				if s, ok := i.(string); ok {
					sa = append(sa, s)
				}
			}
			return sa
		}
	}

	return []string{}
}

func arrayContains(search string, completeMatch bool, array []string) bool {
	for _, element := range array {
		if completeMatch {
			if element == search {
				return true
			}
		} else {
			if strings.Contains(element, search) {
				return true
			}
		}
	}

	return false
}

func unknownServerError(statusCode int, message string) error {
	// newlines set from controller aren't evaluated as controller characters, so they need to be replaced
	message = strings.Replace(message, `\n`, "\n", -1)
	return fmt.Errorf(formatErrUnknown, statusCode, message)
}

func scanResponse(
	body map[string]interface{}, field string, errMsgs []string, completeMatch bool) bool {
	for _, msg := range errMsgs {
		if arrayContains(msg, completeMatch, arrayContents(body, field)) {
			return true
		}
	}

	return false
}
