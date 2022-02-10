package steps

import (
	"fmt"
	"net/url"

	"github.com/pkg/errors"
)

// imagePathToMarkdown converts an image path in the /public/setup_flow_images folder into a markdown-compatible image
func imagePathToMarkdown(pluginURL, name, imgPath string) string {
	return fmt.Sprintf("![%s](%s/public/setup_flow_images/%s)", name, pluginURL, imgPath)
}

// isValidURL checks if a given URL is a valid URL with a host and a http or http scheme.
func isValidURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.Errorf("URL schema must either be %q or %q", "http", "https")
	}

	if u.Host == "" {
		return errors.New("URL must contain a host")
	}

	return nil
}

// isValidURLSubmission checks if a given URL from a dialog submission is a valid URL
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
