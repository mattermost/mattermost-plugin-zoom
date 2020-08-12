module github.com/mattermost/mattermost-plugin-zoom

go 1.12

replace github.com/mattermost/mattermost-server/v5 v5.25.1 => github.com/larkox/mattermost-server/v5 v5.3.2-0.20200702114622-6c1dee6bb932

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/mattermost/mattermost-server/v5 v5.25.1
	github.com/mholt/archiver/v3 v3.3.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.6.1
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
)
