// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package zoom

import "time"

// User is defined at https://marketplace.zoom.us/docs/api-reference/zoom-api/users/users
type User struct {
	ID                string    `json:"id"`
	FirstName         string    `json:"first_name"`
	LastName          string    `json:"last_name"`
	Email             string    `json:"email"`
	Type              int       `json:"type"`
	Pmi               int       `json:"pmi"`
	Timezone          string    `json:"timezone"`
	Dept              string    `json:"dept"`
	CreatedAt         time.Time `json:"created_at"`
	LastLoginTime     time.Time `json:"last_login_time"`
	LastClientVersion string    `json:"last_client_version"`
	VanityURL         string    `json:"vanity_url"`
	Verified          int       `json:"verified"`
	PicURL            string    `json:"pic_url"`
}
