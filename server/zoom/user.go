// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package zoom

import "time"

// User is defined at https://marketplace.zoom.us/docs/api-reference/zoom-api/users/users
type User struct {
	CreatedAt         time.Time `json:"created_at"`
	LastLoginTime     time.Time `json:"last_login_time"`
	ID                string    `json:"id"`
	FirstName         string    `json:"first_name"`
	LastName          string    `json:"last_name"`
	Email             string    `json:"email"`
	Timezone          string    `json:"timezone"`
	Dept              string    `json:"dept"`
	LastClientVersion string    `json:"last_client_version"`
	VanityURL         string    `json:"vanity_url"`
	PicURL            string    `json:"pic_url"`
	Type              int       `json:"type"`
	Pmi               int       `json:"pmi"`
	Verified          int       `json:"verified"`
}
