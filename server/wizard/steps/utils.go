package steps

import "emperror.dev/errors"

func isValidURL(u string) error {
	return nil
}

func isValidURLSubmission(submission map[string]interface{}, name string) (string, error) {
	typedString, err := safeString(submission, name)
	if err != nil {
		return "", err
	}

	err = isValidURL(typedString)
	if err != nil {
		return "", err
	}

	return typedString, nil
}

func safeString(submission map[string]interface{}, name string) (string, error) {
	rawString, ok := submission[name]
	if !ok {
		return "", errors.Errorf("%s missing", name)
	}
	typedString, ok := rawString.(string)
	if !ok {
		return "", errors.Errorf("%s is not a string", name)
	}

	return typedString, nil
}
